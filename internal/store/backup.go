package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"modernc.org/sqlite"
)

// ErrInvalidBackup is a content/version validation failure, not a storage/context error.
var ErrInvalidBackup = errors.New("not a valid Arc backup")

func backupReadError(ctx context.Context, err error, message string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var sqliteError *sqlite.Error
	if errors.As(err, &sqliteError) {
		switch sqliteError.Code() & 255 {
		case 1, 11, 26:
			return fmt.Errorf("%w: %s", ErrInvalidBackup, message) // ERROR, CORRUPT, NOTADB
		}
	}
	return err
}

// BackupTo writes a consistent, compacted copy of the database to path, which must not exist.
// It's safe to call while AMS is in use.
func (s *Store) BackupTo(ctx context.Context, path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("backup %s already exists", path)
	}
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path)
	return err
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
		return backupReadError(ctx, err, "not a readable database")
	}
	if result != "ok" {
		return fmt.Errorf("%w: backup is damaged", ErrInvalidBackup)
	}
	known, err := migrationVersions()
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return backupReadError(ctx, err, "not an Arc Media Server database")
	}
	defer rows.Close()
	applied := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: invalid migration history", ErrInvalidBackup)
		}
		if !known[v] {
			return fmt.Errorf("%w: backup is from a newer version of Arc Media Server", ErrInvalidBackup)
		}
		applied[v] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(applied) == 0 {
		return fmt.Errorf("%w: not an Arc Media Server database: no applied migrations", ErrInvalidBackup)
	}
	names, err := migrationNames()
	if err != nil {
		return err
	}
	// A supported older database has a continuous prefix of our migrations. Missing
	// entries would otherwise be replayed against an incompatible schema after restore.
	for _, name := range names[:len(applied)] {
		if !applied[migrationVersion(name)] {
			return fmt.Errorf("%w: backup has an incomplete migration history", ErrInvalidBackup)
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
