package store

import (
	"context"
	"time"
)

// Skip markers: where a file's intro, recap and credits are, for Skip Intro and Skip Credits.
// They're per file (timings differ between releases) and come from, best first: the user, the
// file's own chapters, online databases, or detection on this computer.

// Marker is one part of a file to offer skipping.
type Marker struct {
	Kind    string `json:"kind"` // intro, recap, credits
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
	// manual, chapters, theintrodb, introdb or detected.
	Source string `json:"source"`
}

// MarkerKinds are the parts that can be skipped, in play order.
var MarkerKinds = []string{"recap", "intro", "credits"}

// sourceRank orders sources: a higher one replaces a lower one, never the other way.
var sourceRank = map[string]int{"detected": 1, "introdb": 2, "theintrodb": 3, "chapters": 4, "manual": 5}

// FileMarkers returns a file's skip markers, in play order.
func (s *Store) FileMarkers(ctx context.Context, fileID int64) ([]Marker, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT kind, start_ms, end_ms, source FROM media_markers WHERE file_id = ? ORDER BY start_ms`, fileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Marker{}
	for rows.Next() {
		var m Marker
		if err := rows.Scan(&m.Kind, &m.StartMS, &m.EndMS, &m.Source); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SaveMarkers stores markers found for a file. Each replaces the file's marker of the same kind
// only if it comes from an equal or better source, so the user's own are never overwritten.
func (s *Store) SaveMarkers(ctx context.Context, fileID int64, markers []Marker) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, m := range markers {
		var source string
		err := tx.QueryRowContext(ctx, `SELECT source FROM media_markers WHERE file_id = ? AND kind = ?`, fileID, m.Kind).Scan(&source)
		if err == nil && sourceRank[source] > sourceRank[m.Source] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO media_markers (file_id, kind, start_ms, end_ms, source, updated_at)
			VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (file_id, kind) DO UPDATE SET
			start_ms = excluded.start_ms, end_ms = excluded.end_ms, source = excluded.source, updated_at = excluded.updated_at`,
			fileID, m.Kind, m.StartMS, m.EndMS, m.Source, time.Now().Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetManualMarkers replaces a file's markers with the user's own: kinds not listed are cleared
// (and stay cleared: a "none" marker of length zero records that there's nothing to skip).
func (s *Store) SetManualMarkers(ctx context.Context, fileID int64, markers []Marker) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	given := map[string]Marker{}
	for _, m := range markers {
		given[m.Kind] = m
	}
	now := time.Now().Unix()
	for _, kind := range MarkerKinds {
		m, ok := given[kind]
		if !ok {
			m = Marker{Kind: kind} // nothing to skip, and nothing found later replaces that
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO media_markers (file_id, kind, start_ms, end_ms, source, updated_at)
			VALUES (?, ?, ?, ?, 'manual', ?) ON CONFLICT (file_id, kind) DO UPDATE SET
			start_ms = excluded.start_ms, end_ms = excluded.end_ms, source = 'manual', updated_at = excluded.updated_at`,
			fileID, kind, m.StartMS, m.EndMS, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ResetMarkers forgets a file's markers (the user's too), so they're looked for again.
func (s *Store) ResetMarkers(ctx context.Context, fileID int64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM media_markers WHERE file_id = ?`, fileID); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE media_files SET markers_checked_at = 0,local_marker_generation=local_marker_generation+1 WHERE id = ?`, fileID)
	return err
}

// MarkerJob is an episode file to look for skip markers for.
type MarkerJob struct {
	ProbedAt   int64
	FileID     int64
	Path       string
	DurationMS int64
	EpisodeID  string
	SeriesID   string
	// The show's online ids, and the episode's place in TheTVDB's (else TMDB's) aired order.
	SeriesIMDB string
	SeriesTVDB int64
	SeriesTMDB int64
	Season     int
	Episode    int
	LibraryID  int64
	// A movie (only its credits are looked for); EpisodeID and SeriesID are then the movie's.
	Movie bool
}

// MarkerJobs lists present files whose markers haven't been looked for since a time (unix
// seconds), never-checked first: episodes in libraries that skip intros or credits, and movies in
// libraries that skip credits.
func (s *Store) MarkerJobs(ctx context.Context, before int64, limit int) ([]MarkerJob, error) {
	query := `
		SELECT * FROM (
		SELECT m.id, m.path, m.duration_ms, e.id, sr.id, sr.imdb_id, sr.tvdb_id, sr.tmdb_id, e.library_id,
			COALESCE((SELECT oe.season_number FROM ordering_entries oe JOIN orderings o ON o.id = oe.ordering_id
				WHERE oe.episode_id = e.id AND o.source IN ('tvdb_aired', 'tmdb_aired') ORDER BY o.source = 'tvdb_aired' DESC LIMIT 1), -1),
			COALESCE((SELECT oe.episode_number FROM ordering_entries oe JOIN orderings o ON o.id = oe.ordering_id
				WHERE oe.episode_id = e.id AND o.source IN ('tvdb_aired', 'tmdb_aired') ORDER BY o.source = 'tvdb_aired' DESC LIMIT 1), -1),
			0, m.markers_checked_at AS checked, sr.id AS grp, m.path AS p, COALESCE(m.probed_at,0)
		FROM media_files m JOIN item_files f ON f.file_id = m.id JOIN items e ON e.id = f.item_id AND e.kind = 'episode'
		JOIN items sr ON sr.id = e.parent_id
		JOIN libraries l ON l.id = e.library_id AND (l.skip_intros = 1 OR l.skip_credits = 1)
		WHERE m.missing_since IS NULL AND m.duration_ms > 0 AND m.markers_checked_at < ?
		UNION ALL
		SELECT m.id, m.path, m.duration_ms, mv.id, mv.id, mv.imdb_id, mv.tvdb_id, mv.tmdb_id, mv.library_id, -1, -1,
			1, m.markers_checked_at, mv.id, m.path, COALESCE(m.probed_at,0)
		FROM media_files m JOIN item_files f ON f.file_id = m.id JOIN items mv ON mv.id = f.item_id AND mv.kind = 'movie'
		JOIN libraries l ON l.id = mv.library_id AND l.skip_credits = 1 AND l.videos = 0
		WHERE m.missing_since IS NULL AND m.duration_ms > 0 AND m.markers_checked_at < ?
		 ) ORDER BY checked, grp, p`
	args := []any{before, before}
	if limit >= 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MarkerJob
	for rows.Next() {
		var j MarkerJob
		var checked int64
		var grp, p string
		if err := rows.Scan(&j.FileID, &j.Path, &j.DurationMS, &j.EpisodeID, &j.SeriesID, &j.SeriesIMDB, &j.SeriesTVDB,
			&j.SeriesTMDB, &j.LibraryID, &j.Season, &j.Episode, &j.Movie, &checked, &grp, &p, &j.ProbedAt); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// MarkersChecked records that a file's markers were looked for.
func (s *Store) MarkersChecked(ctx context.Context, fileID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE media_files SET markers_checked_at = ? WHERE id = ?`, time.Now().Unix(), fileID)
	return err
}

// SeasonFiles lists the present files of a show's episodes in one season of an aired order, for
// finding the intro they share.
func (s *Store) SeasonFiles(ctx context.Context, seriesID string, season int) ([]MarkerJob, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT m.id, m.path, m.duration_ms, e.id, oe.episode_number
		FROM ordering_entries oe JOIN orderings o ON o.id = oe.ordering_id
		JOIN items e ON e.id = oe.episode_id JOIN item_files f ON f.item_id = e.id JOIN media_files m ON m.id = f.file_id
		WHERE o.series_id = ? AND o.source = (SELECT source FROM orderings WHERE series_id = ? AND source IN ('tvdb_aired', 'tmdb_aired')
			ORDER BY source = 'tvdb_aired' DESC LIMIT 1)
			AND oe.season_number = ? AND m.missing_since IS NULL AND m.duration_ms > 0
		ORDER BY oe.episode_number`, seriesID, seriesID, season)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MarkerJob
	for rows.Next() {
		j := MarkerJob{SeriesID: seriesID, Season: season}
		if err := rows.Scan(&j.FileID, &j.Path, &j.DurationMS, &j.EpisodeID, &j.Episode); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// MarkerCoverage is how many of a title's present files have intro and credits markers, and how
// many have been looked at, so the owner can see where Skip Intro/Credits will appear.
type MarkerCoverage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Kind    string `json:"kind"`
	Files   int    `json:"files"`
	Intros  int    `json:"intros"`
	Credits int    `json:"credits"`
	Checked int    `json:"checked"`
}

// MarkerCoverage lists a library's shows (counting their episode files) and movies.
func (s *Store) MarkerCoverage(ctx context.Context, libraryID int64) ([]MarkerCoverage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.id, t.title, t.kind, COUNT(DISTINCT m.id),
			COUNT(DISTINCT CASE WHEN EXISTS (SELECT 1 FROM media_markers k WHERE k.file_id = m.id AND k.kind = 'intro') THEN m.id END),
			COUNT(DISTINCT CASE WHEN EXISTS (SELECT 1 FROM media_markers k WHERE k.file_id = m.id AND k.kind = 'credits') THEN m.id END),
			COUNT(DISTINCT CASE WHEN m.markers_checked_at > 0 THEN m.id END)
		FROM items t
		JOIN items i ON (t.kind = 'series' AND i.parent_id = t.id AND i.kind = 'episode') OR (t.kind = 'movie' AND i.id = t.id)
		JOIN item_files f ON f.item_id = i.id
		JOIN media_files m ON m.id = f.file_id AND m.missing_since IS NULL
		WHERE t.library_id = ? AND t.kind IN ('series', 'movie')
		GROUP BY t.id ORDER BY t.sort_title`, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []MarkerCoverage{}
	for rows.Next() {
		var c MarkerCoverage
		if err := rows.Scan(&c.ID, &c.Title, &c.Kind, &c.Files, &c.Intros, &c.Credits, &c.Checked); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
