// Package store owns the SQLite database: connection setup, migrations and all queries.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no CGO needed
)

// Options customize startup. PostgreSQL schema upgrades require BeforeMigrate.
type Options struct {
	// Log receives infrequent PostgreSQL schema-upgrade start/completion records.
	Log *slog.Logger
	// BeforeMigrate is called when an existing database is about to be upgraded to a new schema,
	// so a backup can be taken first. Opening fails if it returns an error. PostgreSQL
	// upgrades require this hook to successfully call BackupTo before DDL may start.
	BeforeMigrate func(s *Store) error
}

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	backend                 string
	pgLock                  *sql.Conn
	pgLockDB                *sql.DB
	pgConfigID              string
	db                      *database
	sessionUses             *sessionUseQueue
	pgReadConfigID          string
	pgBackupRelease         *postgresRelease
	pgUpgradeBackupVerified bool
}

// Open opens (or creates) the database at dbPath and applies pending migrations.
func Open(ctx context.Context, dbPath string, opts Options) (*Store, error) {
	// WAL lets playback reads continue while the scanner writes. openDatabase queues
	// in-process writers; immediate transactions reserve their lock before reading.
	// busy_timeout remains protection against brief contention outside this store.
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(10000)" +
		"&_pragma=synchronous(NORMAL)&_txlock=immediate"
	db, err := openDatabase(ctx, dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.checkVersion(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if opts.BeforeMigrate != nil {
		upgrading, err := s.upgrading(ctx)
		if err != nil {
			db.Close()
			return nil, err
		}
		if upgrading {
			if err := opts.BeforeMigrate(s); err != nil {
				db.Close()
				return nil, err
			}
		}
	}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	// Ages for parental controls follow the age ratings (and the rules for reading them).
	if err := s.syncMinAges(ctx); err != nil {
		db.Close()
		return nil, err
	}
	s.sessionUses = newSessionUseQueue(s)
	return s, nil
}

func (s *Store) Close() error {
	if s.sessionUses != nil {
		s.sessionUses.stop()
	}
	err := s.db.Close()
	if s.pgLock != nil {
		s.pgLock.Close()
	}
	if s.pgLockDB != nil {
		s.pgLockDB.Close()
	}
	if s.pgReadConfigID != "" {
		unregisterPGConfig(s.pgReadConfigID)
	}
	if s.pgConfigID != "" {
		unregisterPGConfig(s.pgConfigID)
	}
	return err
}

// DatabaseStats exposes queue pressure without querying SQL or revealing stored values.
func (s *Store) DatabaseStats() map[string]any {
	w, r := s.db.writer.Stats(), s.db.DB.Stats()
	return map[string]any{"writer_in_use": w.InUse, "write_wait_count": w.WaitCount,
		"write_wait_ms": w.WaitDuration.Milliseconds(), "read_connections": r.OpenConnections, "reads_in_use": r.InUse}
}

// Refuse downgrades before applying migrations or derived-data updates. Rollback
// requires the earlier binary and a backup from a schema that binary understands.
func (s *Store) checkVersion(ctx context.Context) error {
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type='table' AND name='schema_migrations'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	known, err := migrationVersions()
	if err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return err
		}
		if !known[version] {
			return fmt.Errorf("database uses a newer or unsupported Arc schema (%s); use the matching Arc version, or restore a compatible backup to roll back", version)
		}
	}
	return rows.Err()
}

func migrationNames() ([]string, error) {
	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	sort.Strings(names)
	return names, err
}

func migrationVersion(name string) string { return strings.TrimSuffix(path.Base(name), ".sql") }

// migrationVersions returns the schema versions this build knows.
func migrationVersions() (map[string]bool, error) {
	names, err := migrationNames()
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, n := range names {
		out[migrationVersion(n)] = true
	}
	return out, nil
}

// upgrading reports whether the database already has data and migrations still to apply.
func (s *Store) upgrading(ctx context.Context) (bool, error) {
	var applied int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&applied)
	if err != nil {
		return false, nil // a new, empty database
	}
	known, err := migrationVersions()
	if err != nil {
		return false, err
	}
	return applied > 0 && applied < len(known), nil
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		return err
	}
	names, err := migrationNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		version := migrationVersion(name)
		var applied int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, time.Now().Unix()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// NewID returns a random 128-bit id as 32 lowercase hex characters.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

// SortTitle lowercases a title and drops a leading English article.
func SortTitle(title string) string {
	t := strings.ToLower(strings.TrimSpace(title))
	for _, article := range []string{"the ", "a ", "an "} {
		if strings.HasPrefix(t, article) && len(t) > len(article) {
			return t[len(article):]
		}
	}
	return t
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
