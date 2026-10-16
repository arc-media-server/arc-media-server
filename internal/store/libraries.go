package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SyncLibrary creates or updates a library by name and replaces its folder list.
func (s *Store) SyncLibrary(ctx context.Context, name, kind, sourceOrdering string, paths []string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO libraries (name, kind, source_ordering, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET kind = excluded.kind, source_ordering = excluded.source_ordering
		RETURNING id`, name, kind, sourceOrdering, time.Now().Unix()).Scan(&id)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM library_paths WHERE library_id = ?`, id); err != nil {
		return 0, err
	}
	for _, p := range paths {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO library_paths (library_id, path) VALUES (?, ?)`, id, p); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

func (s *Store) ListLibraries(ctx context.Context) ([]Library, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, kind, source_ordering FROM libraries ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var libs []Library
	for rows.Next() {
		var l Library
		if err := rows.Scan(&l.ID, &l.Name, &l.Kind, &l.SourceOrdering); err != nil {
			rows.Close()
			return nil, err
		}
		libs = append(libs, l)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range libs {
		if libs[i].Paths, err = s.libraryPaths(ctx, libs[i].ID); err != nil {
			return nil, err
		}
	}
	return libs, nil
}

// GetLibrary returns nil when no library has the given id.
func (s *Store) GetLibrary(ctx context.Context, id int64) (*Library, error) {
	var l Library
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, kind, source_ordering FROM libraries WHERE id = ?`, id).
		Scan(&l.ID, &l.Name, &l.Kind, &l.SourceOrdering)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if l.Paths, err = s.libraryPaths(ctx, id); err != nil {
		return nil, err
	}
	return &l, nil
}

func (s *Store) libraryPaths(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT path FROM library_paths WHERE library_id = ? ORDER BY path`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}
