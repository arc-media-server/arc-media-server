package store

import (
	"context"
	"time"
)

// IMDbRating is a title's IMDb rating (out of 10) and how many people voted.
type IMDbRating struct {
	Rating float64
	Votes  int
}

// IMDbIDs returns the IMDb ids of every movie and show.
func (s *Store) IMDbIDs(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT imdb_id FROM items WHERE imdb_id <> '' AND kind IN ('movie', 'series')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// SetIMDbRatings saves IMDb ratings by IMDb id; titles not in the map keep none.
func (s *Store) SetIMDbRatings(ctx context.Context, ratings map[string]IMDbRating) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for id, r := range ratings {
		if _, err := tx.ExecContext(ctx, `UPDATE items SET imdb_rating = ?, imdb_votes = ?
			WHERE imdb_id = ? AND kind IN ('movie', 'series') AND (imdb_rating <> ? OR imdb_votes <> ?)`,
			r.Rating, r.Votes, id, r.Rating, r.Votes); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ItemsNeedingCritics lists movies and shows with an IMDb id whose critics' scores were never
// fetched or were fetched before a time, never-fetched first.
func (s *Store) ItemsNeedingCritics(ctx context.Context, before int64, limit int) ([]Item, error) {
	return s.queryItems(ctx, `SELECT `+itemCols+` FROM items WHERE imdb_id <> '' AND kind IN ('movie', 'series')
		AND critics_at < ? AND `+playableSQL+` ORDER BY critics_at, added_at DESC LIMIT ?`, before, limit)
}

// SetCriticRatings saves a title's Rotten Tomatoes and Metacritic scores (nil for none).
func (s *Store) SetCriticRatings(ctx context.Context, id string, rottenTomatoes, metacritic *int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET rt_critics = ?, metacritic = ?, critics_at = ? WHERE id = ?`,
		rottenTomatoes, metacritic, time.Now().Unix(), id)
	return err
}
