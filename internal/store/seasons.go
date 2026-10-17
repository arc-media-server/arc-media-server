package store

import "context"

// SeasonArt is a season's poster: the provider's path and a poster file next to the media.
type SeasonArt struct {
	Path  string
	Local string
}

// SetSeasonPosters replaces a show's provider season posters (season number -> TMDB path).
func (s *Store) SetSeasonPosters(ctx context.Context, seriesID string, posters map[int]string) error {
	return s.setSeasonArt(ctx, seriesID, "poster_path", posters)
}

// SetSeasonLocalPosters replaces the season poster files found in a show's folder.
func (s *Store) SetSeasonLocalPosters(ctx context.Context, seriesID string, files map[int]string) error {
	return s.setSeasonArt(ctx, seriesID, "local_poster", files)
}

func (s *Store) setSeasonArt(ctx context.Context, seriesID, col string, values map[int]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE season_art SET `+col+` = '' WHERE series_id = ?`, seriesID); err != nil {
		return err
	}
	for n, v := range values {
		if v == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO season_art (series_id, season_number, `+col+`) VALUES (?, ?, ?)
			ON CONFLICT (series_id, season_number) DO UPDATE SET `+col+` = excluded.`+col, seriesID, n, v); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM season_art WHERE series_id = ? AND poster_path = '' AND local_poster = ''`, seriesID); err != nil {
		return err
	}
	return tx.Commit()
}

// SeasonPosters returns a show's season posters by season number.
func (s *Store) SeasonPosters(ctx context.Context, seriesID string) (map[int]SeasonArt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT season_number, poster_path, local_poster FROM season_art WHERE series_id = ?`, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]SeasonArt{}
	for rows.Next() {
		var n int
		var a SeasonArt
		if err := rows.Scan(&n, &a.Path, &a.Local); err != nil {
			return nil, err
		}
		out[n] = a
	}
	return out, rows.Err()
}

// RecentEpisodes lists a library's episodes with files present, newest first.
func (s *Store) RecentEpisodes(ctx context.Context, libraryID int64, maxAge, limit int) ([]Item, error) {
	q := `SELECT ` + itemCols + ` FROM items WHERE library_id = ? AND kind = 'episode' AND ` + playableSQL
	args := []any{libraryID}
	if maxAge > 0 {
		q += ` AND ` + AgeAllowedSQL("items")
		args = append(args, maxAge)
	}
	q += ` ORDER BY added_at DESC, release_date DESC LIMIT ?`
	return s.queryItems(ctx, q, append(args, limit)...)
}
