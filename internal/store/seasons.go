package store

import (
	"context"
	"database/sql"
	"errors"
)

// SeasonArt is a season's poster: the provider's path, a poster file next to the media, and one
// the user chose (a provider path or an upload), which wins.
type SeasonArt struct {
	Path   string
	Local  string
	Chosen string
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
	if _, err := tx.ExecContext(ctx, `DELETE FROM season_art WHERE series_id = ? AND poster_path = '' AND local_poster = '' AND chosen_path = ''`, seriesID); err != nil {
		return err
	}
	return tx.Commit()
}

// SetSeasonChosen sets the poster the user chose for a season ("" to go back to automatic) and
// returns the one it replaces.
func (s *Store) SetSeasonChosen(ctx context.Context, seriesID string, season int, path string) (old string, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, `SELECT chosen_path FROM season_art WHERE series_id = ? AND season_number = ?`, seriesID, season).Scan(&old)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO season_art (series_id, season_number, chosen_path) VALUES (?, ?, ?)
		ON CONFLICT (series_id, season_number) DO UPDATE SET chosen_path = excluded.chosen_path`, seriesID, season, path); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM season_art WHERE series_id = ? AND poster_path = '' AND local_poster = '' AND chosen_path = ''`, seriesID); err != nil {
		return "", err
	}
	return old, tx.Commit()
}

// SeasonPosters returns a show's season posters by season number.
func (s *Store) SeasonPosters(ctx context.Context, seriesID string) (map[int]SeasonArt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT season_number, poster_path, local_poster, chosen_path FROM season_art WHERE series_id = ?`, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]SeasonArt{}
	for rows.Next() {
		var n int
		var a SeasonArt
		if err := rows.Scan(&n, &a.Path, &a.Local, &a.Chosen); err != nil {
			return nil, err
		}
		out[n] = a
	}
	return out, rows.Err()
}

// RecentEpisodes lists a library's episodes with files present, newest first.
func (s *Store) RecentEpisodes(ctx context.Context, libraryID int64, acc Access, limit int) ([]Item, error) {
	q := `SELECT ` + itemCols + ` FROM items WHERE library_id = ? AND kind = 'episode' AND ` + playableSQL
	args := []any{libraryID}
	q, args = acc.Where("items", q, args)
	q += ` ORDER BY added_at DESC, release_date DESC LIMIT ?`
	return s.queryItems(ctx, q, append(args, limit)...)
}
