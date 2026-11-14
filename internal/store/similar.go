package store

import "context"

// SetRecommendations replaces TMDB's recommendations for a movie or show (TMDB ids, best first).
func (s *Store) SetRecommendations(ctx context.Context, itemID string, tmdbIDs []int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_recommendations WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	for i, id := range tmdbIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO item_recommendations (item_id, rank, tmdb_id) VALUES (?, ?, ?)`, itemID, i, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Similar lists movies or shows like one, from its own library, that a profile can see: TMDB's
// recommendations that are in the library first, then titles sharing its collection, genres and
// lead actors.
func (s *Store) Similar(ctx context.Context, it *Item, acc Access, limit int) ([]Item, error) {
	q := `SELECT ` + prefixCols("items.", itemCols) + ` FROM item_recommendations r
		JOIN items ON items.tmdb_id = r.tmdb_id AND items.library_id = ? AND items.kind = ?
		WHERE r.item_id = ? AND items.id <> ? AND ` + playableSQL
	q, args := acc.Where("items", q, []any{it.LibraryID, it.Kind, it.ID, it.ID})
	out, err := s.queryItems(ctx, q+` ORDER BY r.rank LIMIT ?`, append(args, limit)...)
	if err != nil || len(out) >= limit {
		return out, err
	}
	seen := map[string]bool{it.ID: true}
	for _, x := range out {
		seen[x.ID] = true
	}

	// Titles that share the most: the same collection, genres, then lead actors.
	q = `SELECT ` + itemCols + `, (
			CASE WHEN items.tmdb_collection_id <> 0 AND items.tmdb_collection_id = ? THEN 6 ELSE 0 END +
			2 * (SELECT COUNT(*) FROM item_genres g WHERE g.item_id = items.id AND g.name IN (SELECT name FROM item_genres WHERE item_id = ?)) +
			(SELECT COUNT(*) FROM item_people p WHERE p.item_id = items.id AND p.role = 'actor' AND p.ord < 10
				AND p.person_id IN (SELECT person_id FROM item_people WHERE item_id = ? AND role = 'actor' AND ord < 10))
		) AS score
		FROM items WHERE items.library_id = ? AND items.kind = ? AND items.id <> ? AND ` + playableSQL
	var collection int64
	s.db.QueryRowContext(ctx, `SELECT tmdb_collection_id FROM items WHERE id = ?`, it.ID).Scan(&collection)
	q, args = acc.Where("items", q, []any{collection, it.ID, it.ID, it.LibraryID, it.Kind, it.ID})
	rows, err := s.db.QueryContext(ctx, `SELECT * FROM (`+q+`) WHERE score > 2 ORDER BY score DESC, rating DESC LIMIT ?`, append(args, limit*2)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() && len(out) < limit {
		var x Item
		var score int
		if err := scanItemWith(rows, &x, &score); err != nil {
			return nil, err
		}
		if !seen[x.ID] {
			seen[x.ID] = true
			out = append(out, x)
		}
	}
	return out, rows.Err()
}
