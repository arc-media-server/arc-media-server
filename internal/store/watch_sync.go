package store

import (
	"context"
	"database/sql"
	"errors"
)

// SetResumePosition changes only the saved position. Sync never starts a viewing,
// marks an item watched or updates the time it was last played.
func (s *Store) SetResumePosition(ctx context.Context, userID, itemID string, position int64) error {
	if position == 0 {
		_, err := s.db.ExecContext(ctx, `UPDATE user_item_data SET position_ms = 0 WHERE user_id = ? AND item_id = ?`, userID, itemID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_item_data (user_id, item_id, position_ms)
		VALUES (?, ?, ?) ON CONFLICT (user_id, item_id) DO UPDATE SET position_ms = excluded.position_ms`,
		userID, itemID, position)
	return err
}

// WatchViewing is one completed history entry. WatchedAt is the source timestamp
// for imported viewings and the start time of viewings recorded by Arc.
type WatchViewing struct {
	ID        int64  `json:"id"`
	ItemID    string `json:"item_id"`
	WatchedAt int64  `json:"watched_at"`
}

// WatchViewings lists completed history for visible items in descending ID order,
// so viewings added later with older timestamps can still be paged reliably.
func (s *Store) WatchViewings(ctx context.Context, userID string, acc Access, before int64, limit int) ([]WatchViewing, error) {
	q, args := acc.Where("i", `SELECT h.id, h.item_id, h.started_at FROM play_history h
		JOIN items i ON i.id = h.item_id WHERE h.user_id = ? AND h.completed = 1`, []any{userID})
	if before != 0 {
		q += ` AND h.id < ?`
		args = append(args, before)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY h.id DESC LIMIT ?`, append(args, limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WatchViewing{}
	for rows.Next() {
		var viewing WatchViewing
		if err := rows.Scan(&viewing.ID, &viewing.ItemID, &viewing.WatchedAt); err != nil {
			return nil, err
		}
		out = append(out, viewing)
	}
	return out, rows.Err()
}

// AddWatchViewing records a completed viewing once per item and timestamp. The
// single writer transaction prevents concurrent retries from recording it twice.
func (s *Store) AddWatchViewing(ctx context.Context, userID, itemID string, watchedAt int64) (WatchViewing, error) {
	viewing := WatchViewing{ItemID: itemID, WatchedAt: watchedAt}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return viewing, err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT id FROM play_history
		WHERE user_id = ? AND item_id = ? AND started_at = ? AND completed = 1 ORDER BY id LIMIT 1`,
		userID, itemID, watchedAt).Scan(&viewing.ID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `INSERT INTO play_history
			(user_id, item_id, kind, library_id, series_id, title, series_title, started_at, ended_at, completed, device)
			SELECT ?, i.id, i.kind, i.library_id, COALESCE(i.parent_id, ''), i.title, COALESCE(p.title, ''), ?, ?, 1, 'Imported'
			FROM items i LEFT JOIN items p ON p.id = i.parent_id WHERE i.id = ?
			RETURNING id`, userID, watchedAt, watchedAt, itemID).Scan(&viewing.ID)
	}
	if err != nil {
		return viewing, err
	}
	return viewing, tx.Commit()
}

// RemoveWatchViewing guards against recycled history IDs on SQLite by requiring
// the original item and timestamp as well as the profile and viewing ID.
func (s *Store) RemoveWatchViewing(ctx context.Context, userID string, viewing WatchViewing) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM play_history
		WHERE user_id = ? AND id = ? AND item_id = ? AND started_at = ? AND completed = 1`,
		userID, viewing.ID, viewing.ItemID, viewing.WatchedAt)
	return err
}
