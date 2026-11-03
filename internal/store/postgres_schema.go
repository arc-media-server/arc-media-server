package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed postgres/baseline.sql postgres/tables.json
var pgSchemaFS embed.FS
var pgColumns map[string][]string
var pgTextColumns = map[string]map[string]bool{}

func init() {
	b, _ := pgSchemaFS.ReadFile("postgres/tables.json")
	if err := json.Unmarshal(b, &pgColumns); err != nil {
		panic(err)
	}
	body, _ := pgSchemaFS.ReadFile("postgres/baseline.sql")
	for table, cols := range pgColumns {
		pgTextColumns[table] = map[string]bool{}
		start := strings.Index(string(body), "CREATE TABLE "+quoted(table)+" (")
		if start < 0 {
			continue
		}
		part := string(body)[start:]
		end := strings.Index(part, "\n);")
		if end < 0 {
			continue
		}
		for _, col := range cols {
			pgTextColumns[table][col] = strings.Contains(part[:end], quoted(col)+" TEXT ")
		}
	}

}

// OpenPostgres opens an explicitly selected external database. It never falls back to SQLite.
// A dedicated database is required. Only one Arc server may use it at a time.
func OpenPostgres(ctx context.Context, dsn string, opts Options) (*Store, error) {
	return openPostgres(ctx, dsn, opts, nil)
}
func openPostgres(ctx context.Context, dsn string, opts Options, initialize func(*sql.Tx) error) (*Store, error) {
	body, _ := pgSchemaFS.ReadFile("postgres/baseline.sql")
	if !strings.Contains(string(body), "-- Arc migration fingerprint: "+migrationFingerprint()) {
		return nil, errors.New("PostgreSQL baseline must be regenerated for this Arc schema")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL connection configuration")
	}
	cfg.ConnectTimeout = 5 * time.Second
	cfg.RuntimeParams["application_name"] = "arc-media-server"
	cfg.RuntimeParams["statement_timeout"] = "30000"
	cfg.RuntimeParams["idle_in_transaction_session_timeout"] = "60000"
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	// Use stdlib's registered config so connection strings never need to be rendered in logs.
	registered := registerPGConfig(cfg)
	success := false
	defer func() {
		if !success {
			unregisterPGConfig(registered)
		}
	}()
	db, err := sql.Open("arc-postgres", registered)
	if err != nil {
		return nil, safePGError(err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err = db.PingContext(ctx); err != nil {
		db.Close()
		return nil, safePGError(err)
	}
	s := &Store{db: &database{DB: db, writer: db}, backend: "postgres", pgConfigID: registered}
	// A session-level advisory lock excludes other Arc instances, restore and transfers.
	// The dedicated control connection is retained separately from query pools.
	lockDB, err := sql.Open("arc-postgres", registered)
	if err != nil {
		db.Close()
		return nil, safePGError(err)
	}
	lockDB.SetMaxOpenConns(1)
	lock, err := lockDB.Conn(ctx)
	if err != nil {
		lockDB.Close()
		db.Close()
		return nil, safePGError(err)
	}
	var locked bool
	err = lock.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(hashtextextended(current_database() || ':' || current_schema(),308))").Scan(&locked)
	if err != nil || !locked {
		lock.Close()
		lockDB.Close()
		db.Close()
		return nil, errors.New("PostgreSQL database is unavailable or already used by another Arc process")
	}
	s.pgLock, s.pgLockDB = lock, lockDB
	fail := func(err error) (*Store, error) { s.Close(); return nil, err }
	if err = s.migratePostgres(ctx, opts, initialize); err != nil {
		return fail(err)
	}
	// Separate read pool; a serialized writer preserves existing read/modify/write semantics.
	readCfg := cfg.Copy()
	readCfg.RuntimeParams["default_transaction_read_only"] = "on"
	readID := registerPGConfig(readCfg)
	s.pgReadConfigID = readID
	reader, err := sql.Open("arc-postgres", readID)
	if err != nil {
		return fail(safePGError(err))
	}
	reader.SetMaxOpenConns(8)
	reader.SetMaxIdleConns(2)
	s.db.DB = reader
	if initialize == nil {
		if err = s.syncMinAges(ctx); err != nil {
			return fail(err)
		}
	}
	s.sessionUses = newSessionUseQueue(s)
	success = true
	return s, nil
}

func (s *Store) migratePostgres(ctx context.Context, opts Options, initialize func(*sql.Tx) error) error {
	var version int
	if err := s.db.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::integer").Scan(&version); err != nil {
		return err
	}
	if version < 160000 || version >= 190000 {
		return errors.New("PostgreSQL 16–18 is required")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, "SELECT to_regclass('schema_migrations') IS NOT NULL").Scan(&exists); err != nil {
		return err
	}
	if exists {
		var marker sql.NullString
		if err := s.db.QueryRowContext(ctx, "SELECT fingerprint FROM arc_postgres_schema WHERE singleton=1").Scan(&marker); err != nil || marker.String != postgresFingerprint() {
			return errors.New("PostgreSQL schema marker differs; use matching Arc version or a verified portable backup")
		}

		if initialize != nil {
			return errors.New("PostgreSQL transfer destination must be empty; existing Arc data is never overwritten")
		}
		var n int
		if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&n); err != nil {
			return err
		}
		known, _ := migrationVersions()
		if n != len(known) {
			return errors.New("PostgreSQL schema differs from this Arc build; use the matching version and a safety backup")
		}
		rows, err := s.db.QueryContext(ctx, "SELECT version FROM schema_migrations")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var v string
			if err = rows.Scan(&v); err != nil {
				return err
			}
			if !known[v] {
				return errors.New("PostgreSQL schema is newer or unsupported")
			}
		}
		return rows.Err()
	}
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=current_schema() AND c.relkind IN ('r','v','m','S','f','p')").Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return errors.New("PostgreSQL destination must be a dedicated empty schema")
	}
	body, err := pgSchemaFS.ReadFile("postgres/baseline.sql")
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, string(body)); err != nil {
		return err
	}
	names, _ := migrationNames()
	for _, name := range names {
		if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", migrationVersion(name), time.Now().Unix()); err != nil {
			return err
		}
	}
	for k, v := range map[string]string{"server_id": NewID(), "background_epoch": fmt.Sprint(time.Now().Unix())} {
		if _, err = tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES(?,?)", k, v); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO arrival_clock(id,sequence) VALUES(1,0)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO arc_postgres_schema(singleton,fingerprint) VALUES(1,?)", postgresFingerprint()); err != nil {
		return err
	}
	if initialize != nil {
		if err = initialize(tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Backend reports the selected engine without disclosing its connection configuration.
func (s *Store) Backend() string {
	if s.backend == "postgres" {
		return "postgres"
	}
	return "sqlite"
}

// GeneratePostgresBaseline builds the checked-in schema from the complete SQLite migration
// history. Future schema changes must regenerate and add an explicit PostgreSQL upgrade path.
func GeneratePostgresBaseline(ctx context.Context, dir string) error {
	root, err := os.MkdirTemp("", "arc-schema-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	st, err := Open(ctx, filepath.Join(root, "schema.db"), Options{})
	if err != nil {
		return err
	}
	defer st.Close()
	sqlText, columns, err := postgresBaseline(ctx, st)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(columns, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "tables.json"), append(b, '\n'), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "baseline.sql"), []byte(sqlText), 0644)
}

func quoted(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func tableNames() []string {
	var a []string
	for k := range pgColumns {
		a = append(a, k)
	}
	sort.Strings(a)
	return a
}

func registerPGConfig(c *pgx.ConnConfig) string { return stdlib.RegisterConnConfig(c) }
func unregisterPGConfig(id string)              { stdlib.UnregisterConnConfig(id) }

func migrationFingerprint() string {
	h := sha256.New()
	names, _ := migrationNames()
	for _, name := range names {
		body, _ := migrationsFS.ReadFile(name)
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(body)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func postgresFingerprint() string {
	h := sha256.New()
	body, _ := pgSchemaFS.ReadFile("postgres/baseline.sql")
	columns, _ := pgSchemaFS.ReadFile("postgres/tables.json")
	h.Write(body)
	h.Write(columns)
	return hex.EncodeToString(h.Sum(nil))
}
