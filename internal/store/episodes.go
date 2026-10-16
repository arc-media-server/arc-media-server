package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// GetItemByKey returns the item with matchKey in a library, or nil.
func (s *Store) GetItemByKey(ctx context.Context, libraryID int64, matchKey string) (*Item, error) {
	var it Item
	err := scanItem(s.db.QueryRowContext(ctx,
		`SELECT `+itemCols+` FROM items WHERE library_id = ? AND match_key = ?`, libraryID, matchKey), &it)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// RekeyItem changes an item's match key, e.g. when a file-only episode is identified by a provider.
// The item id, and therefore its watch history, is kept.
func (s *Store) RekeyItem(ctx context.Context, id, matchKey string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET match_key = ?, updated_at = ? WHERE id = ?`,
		matchKey, time.Now().Unix(), id)
	return err
}

// MarkSynced records that an item's provider metadata is up to date.
func (s *Store) MarkSynced(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET synced_at = ? WHERE id = ?`, time.Now().Unix(), id)
	return err
}

// SyncedAt returns when an item's metadata was last synced (unix seconds, 0 for never).
func (s *Store) SyncedAt(ctx context.Context, id string) (int64, error) {
	var t int64
	err := s.db.QueryRowContext(ctx, `SELECT synced_at FROM items WHERE id = ?`, id).Scan(&t)
	return t, err
}

// Children returns all items whose parent is id (a series' episodes), regardless of files.
func (s *Store) Children(ctx context.Context, id string) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+itemCols+` FROM items WHERE parent_id = ? ORDER BY release_date, sort_title`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// AvailableChildren returns the ids of a series' episodes that have at least one present file.
func (s *Store) AvailableChildren(ctx context.Context, id string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT i.id FROM items i
		JOIN item_files f ON f.item_id = i.id
		JOIN media_files m ON m.id = f.file_id
		WHERE i.parent_id = ? AND m.missing_since IS NULL`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var eid string
		if err := rows.Scan(&eid); err != nil {
			return nil, err
		}
		out[eid] = true
	}
	return out, rows.Err()
}

// LibraryItemsByKind lists every item of a kind in a library, regardless of files.
func (s *Store) LibraryItemsByKind(ctx context.Context, libraryID int64, kind string) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+itemCols+` FROM items WHERE library_id = ? AND kind = ? ORDER BY sort_title`, libraryID, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// Unmatched lists items in a library that need attention: unmatched series and movies, and
// episodes that only exist as files (not found in the provider's episode list).
func (s *Store) Unmatched(ctx context.Context, libraryID int64) ([]Item, error) {
	// Episodes of an unmatched series are left out: fixing the series fixes them.
	rows, err := s.db.QueryContext(ctx, `SELECT `+itemCols+` FROM items
		WHERE library_id = ? AND metadata_state = 'unmatched' AND (kind <> 'episode' OR parent_id IN
			(SELECT id FROM items WHERE library_id = ? AND kind = 'series' AND metadata_state <> 'unmatched'))
		ORDER BY kind, sort_title`, libraryID, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}
