package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// UserData is one profile's watch state for an item.
type UserData struct {
	PositionMS   int64 `json:"position_ms,omitempty"`
	Played       bool  `json:"played"`
	PlayCount    int   `json:"play_count,omitempty"`
	LastPlayedAt int64 `json:"last_played_at,omitempty"`
}

// Watched thresholds, as in Plex: a resume point is kept after the first minute, and anything
// more than 90% through counts as watched.
const (
	minResumeMS    = 60_000
	watchedPercent = 0.90
)

// ReportProgress records how far a profile has got through an item. Passing the watched
// threshold marks it watched (once per viewing) and clears the resume point. It returns the new state.
func (s *Store) ReportProgress(ctx context.Context, userID, itemID string, positionMS, durationMS int64) (UserData, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return UserData{}, err
	}
	defer tx.Rollback()

	var d UserData
	var last *int64
	err = tx.QueryRowContext(ctx, `SELECT position_ms, played, play_count, last_played_at FROM user_item_data
		WHERE user_id = ? AND item_id = ?`, userID, itemID).Scan(&d.PositionMS, &d.Played, &d.PlayCount, &last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return UserData{}, err
	}
	now := time.Now().Unix()
	finished := durationMS > 0 && float64(positionMS) >= watchedPercent*float64(durationMS)
	switch {
	case finished:
		// Count it once: not again for the progress reports that follow in the same viewing.
		if d.PositionMS > 0 || !d.Played {
			d.Played = true
			d.PlayCount++
		}
		d.PositionMS = 0
	case positionMS >= minResumeMS:
		d.PositionMS = positionMS
	default:
		d.PositionMS = 0
	}
	d.LastPlayedAt = now
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_item_data (user_id, item_id, position_ms, played, play_count, last_played_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (user_id, item_id) DO UPDATE SET position_ms = excluded.position_ms, played = excluded.played,
			play_count = excluded.play_count, last_played_at = excluded.last_played_at`,
		userID, itemID, d.PositionMS, boolInt(d.Played), d.PlayCount, now); err != nil {
		return UserData{}, err
	}
	return d, tx.Commit()
}

// SetPlayed marks items watched or unwatched for a profile. Marking watched counts a play and
// clears the resume point; marking unwatched clears both.
func (s *Store) SetPlayed(ctx context.Context, userID string, itemIDs []string, played bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	for _, id := range itemIDs {
		var err error
		if played {
			_, err = tx.ExecContext(ctx, `
				INSERT INTO user_item_data (user_id, item_id, position_ms, played, play_count, last_played_at)
				VALUES (?, ?, 0, 1, 1, ?)
				ON CONFLICT (user_id, item_id) DO UPDATE SET position_ms = 0, played = 1,
					play_count = CASE WHEN played = 1 THEN play_count ELSE play_count + 1 END,
					last_played_at = excluded.last_played_at`, userID, id, now)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE user_item_data SET played = 0, position_ms = 0
				WHERE user_id = ? AND item_id = ?`, userID, id)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UserDataFor returns a profile's watch state for the given items (items never played are absent).
func (s *Store) UserDataFor(ctx context.Context, userID string, itemIDs []string) (map[string]UserData, error) {
	out := map[string]UserData{}
	for start := 0; start < len(itemIDs); start += 500 {
		batch := itemIDs[start:min(start+500, len(itemIDs))]
		args := []any{userID}
		for _, id := range batch {
			args = append(args, id)
		}
		rows, err := s.db.QueryContext(ctx, `SELECT item_id, position_ms, played, play_count, COALESCE(last_played_at, 0)
			FROM user_item_data WHERE user_id = ? AND item_id IN (`+placeholders(len(batch))+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var d UserData
			if err := rows.Scan(&id, &d.PositionMS, &d.Played, &d.PlayCount, &d.LastPlayedAt); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = d
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UnwatchedCounts returns, per series, how many of its episodes on disk a profile hasn't watched.
func (s *Store) UnwatchedCounts(ctx context.Context, userID string, seriesIDs []string) (map[string]int, error) {
	out := map[string]int{}
	if len(seriesIDs) == 0 {
		return out, nil
	}
	args := []any{userID}
	for _, id := range seriesIDs {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.parent_id, COUNT(*) FROM items i
		WHERE NOT EXISTS (SELECT 1 FROM user_item_data u WHERE u.user_id = ? AND u.item_id = i.id AND u.played = 1)
			AND i.parent_id IN (`+placeholders(len(seriesIDs))+`) AND i.kind = 'episode'
			AND EXISTS (SELECT 1 FROM item_files f JOIN media_files m ON m.id = f.file_id
				WHERE f.item_id = i.id AND m.missing_since IS NULL)
		GROUP BY i.parent_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return nil, err
		}
		out[id] = n
	}
	return out, rows.Err()
}

// InProgress lists the movies and episodes a profile is part-way through (with a file on disk),
// most recently watched first.
func (s *Store) InProgress(ctx context.Context, userID string, limit int) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+prefixCols("i.", itemCols)+` FROM user_item_data u
		JOIN items i ON i.id = u.item_id
		WHERE u.user_id = ? AND u.position_ms > 0 AND i.kind IN ('movie', 'episode')
			AND EXISTS (SELECT 1 FROM item_files f JOIN media_files m ON m.id = f.file_id
				WHERE f.item_id = i.id AND m.missing_since IS NULL)
		ORDER BY u.last_played_at DESC LIMIT ?`, userID, limit)
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

// WatchedSeries is a series a profile has watched episodes of, with the one watched most recently.
type WatchedSeries struct {
	SeriesID      string
	LastEpisodeID string
	LastPlayedAt  int64
}

// RecentlyWatchedSeries lists the series a profile has finished episodes of, most recent first.
func (s *Store) RecentlyWatchedSeries(ctx context.Context, userID string, limit int) ([]WatchedSeries, error) {
	// SQLite returns the other columns from the row that has the MAX.
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.parent_id, i.id, MAX(u.last_played_at) FROM user_item_data u JOIN items i ON i.id = u.item_id
		WHERE u.user_id = ? AND u.played = 1 AND i.kind = 'episode' AND u.last_played_at IS NOT NULL
		GROUP BY i.parent_id ORDER BY 3 DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchedSeries
	for rows.Next() {
		var w WatchedSeries
		if err := rows.Scan(&w.SeriesID, &w.LastEpisodeID, &w.LastPlayedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	return strings.Repeat("?,", n-1) + "?"
}

// DismissContinue removes items from a profile's Continue Watching until they're played again.
func (s *Store) DismissContinue(ctx context.Context, userID string, itemIDs []string) error {
	now := time.Now().Unix()
	for _, id := range itemIDs {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO continue_dismissed (user_id, item_id, at) VALUES (?, ?, ?)
			ON CONFLICT (user_id, item_id) DO UPDATE SET at = excluded.at`, userID, id, now); err != nil {
			return err
		}
	}
	return nil
}

// ContinueDismissed returns when a profile removed items from Continue Watching, by item id.
func (s *Store) ContinueDismissed(ctx context.Context, userID string) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT item_id, at FROM continue_dismissed WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id string
		var at int64
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out[id] = at
	}
	return out, rows.Err()
}
