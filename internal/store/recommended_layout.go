package store

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) RecommendedLayout(ctx context.Context, id int64) (string, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT layout FROM library_recommended WHERE library_id = ?`, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "[]", nil
	}
	return raw, err
}
func (s *Store) SetRecommendedLayout(ctx context.Context, id int64, raw string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO library_recommended (library_id, layout) VALUES (?, ?) ON CONFLICT(library_id) DO UPDATE SET layout=excluded.layout`, id, raw)
	return err
}
