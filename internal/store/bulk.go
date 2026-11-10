package store

import (
	"context"
	"sort"
	"strings"
	"time"
)

// BulkEdit is a change made to many movies or shows at once. Zero values leave things alone.
type BulkEdit struct {
	AddGenres, RemoveGenres []string
	AddLabels, RemoveLabels []string
	// A new age rating for all of them (nil leaves each one's own).
	ContentRating *string
	// "all" locks every field, so refreshes leave them alone; "none" unlocks every field.
	Locks string
	// Watch is applied atomically with metadata. Episode IDs are resolved before writing.
	Watch *BulkWatch
}

// BulkWatch describes the profile changes accompanying one metadata edit.
type BulkWatch struct {
	UserID          string
	ItemIDs         []string
	Played          bool
	RemoveWatchlist []string
}

// EditItems applies a BulkEdit to movies and shows. Changed genres and age ratings are locked, as
// with a single edit.
func (s *Store) EditItems(ctx context.Context, ids []string, e BulkEdit) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, id := range ids {
		var locked string
		if err := tx.QueryRowContext(ctx, `SELECT locked_fields FROM items WHERE id = ? AND kind IN ('movie', 'series')`, id).Scan(&locked); err != nil {
			return err
		}
		lock := map[string]bool{}
		for _, f := range splitLocked(locked) {
			lock[f] = true
		}
		for _, g := range e.AddGenres {
			if _, err := tx.ExecContext(ctx, `INSERT INTO item_genres (item_id, name) SELECT ?, ?
				WHERE NOT EXISTS (SELECT 1 FROM item_genres WHERE item_id = ? AND name = ? COLLATE NOCASE)`, id, g, id, g); err != nil {
				return err
			}
		}
		for _, g := range e.RemoveGenres {
			if _, err := tx.ExecContext(ctx, `DELETE FROM item_genres WHERE item_id = ? AND name = ? COLLATE NOCASE`, id, g); err != nil {
				return err
			}
		}
		if len(e.AddGenres)+len(e.RemoveGenres) > 0 {
			lock["genres"] = true
		}
		for _, l := range e.AddLabels {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_labels (item_id, label) VALUES (?, ?)`, id, l); err != nil {
				return err
			}
		}
		for _, l := range e.RemoveLabels {
			if _, err := tx.ExecContext(ctx, `DELETE FROM item_labels WHERE item_id = ? AND label = ?`, id, l); err != nil {
				return err
			}
		}
		if e.ContentRating != nil {
			if _, err := tx.ExecContext(ctx, `UPDATE items SET content_rating = ?, min_age = ? WHERE id = ?`, *e.ContentRating, RatingAge(*e.ContentRating), id); err != nil {
				return err
			}
			lock["content_rating"] = true
		}
		switch e.Locks {
		case "all":
			for f := range LockableFields {
				lock[f] = true
			}
		case "none":
			lock = map[string]bool{}
		}
		fields := make([]string, 0, len(lock))
		for f := range lock {
			if LockableFields[f] != "" {
				fields = append(fields, f)
			}
		}
		sort.Strings(fields)
		if _, err := tx.ExecContext(ctx, `UPDATE items SET locked_fields = ?, updated_at = ? WHERE id = ?`,
			strings.Join(fields, ","), time.Now().Unix(), id); err != nil {
			return err
		}
	}
	if e.Watch != nil {
		if err := setPlayedTx(ctx, tx, e.Watch.UserID, e.Watch.ItemIDs, e.Watch.Played); err != nil {
			return err
		}
		if e.Watch.Played {
			for _, id := range e.Watch.RemoveWatchlist {
				if _, err := tx.ExecContext(ctx, `UPDATE user_item_data SET watchlisted_at = NULL WHERE user_id = ? AND item_id = ?`, e.Watch.UserID, id); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
