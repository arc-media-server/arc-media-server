package store

import (
	"context"
	"path/filepath"
	"time"
)

// Other videos libraries hold home videos and the like: each file is a title of its own, named
// from the file and dated by it, never looked up online (metadata_state "locked"), and each folder
// is a collection.

// SetLocalVideo names and dates a home video (fields the user edited stay as they are) and marks
// it as one that isn't looked up online.
func (s *Store) SetLocalVideo(ctx context.Context, id, title, date string) error {
	it, err := s.GetItem(ctx, id)
	if err != nil || it == nil {
		return err
	}
	if it.MetadataState == "locked" && it.Title == title && it.ReleaseDate == date {
		return nil
	}
	year := 0
	if t, err := time.Parse(time.DateOnly, date); err == nil {
		year = t.Year()
	}
	// Written here rather than with UpdateMetadata (which leaves locked items alone), keeping
	// fields the user edited.
	_, err = s.db.ExecContext(ctx, `UPDATE items SET `+keepIfLocked("title")+`,
			sort_title = CASE WHEN `+lockedSQL("sort_title")+` OR `+lockedSQL("title")+` THEN sort_title ELSE ? END,
			`+keepIfLocked("release_date")+`, `+keepIfLocked("year")+`, metadata_state = 'locked', updated_at = ?
		WHERE id = ?`, title, SortTitle(title), date, year, time.Now().Unix(), id)
	return err
}

// SyncFolderCollections makes each folder of an Other videos library a collection of the videos
// in it (in the order given), keeping their ids, edits and artwork from scan to scan. Folders no
// longer there are removed when prune is set (every folder was read).
func (s *Store) SyncFolderCollections(ctx context.Context, libraryID int64, folders map[string][]string, prune bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	seen := map[string]bool{}
	for folder, ids := range folders {
		var id string
		err := tx.QueryRowContext(ctx, `SELECT id FROM collections WHERE library_id = ? AND kind = 'folder' AND filter = ?`,
			libraryID, folder).Scan(&id)
		if err != nil {
			id = newID()
			title := filepath.Base(folder)
			if _, err := tx.ExecContext(ctx, `INSERT INTO collections (id, library_id, kind, title, sort_title, filter, created_at)
				VALUES (?, ?, 'folder', ?, ?, ?, ?)`, id, libraryID, title, SortTitle(title), folder, time.Now().Unix()); err != nil {
				return err
			}
		}
		seen[id] = true
		if _, err := tx.ExecContext(ctx, `DELETE FROM collection_items WHERE collection_id = ?`, id); err != nil {
			return err
		}
		for i, item := range ids {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO collection_items (collection_id, item_id, position) VALUES (?, ?, ?)`,
				id, item, i); err != nil {
				return err
			}
		}
	}
	if prune {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM collections WHERE library_id = ? AND kind = 'folder'`, libraryID)
		if err != nil {
			return err
		}
		var gone []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			if !seen[id] {
				gone = append(gone, id)
			}
		}
		rows.Close()
		for _, id := range gone {
			if _, err := tx.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, id); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// LibraryVideo is an Other videos library's video with the path of one of its present files.
type LibraryVideo struct {
	Item Item
	Path string
}

// LibraryVideos lists a library's videos the profile can see, each with a present file, oldest
// first (by the date in the name or the file's), for browsing folder by folder.
func (s *Store) LibraryVideos(ctx context.Context, libraryID int64, acc Access) ([]LibraryVideo, error) {
	q := `SELECT ` + prefixCols("items.", itemCols) + `, MIN(m.path) FROM items
		JOIN item_files f ON f.item_id = items.id JOIN media_files m ON m.id = f.file_id
		WHERE items.library_id = ? AND items.kind = 'movie' AND m.missing_since IS NULL`
	args := []any{libraryID}
	q, args = acc.Where("items", q, args)
	rows, err := s.db.QueryContext(ctx, q+` GROUP BY items.id ORDER BY items.release_date, items.sort_title`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LibraryVideo
	for rows.Next() {
		var v LibraryVideo
		if err := scanItemWith(rows, &v.Item, &v.Path); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
