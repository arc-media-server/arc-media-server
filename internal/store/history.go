package store

import (
	"context"
	"strings"
)

// Play is one viewing of a movie or episode.
type Play struct {
	ID          int64  `json:"id"`
	UserID      string `json:"user_id"`
	ItemID      string `json:"item_id"`
	Kind        string `json:"kind"`
	LibraryID   int64  `json:"library_id"`
	SeriesID    string `json:"series_id,omitempty"`
	Title       string `json:"title"`
	SeriesTitle string `json:"series_title,omitempty"`
	StartedAt   int64  `json:"started_at"`
	EndedAt     int64  `json:"ended_at"`
	// Time actually spent playing, not paused.
	WatchedMS  int64  `json:"watched_ms"`
	PositionMS int64  `json:"position_ms"`
	DurationMS int64  `json:"duration_ms"`
	Completed  bool   `json:"completed"`
	Device     string `json:"device,omitempty"`
	Method     string `json:"method,omitempty"`
}

const playCols = `id, user_id, item_id, kind, library_id, series_id, title, series_title, started_at, ended_at,
	watched_ms, position_ms, duration_ms, completed, device, method`

// AddPlay records a viewing and returns its id.
func (s *Store) AddPlay(ctx context.Context, p Play) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO play_history (user_id, item_id, kind, library_id, series_id, title, series_title, started_at,
			ended_at, watched_ms, position_ms, duration_ms, completed, device, method)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.ItemID, p.Kind, p.LibraryID, p.SeriesID, p.Title, p.SeriesTitle, p.StartedAt, p.EndedAt,
		p.WatchedMS, p.PositionMS, p.DurationMS, boolInt(p.Completed), p.Device, p.Method)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdatePlay records how a viewing has gone on. Once completed it stays completed.
func (s *Store) UpdatePlay(ctx context.Context, p Play) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE play_history SET ended_at = ?, watched_ms = ?, position_ms = ?, duration_ms = ?,
			completed = MAX(completed, ?), method = CASE WHEN ? <> '' THEN ? ELSE method END
		WHERE id = ?`,
		p.EndedAt, p.WatchedMS, p.PositionMS, p.DurationMS, boolInt(p.Completed), p.Method, p.Method, p.ID)
	return err
}

// PlayFilter narrows the viewings Plays returns. Zero values don't filter.
type PlayFilter struct {
	UserID    string
	LibraryID int64
	// Started at or after From and before To (unix seconds).
	From, To int64
	// For paging back through history: only viewings with a smaller id.
	BeforeID int64
	Limit    int
}

// Plays lists viewings, newest first.
func (s *Store) Plays(ctx context.Context, f PlayFilter) ([]Play, error) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		where = append(where, cond)
		args = append(args, v)
	}
	if f.UserID != "" {
		add("user_id = ?", f.UserID)
	}
	if f.LibraryID != 0 {
		add("library_id = ?", f.LibraryID)
	}
	if f.From != 0 {
		add("started_at >= ?", f.From)
	}
	if f.To != 0 {
		add("started_at < ?", f.To)
	}
	if f.BeforeID != 0 {
		add("id < ?", f.BeforeID)
	}
	q := `SELECT ` + playCols + ` FROM play_history`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	q += ` ORDER BY started_at DESC, id DESC`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Play{}
	for rows.Next() {
		var p Play
		if err := rows.Scan(&p.ID, &p.UserID, &p.ItemID, &p.Kind, &p.LibraryID, &p.SeriesID, &p.Title, &p.SeriesTitle,
			&p.StartedAt, &p.EndedAt, &p.WatchedMS, &p.PositionMS, &p.DurationMS, &p.Completed, &p.Device, &p.Method); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// PlayYears lists the years a profile has viewings in, newest first (by local time).
func (s *Store) PlayYears(ctx context.Context, userID string) ([]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT CAST(strftime('%Y', started_at, 'unixepoch', 'localtime') AS INTEGER)
		FROM play_history WHERE user_id = ? ORDER BY 1 DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	return out, rows.Err()
}

// GenresFor returns the genres of movies and shows by id.
func (s *Store) GenresFor(ctx context.Context, ids []string) (map[string][]string, error) {
	out := map[string][]string{}
	for chunk := range chunks(ids, 500) {
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := s.db.QueryContext(ctx, `SELECT item_id, name FROM item_genres WHERE item_id IN (`+placeholders(len(chunk))+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = append(out[id], name)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// LeadActors returns the top-billed actors (up to five) of movies and shows by id.
func (s *Store) LeadActors(ctx context.Context, ids []string) (map[string][]Person, error) {
	out := map[string][]Person{}
	for chunk := range chunks(ids, 500) {
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := s.db.QueryContext(ctx, `
			SELECT ip.item_id, p.id, p.tmdb_id, p.name, p.profile_path FROM item_people ip JOIN people p ON p.id = ip.person_id
			WHERE ip.role = 'actor' AND ip.ord < 5 AND ip.item_id IN (`+placeholders(len(chunk))+`)
			ORDER BY ip.item_id, ip.ord`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var p Person
			if err := rows.Scan(&id, &p.ID, &p.TMDBID, &p.Name, &p.ProfilePath); err != nil {
				rows.Close()
				return nil, err
			}
			out[id] = append(out[id], p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func chunks(ids []string, n int) func(func([]string) bool) {
	return func(yield func([]string) bool) {
		for i := 0; i < len(ids); i += n {
			if !yield(ids[i:min(i+n, len(ids))]) {
				return
			}
		}
	}
}

// TitleSize is how much space a movie or show's files take, and whether anyone has watched it.
type TitleSize struct {
	ItemID    string
	LibraryID int64
	Kind      string
	Size      int64
	Files     int
	AddedAt   int64
	// Whether any profile has watched or started it (any episode, for a show), and when it was
	// last played (unix seconds, 0 if never).
	Watched    bool
	LastPlayed int64
}

// TitleSizes lists every movie and show with files present, with their size on disk.
func (s *Store) TitleSizes(ctx context.Context) ([]TitleSize, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH sizes AS (
			SELECT COALESCE(CASE WHEN i.kind = 'episode' THEN i.parent_id END, i.id) AS title_id,
				SUM(m.size) AS size, COUNT(*) AS files
			FROM items i JOIN item_files f ON f.item_id = i.id JOIN media_files m ON m.id = f.file_id
			WHERE m.missing_since IS NULL AND i.kind IN ('movie', 'episode')
			GROUP BY title_id
		), played AS (
			SELECT COALESCE(CASE WHEN i.kind = 'episode' THEN i.parent_id END, i.id) AS title_id,
				MAX(u.last_played_at) AS last
			FROM user_item_data u JOIN items i ON i.id = u.item_id
			WHERE u.played = 1 OR u.play_count > 0 OR u.position_ms > 0
			GROUP BY title_id
		), history AS (
			SELECT CASE WHEN series_id <> '' THEN series_id ELSE item_id END AS title_id, MAX(ended_at) AS last
			FROM play_history GROUP BY title_id
		)
		SELECT items.id, items.library_id, items.kind, sizes.size, sizes.files, items.added_at,
			played.title_id IS NOT NULL OR history.title_id IS NOT NULL,
			MAX(COALESCE(played.last, 0), COALESCE(history.last, 0))
		FROM sizes JOIN items ON items.id = sizes.title_id
			LEFT JOIN played ON played.title_id = items.id
			LEFT JOIN history ON history.title_id = items.id
		WHERE items.kind IN ('movie', 'series')
		ORDER BY sizes.size DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TitleSize{}
	for rows.Next() {
		var t TitleSize
		if err := rows.Scan(&t.ItemID, &t.LibraryID, &t.Kind, &t.Size, &t.Files, &t.AddedAt, &t.Watched, &t.LastPlayed); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ItemsByID returns items by id; ids that no longer exist are left out.
func (s *Store) ItemsByID(ctx context.Context, ids []string) (map[string]Item, error) {
	out := map[string]Item{}
	for chunk := range chunks(ids, 500) {
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		items, err := s.queryItems(ctx, `SELECT `+itemCols+` FROM items WHERE id IN (`+placeholders(len(chunk))+`)`, args...)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			out[it.ID] = it
		}
	}
	return out, nil
}
