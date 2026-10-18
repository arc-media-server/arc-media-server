package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// FileItemOverride returns the title a user put a file under (by merging or splitting), or "" when
// the scanner should group it by its name as usual.
func (s *Store) FileItemOverride(ctx context.Context, fileID int64) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT m.item_override FROM media_files m JOIN items i ON i.id = m.item_override
		WHERE m.id = ?`, fileID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// MergeItems makes several movies one: the sources' files become versions of the target, their
// watch state, history, ratings, playlist entries, collections and labels move to it, and the
// emptied sources are removed. Files stay with the target on later scans.
func (s *Store) MergeItems(ctx context.Context, targetID string, sourceIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, src := range sourceIDs {
		if src == targetID {
			continue
		}
		steps := []string{
			// Files: keep each one's part, and remember the choice.
			`UPDATE media_files SET item_override = ?1 WHERE id IN (SELECT file_id FROM item_files WHERE item_id = ?2)`,
			`UPDATE OR IGNORE item_files SET item_id = ?1 WHERE item_id = ?2`,
			// Per-profile state: where both have some, the target's is kept.
			`UPDATE OR IGNORE user_item_data SET item_id = ?1 WHERE item_id = ?2`,
			`UPDATE OR IGNORE continue_dismissed SET item_id = ?1 WHERE item_id = ?2`,
			`UPDATE play_history SET item_id = ?1 WHERE item_id = ?2`,
			`UPDATE playlist_entries SET item_id = ?1 WHERE item_id = ?2`,
			`UPDATE OR IGNORE collection_items SET item_id = ?1 WHERE item_id = ?2`,
			`UPDATE OR IGNORE item_labels SET item_id = ?1 WHERE item_id = ?2`,
			`UPDATE extras SET item_id = ?1 WHERE item_id = ?2`,
		}
		for _, q := range steps {
			if _, err := tx.ExecContext(ctx, q, targetID, src); err != nil {
				return err
			}
		}
		// What's left of the source (and any rows the target already had) goes with it.
		if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, src); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE items SET updated_at = ? WHERE id = ?`, time.Now().Unix(), targetID); err != nil {
		return err
	}
	return tx.Commit()
}

// SplitItem gives every version of a movie but the first its own title (parts of a multi-part
// movie stay together), and returns the new titles' ids. They're identified online at the next scan.
func (s *Store) SplitItem(ctx context.Context, it *Item, title func(path string) (string, int)) ([]string, error) {
	files, err := s.ItemFiles(ctx, it.ID)
	if err != nil {
		return nil, err
	}
	var versions []MediaFile
	for _, f := range files {
		if f.Part <= 1 {
			versions = append(versions, f)
		}
	}
	if len(versions) < 2 {
		return nil, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	var ids []string
	for _, f := range versions[1:] {
		id := NewID()
		name, year := title(f.Path)
		if name == "" {
			name = it.Title
		}
		// A key of its own, so the file's name can't group it back with the original.
		if _, err := tx.ExecContext(ctx, `INSERT INTO items (id, library_id, kind, match_key, title, sort_title, year,
				added_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, it.LibraryID, it.Kind, "split:"+id, name, SortTitle(name), year, now, now); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE item_files SET item_id = ? WHERE item_id = ? AND file_id = ?`, id, it.ID, f.ID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE media_files SET item_override = ? WHERE id = ?`, id, f.ID); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	// The original keeps its other files where they are.
	if _, err := tx.ExecContext(ctx, `UPDATE media_files SET item_override = ? WHERE id = ?`, it.ID, versions[0].ID); err != nil {
		return nil, err
	}
	return ids, tx.Commit()
}
