package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"modernc.org/sqlite"
)

// BackupTo writes a consistent, compacted copy of the database to path, which must not exist.
// It's safe to call while AMS is in use.
func (s *Store) BackupTo(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("backup %s already exists", path)
	}
	// VACUUM INTO writes the destination, not the source. The normal read pool is query_only
	// (which forbids VACUUM); the writer pool would queue every scan/watch-state write behind it.
	reader, err := s.backupReader(ctx)
	if err != nil {
		return err
	}
	defer reader.Close()
	_, err = reader.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
}

// backupReader opens one dedicated read-only source connection. It has no permission to
// change the main database, but can create the separate VACUUM INTO destination.
func (s *Store) backupReader(ctx context.Context) (*sql.DB, error) {
	var source string
	if err := s.db.QueryRowContext(ctx, `SELECT file FROM pragma_database_list WHERE name = 'main'`).Scan(&source); err != nil {
		return nil, err
	}
	if source == "" {
		return nil, errors.New("backups require a file-backed database")
	}
	path := filepath.ToSlash(source)
	// Windows drive paths need a leading slash in file URIs.
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	location := url.URL{Scheme: "file", Path: path}
	query := url.Values{"mode": {"ro"}, "_pragma": {"busy_timeout(10000)"}}
	location.RawQuery = query.Encode()
	reader, err := sql.Open("sqlite", location.String())
	if err != nil {
		return nil, err
	}
	reader.SetMaxOpenConns(1)
	return reader, nil
}

// CheckBackup verifies that path is an AMS database this version can restore: it must pass
// SQLite's integrity check and not come from a newer AMS (with migrations this one doesn't know).
func CheckBackup(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&result); err != nil {
		return fmt.Errorf("not a readable database: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("backup is damaged: %s", result)
	}
	known, err := migrationVersions()
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return errors.New("not an Arc Media Server database")
	}
	defer rows.Close()
	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		if !known[v] {
			return fmt.Errorf("backup is from a newer version of Arc Media Server (%s)", v)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(applied) == 0 {
		return errors.New("not an Arc Media Server database: no applied migrations")
	}
	names, err := migrationNames()
	if err != nil {
		return err
	}
	// A supported older database has a continuous prefix of our migrations. Missing
	// entries would otherwise be replayed against an incompatible schema after restore.
	for _, name := range names[:len(applied)] {
		if !applied[migrationVersion(name)] {
			return errors.New("backup has an incomplete migration history")
		}
	}
	return nil
}

// RestoreFrom replaces the whole database with the backup at path, while AMS keeps running,
// then applies any migrations the backup predates. Call CheckBackup first.
func (s *Store) RestoreFrom(ctx context.Context, path string) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	err = conn.Raw(func(dc any) error {
		r, ok := dc.(interface {
			NewRestore(srcURI string) (*sqlite.Backup, error)
		})
		if !ok {
			return errors.New("database driver doesn't support restore")
		}
		b, err := r.NewRestore(path)
		if err != nil {
			return err
		}
		// Copy everything in one step: it holds the write lock, so nothing sees a half-restored state.
		if _, err := b.Step(-1); err != nil {
			b.Finish()
			return err
		}
		return b.Finish()
	})
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	// Release the writer before migrations/derived updates acquire it again.
	if err := conn.Close(); err != nil {
		return err
	}
	if err := s.migrate(ctx); err != nil {
		return err
	}
	// Match normal startup: older backups may predate the derived parental ages,
	// and rating interpretation may have changed since the backup was written.
	return s.syncMinAges(ctx)
}
