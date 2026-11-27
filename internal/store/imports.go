package store

import (
	"context"
	"database/sql"
	"errors"
)

// Importing watch state from Plex and Jellyfin: titles are found by their TMDB, TheTVDB and IMDb
// ids (episodes by their own ids, or by their show and number), and what's imported is merged
// with what Arc already has, never overwriting newer progress.

// ImportRef identifies a movie or episode in another server by its online ids.
type ImportRef struct {
	Kind       string // movie, series or episode
	TMDB, TVDB int64
	IMDB       string
	// Episodes: their show's ids, and the episode's season and number (aired order).
	SeriesTMDB, SeriesTVDB int64
	SeriesIMDB             string
	Season, Episode        int
}

// Prefer present copies, then the newest catalog identity, with a deterministic tie-breaker.
// Keep unavailable matches as a fallback so imports can preserve offline watch history.
const importedPreference = ` ORDER BY ` + playableSQL + ` DESC, items.added_at DESC, items.id`

// importedEpisodeQuery is shared with the query-plan regression for large imports.
func importedEpisodeQuery(provider string) string {
	return `SELECT id FROM items WHERE kind = 'episode' AND ` + provider + `_id = ?` + importedPreference + ` LIMIT 1`
}

// FindImported returns the Arc movie, show or episode an imported one is, or "" if Arc doesn't have it.
func (s *Store) FindImported(ctx context.Context, r ImportRef) (string, error) {
	one := func(q string, args ...any) (string, error) {
		var id string
		err := s.db.QueryRowContext(ctx, q, args...).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return id, err
	}
	if r.Kind == "series" {
		for _, q := range []struct {
			col string
			v   any
			ok  bool
		}{{"tvdb_id", r.TVDB, r.TVDB != 0}, {"tmdb_id", r.TMDB, r.TMDB != 0}, {"imdb_id", r.IMDB, r.IMDB != ""}} {
			if !q.ok {
				continue
			}
			if id, err := one(`SELECT id FROM items WHERE kind = 'series' AND `+q.col+` = ?`+importedPreference+` LIMIT 1`, q.v); id != "" || err != nil {
				return id, err
			}
		}
		return "", nil
	}
	if r.Kind == "movie" {
		if r.TMDB != 0 {
			if id, err := one(`SELECT id FROM items WHERE kind = 'movie' AND tmdb_id = ?`+importedPreference+` LIMIT 1`, r.TMDB); id != "" || err != nil {
				return id, err
			}
		}
		if r.IMDB != "" {
			return one(`SELECT id FROM items WHERE kind = 'movie' AND imdb_id = ?`+importedPreference+` LIMIT 1`, r.IMDB)
		}
		return "", nil
	}

	// Use indexed episode IDs, including IDs mapped from the other provider.
	// The canonical match key is a storage identity, not the complete provider mapping.
	for _, p := range []struct {
		provider string
		id       int64
	}{{"tvdb", r.TVDB}, {"tmdb", r.TMDB}} {
		if p.id == 0 {
			continue
		}
		if id, err := one(importedEpisodeQuery(p.provider),
			p.id); id != "" || err != nil {
			return id, err
		}
	}
	// Otherwise by its show and its number in an aired order.
	if r.Season < 0 || r.Episode <= 0 {
		return "", nil
	}
	// Preserve the aired-provider preference, then prefer a present episode across
	// all matching shows. A show's other present episodes must not hide this target.
	return one(`SELECT items.id FROM ordering_entries e
		JOIN orderings o ON o.id = e.ordering_id
		JOIN items ON items.id = e.episode_id
		JOIN items series ON series.id = o.series_id
		WHERE series.kind = 'series'
		AND ((series.tvdb_id = ? AND ? <> 0) OR (series.tmdb_id = ? AND ? <> 0) OR (series.imdb_id = ? AND ? <> ''))
		AND o.source IN ('tvdb_aired', 'tmdb_aired') AND e.season_number = ? AND e.episode_number = ?
		ORDER BY o.source = 'tvdb_aired' DESC, `+playableSQL+` DESC,
		series.added_at DESC, items.added_at DESC, items.id LIMIT 1`,
		r.SeriesTVDB, r.SeriesTVDB, r.SeriesTMDB, r.SeriesTMDB, r.SeriesIMDB, r.SeriesIMDB, r.Season, r.Episode)
}

// ImportedWatch is a profile's state for one title on another server.
type ImportedWatch struct {
	Played     bool
	PlayCount  int
	LastPlayed int64 // unix seconds; 0 if unknown
	PositionMS int64
	// The profile's rating, 1–10 (0 for none).
	Rating int
}

// MergeImportedWatch adds imported state to a profile's own: watched if either says so, the
// higher play count and the later last play, a resume point only if Arc has none, and a rating
// only if the profile hasn't rated it.
func (s *Store) MergeImportedWatch(ctx context.Context, userID, itemID string, w ImportedWatch) error {
	played := 0
	if w.Played {
		played = 1
		w.PositionMS = 0
		w.PlayCount = max(w.PlayCount, 1)
	}
	var last any
	if w.LastPlayed > 0 {
		last = w.LastPlayed
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_item_data (user_id, item_id, position_ms, played, play_count, last_played_at, user_rating)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, item_id) DO UPDATE SET
			played = MAX(user_item_data.played, excluded.played),
			play_count = MAX(user_item_data.play_count, excluded.play_count),
			last_played_at = CASE WHEN excluded.last_played_at IS NOT NULL AND (user_item_data.last_played_at IS NULL OR excluded.last_played_at > user_item_data.last_played_at)
				THEN excluded.last_played_at ELSE user_item_data.last_played_at END,
			position_ms = CASE WHEN user_item_data.played = 0 AND excluded.played = 0 AND user_item_data.position_ms = 0 THEN excluded.position_ms
				WHEN excluded.played = 1 AND user_item_data.played = 0 THEN 0 ELSE user_item_data.position_ms END,
			user_rating = CASE WHEN user_item_data.user_rating = 0 THEN excluded.user_rating ELSE user_item_data.user_rating END`,
		userID, itemID, w.PositionMS, played, w.PlayCount, last, w.Rating)
	return err
}

// AddImportedPlay records a past viewing from another server in a profile's watch history, once:
// a viewing of the same title starting at the same second is already there.
func (s *Store) AddImportedPlay(ctx context.Context, userID, itemID string, at int64) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM play_history WHERE user_id = ? AND item_id = ? AND started_at = ?`,
		userID, itemID, at).Scan(&n); err != nil || n > 0 {
		return false, err
	}
	it, err := s.GetItem(ctx, itemID)
	if err != nil || it == nil {
		return false, err
	}
	p := Play{UserID: userID, ItemID: it.ID, Kind: it.Kind, LibraryID: it.LibraryID, Title: it.Title,
		StartedAt: at, EndedAt: at, Completed: true, Device: "Imported"}
	if it.Kind == "episode" {
		p.SeriesID = it.ParentID
		if sr, err := s.GetItem(ctx, it.ParentID); err == nil && sr != nil {
			p.SeriesTitle = sr.Title
		}
	}
	if f, err := s.ItemFiles(ctx, it.ID); err == nil && len(f) > 0 {
		p.DurationMS, p.WatchedMS, p.PositionMS = f[0].DurationMS, f[0].DurationMS, f[0].DurationMS
		p.EndedAt = at + f[0].DurationMS/1000
	}
	_, err = s.AddPlay(ctx, p)
	return err == nil, err
}

// ImportPlaylist adds titles to the profile's playlist of that name (made if it has none, or only a
// smart one), skipping titles already in it, so importing again doesn't repeat anything. It
// returns how many entries were added.
func (s *Store) ImportPlaylist(ctx context.Context, userID, title string, itemIDs []string) (int, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM playlists WHERE user_id = ? AND title = ? AND filter = ''
		ORDER BY created_at LIMIT 1`, userID, title).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		p, err := s.CreatePlaylist(ctx, userID, title)
		if err != nil {
			return 0, err
		}
		id = p.ID
	} else if err != nil {
		return 0, err
	}
	have := map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT item_id FROM playlist_entries WHERE playlist_id = ?`, id)
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var it string
		if err := rows.Scan(&it); err != nil {
			rows.Close()
			return 0, err
		}
		have[it] = true
	}
	rows.Close()
	var add []string
	for _, it := range itemIDs {
		if !have[it] {
			have[it] = true
			add = append(add, it)
		}
	}
	if len(add) == 0 {
		return 0, nil
	}
	return len(add), s.AddToPlaylist(ctx, id, add)
}

// ImportCollection adds titles to the library's manual collection of that name (made if it has
// none), keeping titles already in it where they are. It returns how many titles were added.
func (s *Store) ImportCollection(ctx context.Context, libraryID int64, title string, itemIDs []string) (int, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM collections WHERE library_id = ? AND kind = 'manual' AND title = ?
		ORDER BY created_at LIMIT 1`, libraryID, title).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		c := &Collection{LibraryID: libraryID, Kind: "manual", Title: title}
		if err := s.CreateCollection(ctx, c); err != nil {
			return 0, err
		}
		id = c.ID
	} else if err != nil {
		return 0, err
	}
	var before, after int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM collection_items WHERE collection_id = ?`, id).Scan(&before); err != nil {
		return 0, err
	}
	if err := s.AddToCollection(ctx, id, itemIDs); err != nil {
		return 0, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM collection_items WHERE collection_id = ?`, id).Scan(&after); err != nil {
		return 0, err
	}
	return after - before, nil
}
