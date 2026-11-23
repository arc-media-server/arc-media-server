package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed postgres/baseline.sql
var postgresBaseline string

var errPostgresMaintenance = errors.New("Arc backup, restore and optimization are unavailable in the experimental PostgreSQL store")
var errPostgresInUse = errors.New("PostgreSQL database is already in use by Arc")

// OpenPostgres is used by isolated integration tests until the backup and upgrade path is ready.
// It initializes dedicated empty databases and reopens the exact same Arc schema on restart.
func OpenPostgres(ctx context.Context, dsn string) (*Store, error) {
	names, err := migrationNames()
	if err != nil {
		return nil, err
	}
	versions := make([]string, len(names))
	for i, name := range names {
		versions[i] = migrationVersion(name)
	}
	if !strings.HasPrefix(postgresBaseline, "-- Arc migrations: "+strings.Join(versions, ",")+"\n") {
		return nil, errors.New("PostgreSQL baseline must be regenerated for this Arc schema")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid PostgreSQL connection configuration")
	}
	cfg.ConnectTimeout = 5 * time.Second
	cfg.RuntimeParams["search_path"] = "public"
	cfg.RuntimeParams["application_name"] = "arc-media-server"
	cfg.RuntimeParams["statement_timeout"] = "30000"
	cfg.RuntimeParams["idle_in_transaction_session_timeout"] = "60000"
	cfg.DefaultQueryExecMode = pgx.QueryExecModeExec

	writer := sql.OpenDB(postgresConnector{stdlib.GetConnector(*cfg, stdlib.OptionAfterConnect(lockPostgresWriter))})
	writer.SetMaxOpenConns(1)
	writer.SetMaxIdleConns(1)
	readConfig := cfg.Copy()
	readConfig.RuntimeParams["default_transaction_read_only"] = "on"
	reader := sql.OpenDB(postgresConnector{stdlib.GetConnector(*readConfig)})
	reader.SetMaxOpenConns(8)
	reader.SetMaxIdleConns(2)

	s := &Store{db: &database{DB: reader, writer: writer}, postgres: true}
	if err := initializePostgres(ctx, writer, versions); err != nil {
		s.Close()
		return nil, err
	}
	if err := s.syncMinAges(ctx); err != nil {
		s.Close()
		return nil, err
	}
	s.sessionUses = newSessionUseQueue(s)
	return s, nil
}

// The sole write connection owns the lock. A reconnect reacquires it before running any query.
func lockPostgresWriter(ctx context.Context, conn *pgx.Conn) error {
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(hashtextextended(current_database() || ':public', 308))").Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return errPostgresInUse
	}
	return nil
}

func initializePostgres(ctx context.Context, writer *sql.DB, versions []string) error {
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::integer").Scan(&version); err != nil {
		return err
	}
	if version < 160000 || version >= 200000 {
		return errors.New("the experimental store requires PostgreSQL 16 through 19")
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(postgresBaseline)))
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT to_regclass('public.arc_postgres_schema') IS NOT NULL").Scan(&exists); err != nil {
		return err
	}
	if exists {
		var actual string
		if err := tx.QueryRowContext(ctx, "SELECT fingerprint FROM arc_postgres_schema WHERE singleton=1").Scan(&actual); err != nil {
			return err
		}
		if actual != fingerprint {
			return errors.New("PostgreSQL schema differs from this Arc build")
		}
		return tx.Commit()
	}

	var objects int
	const existingObjects = `SELECT (SELECT count(*) FROM pg_class WHERE relnamespace='public'::regnamespace)
		+ (SELECT count(*) FROM pg_proc WHERE pronamespace='public'::regnamespace)
		+ (SELECT count(*) FROM pg_type WHERE typnamespace='public'::regnamespace)`
	if err := tx.QueryRowContext(ctx, existingObjects).Scan(&objects); err != nil {
		return err
	}
	if objects != 0 {
		return errors.New("PostgreSQL requires a dedicated empty database for first-time setup")
	}
	if _, err := tx.ExecContext(ctx, postgresBaseline); err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, version := range versions {
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", version, now); err != nil {
			return err
		}
	}
	for key, value := range map[string]string{"server_id": NewID(), "background_epoch": fmt.Sprint(now)} {
		if _, err := tx.ExecContext(ctx, "INSERT INTO settings(key,value) VALUES(?,?)", key, value); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO arrival_clock(id,sequence) VALUES(1,0)"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "CREATE TABLE arc_postgres_schema (singleton integer PRIMARY KEY CHECK (singleton=1), fingerprint text NOT NULL)"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO arc_postgres_schema(singleton,fingerprint) VALUES(1,?)", fingerprint); err != nil {
		return err
	}
	return tx.Commit()
}
