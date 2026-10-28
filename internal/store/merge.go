package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

// MergeSeries makes several shows of one library one show, keeping the target's details, orders and
// artwork. Each source's episodes join it: one the target also has (the same episode key) gives
// the target's episode its files, and watch state, history, playlist and collection entries move
// with them; any other episode moves across as it is, keeping its id and so its history,
// markers and edits. Show-level watchlist, ratings, history, labels, playlists and collections
// move too. The sources' folder keys become aliases of the target, so later scans file their
// episodes under it, and the emptied sources (with their own orders) are removed. The target's
// orders are fetched again afterwards (the caller refreshes it) to place the episodes that joined.
func (s *Store) MergeSeries(ctx context.Context, targetID string, sourceIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	exec := func(q string, args ...any) error {
		_, err := tx.ExecContext(ctx, q, args...)
		return err
	}
	for _, src := range sourceIDs {
		if src == targetID {
			continue
		}
		// Later scans of the source's folders find the target; aliases it already had move too.
		if err := exec(`INSERT OR REPLACE INTO item_aliases (library_id, match_key, item_id)
				SELECT library_id, match_key, ?1 FROM items WHERE id = ?2`, targetID, src); err != nil {
			return err
		}
		if err := exec(`UPDATE item_aliases SET item_id = ?1 WHERE item_id = ?2`, targetID, src); err != nil {
			return err
		}
		// Each source episode, with the target's episode of the same key ("ep:<show>:<rest>") if any.
		rows, err := tx.QueryContext(ctx, `SELECT e.id, e.match_key, COALESCE(t.id, '') FROM items e
			LEFT JOIN items t ON t.library_id = e.library_id
				AND t.match_key = 'ep:' || ?1 || substr(e.match_key, length('ep:' || ?2) + 1)
			WHERE e.parent_id = ?2 AND e.kind = 'episode'`, targetID, src)
		if err != nil {
			return err
		}
		type episode struct{ id, key, twin string }
		var episodes []episode
		for rows.Next() {
			var e episode
			if err := rows.Scan(&e.id, &e.key, &e.twin); err != nil {
				rows.Close()
				return err
			}
			episodes = append(episodes, e)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		prefix := "ep:" + src + ":"
		for _, e := range episodes {
			if e.twin == "" {
				// Only the source has it: it moves across under the target's key.
				key := e.key
				if rest, ok := strings.CutPrefix(key, prefix); ok {
					key = "ep:" + targetID + ":" + rest
				}
				if err := exec(`UPDATE items SET parent_id = ?, match_key = ? WHERE id = ?`, targetID, key, e.id); err != nil {
					return err
				}
				continue
			}
			if err := mergeInto(ctx, tx, e.twin, e.id); err != nil {
				return err
			}
			if err := exec(`DELETE FROM items WHERE id = ?`, e.id); err != nil {
				return err
			}
		}
		// The show itself: watchlist, rating, labels, playlists, collections, extras and history.
		if err := mergeInto(ctx, tx, targetID, src); err != nil {
			return err
		}
		if err := exec(`UPDATE play_history SET series_id = ? WHERE series_id = ?`, targetID, src); err != nil {
			return err
		}
		// What's left (its own orders, display choices and season artwork) goes with it.
		if err := exec(`DELETE FROM items WHERE id = ?`, src); err != nil {
			return err
		}
	}
	if err := exec(`UPDATE items SET updated_at = ?, synced_at = 0 WHERE id = ?`, time.Now().Unix(), targetID); err != nil {
		return err
	}
	return tx.Commit()
}

// mergeInto moves one title's files and per-profile rows to another of the same kind. Where a
// profile has watch state on both, the one with more progress is kept: watched wins, then the
// higher play count, then whichever was played last; a rating or watchlist mark on either stays.
func mergeInto(ctx context.Context, tx *sql.Tx, targetID, sourceID string) error {
	steps := []string{
		`UPDATE media_files SET item_override = ?1 WHERE item_override = ?2`,
		`UPDATE OR IGNORE item_files SET item_id = ?1 WHERE item_id = ?2`,
		`INSERT INTO user_item_data (user_id, item_id, position_ms, played, play_count, last_played_at, user_rating, watchlisted_at)
			SELECT user_id, ?1, position_ms, played, play_count, last_played_at, user_rating, watchlisted_at
			FROM user_item_data WHERE item_id = ?2
			ON CONFLICT (user_id, item_id) DO UPDATE SET
				position_ms = CASE WHEN played = 0 AND excluded.played = 0 AND COALESCE(excluded.last_played_at, 0) > COALESCE(last_played_at, 0)
					THEN excluded.position_ms WHEN excluded.played > played THEN 0 ELSE position_ms END,
				played = MAX(played, excluded.played),
				play_count = MAX(play_count, excluded.play_count),
				last_played_at = MAX(COALESCE(last_played_at, 0), COALESCE(excluded.last_played_at, 0)),
				user_rating = CASE WHEN user_rating = 0 THEN excluded.user_rating ELSE user_rating END,
				watchlisted_at = COALESCE(watchlisted_at, excluded.watchlisted_at)`,
		`DELETE FROM user_item_data WHERE item_id = ?2`,
		`UPDATE OR IGNORE continue_dismissed SET item_id = ?1 WHERE item_id = ?2`,
		`UPDATE play_history SET item_id = ?1 WHERE item_id = ?2`,
		`UPDATE playlist_entries SET item_id = ?1 WHERE item_id = ?2`,
		`UPDATE OR IGNORE collection_items SET item_id = ?1 WHERE item_id = ?2`,
		`UPDATE OR IGNORE item_labels SET item_id = ?1 WHERE item_id = ?2`,
		`UPDATE extras SET item_id = ?1 WHERE item_id = ?2`,
	}
	for _, q := range steps {
		if _, err := tx.ExecContext(ctx, q, targetID, sourceID); err != nil {
			return err
		}
	}
	return nil
}

// SeriesAliases maps the folder keys merged into other shows of a library to those shows' own
// keys, so the scanner can serialize work on a merged show whichever folder it comes from.
func (s *Store) SeriesAliases(ctx context.Context, libraryID int64) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.match_key, i.match_key FROM item_aliases a JOIN items i ON i.id = a.item_id
		WHERE a.library_id = ?`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var alias, key string
		if err := rows.Scan(&alias, &key); err != nil {
			return nil, err
		}
		out[alias] = key
	}
	return out, rows.Err()
}
