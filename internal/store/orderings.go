package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"
)

type Ordering struct {
	ID          int64  `json:"id"`
	SeriesID    string `json:"series_id"`
	Source      string `json:"source"`
	SourceRef   string `json:"source_ref,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Episodes    int    `json:"episode_count"`
}

type OrderingEntry struct {
	EpisodeID     string `json:"episode_id"`
	Season        int    `json:"season_number"`
	Episode       int    `json:"episode_number"`
	Locked        bool   `json:"locked,omitempty"`
	GroupSeason   int    `json:"-"` // Display container; source Season/Episode remain unchanged.
	InlineSpecial bool   `json:"inline_special,omitempty"`
}

// UpsertOrdering creates or renames the ordering identified by (series, source, ref).
func (s *Store) UpsertOrdering(ctx context.Context, seriesID, source, ref, name, description string) (int64, error) {
	var id int64
	err := s.db.WriteRowContext(ctx, `
		INSERT INTO orderings (series_id, source, source_ref, name, description, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (series_id, source, source_ref) DO UPDATE SET
			name = excluded.name, description = excluded.description, updated_at = excluded.updated_at
		RETURNING id`, seriesID, source, ref, name, description, time.Now().Unix()).Scan(&id)
	return id, err
}

// ReplaceOrdering rewrites an ordering's seasons (names and summaries) and automatic entries.
// Season fields the user edited are kept, as are locked (manually set) entries; episodes that have
// one are not given an automatic position.
func (s *Store) ReplaceOrdering(ctx context.Context, orderingID int64, seasons, overviews map[int]string, entries []OrderingEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	edited := map[int]Season{}
	rows, err := tx.QueryContext(ctx, `SELECT season_number, name, overview, locked_fields FROM ordering_seasons
		WHERE ordering_id = ? AND locked_fields <> ''`, orderingID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var se Season
		var locked string
		if err := rows.Scan(&se.Number, &se.Name, &se.Overview, &locked); err != nil {
			rows.Close()
			return err
		}
		se.LockedFields = splitLocked(locked)
		edited[se.Number] = se
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM ordering_seasons WHERE ordering_id = ?`, orderingID); err != nil {
		return err
	}
	all := map[int]bool{}
	for n := range seasons {
		all[n] = true
	}
	for n := range edited {
		all[n] = true
	}
	for num := range all {
		se := Season{Number: num, Name: seasons[num], Overview: overviews[num]}
		if e, ok := edited[num]; ok {
			se.LockedFields = e.LockedFields
			if slices.Contains(e.LockedFields, "name") {
				se.Name = e.Name
			}
			if slices.Contains(e.LockedFields, "overview") {
				se.Overview = e.Overview
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO ordering_seasons (ordering_id, season_number, name, overview, locked_fields) VALUES (?, ?, ?, ?, ?)`,
			orderingID, num, se.Name, se.Overview, strings.Join(se.LockedFields, ",")); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM ordering_entries WHERE ordering_id = ? AND locked = 0`, orderingID); err != nil {
		return err
	}
	locked := map[string]bool{}
	rows, err = tx.QueryContext(ctx, `SELECT episode_id FROM ordering_entries WHERE ordering_id = ?`, orderingID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		locked[id] = true
	}
	rows.Close()
	for _, e := range entries {
		if locked[e.EpisodeID] {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO ordering_entries (ordering_id, episode_id, season_number, episode_number, locked)
			VALUES (?, ?, ?, ?, ?)`, orderingID, e.EpisodeID, e.Season, e.Episode, boolInt(e.Locked)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE orderings SET updated_at = ? WHERE id = ?`, time.Now().Unix(), orderingID); err != nil {
		return err
	}
	return tx.Commit()
}

const orderingCols = `o.id, o.series_id, o.source, o.source_ref, o.name, o.description,
	(SELECT COUNT(DISTINCT episode_id) FROM ordering_entries e WHERE e.ordering_id = o.id)`

func scanOrdering(row interface{ Scan(...any) error }, o *Ordering) error {
	return row.Scan(&o.ID, &o.SeriesID, &o.Source, &o.SourceRef, &o.Name, &o.Description, &o.Episodes)
}

// ListOrderings returns a series' orderings: provider orders first, then custom ones.
func (s *Store) ListOrderings(ctx context.Context, seriesID string) ([]Ordering, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+orderingCols+` FROM orderings o WHERE o.series_id = ?
		ORDER BY CASE o.source
			WHEN 'tvdb_aired' THEN 0 WHEN 'tvdb_dvd' THEN 1 WHEN 'tvdb_absolute' THEN 2
			WHEN 'tmdb_aired' THEN 4 WHEN 'tmdb_group' THEN 5 WHEN 'file' THEN 6 WHEN 'custom' THEN 7
			ELSE 3 END, o.name`, seriesID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ordering{}
	for rows.Next() {
		var o Ordering
		if err := scanOrdering(rows, &o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetOrdering returns nil when the ordering doesn't exist.
func (s *Store) GetOrdering(ctx context.Context, id int64) (*Ordering, error) {
	var o Ordering
	err := scanOrdering(s.db.QueryRowContext(ctx, `SELECT `+orderingCols+` FROM orderings o WHERE o.id = ?`, id), &o)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// FindOrdering returns the id of a series' ordering with the given source, or 0.
func (s *Store) FindOrdering(ctx context.Context, seriesID, source, ref string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM orderings WHERE series_id = ? AND source = ? AND source_ref = ?`,
		seriesID, source, ref).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func (s *Store) OrderingEntries(ctx context.Context, orderingID int64) ([]OrderingEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT episode_id, season_number, episode_number, locked FROM ordering_entries
		WHERE ordering_id = ? ORDER BY season_number, episode_number`, orderingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OrderingEntry{}
	for rows.Next() {
		var e OrderingEntry
		if err := rows.Scan(&e.EpisodeID, &e.Season, &e.Episode, &e.Locked); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Season is a season of one ordering: its name and summary, and the fields the user edited.
type Season struct {
	Number       int      `json:"season_number"`
	Name         string   `json:"name"`
	Overview     string   `json:"overview,omitempty"`
	LockedFields []string `json:"locked_fields"`
}

// SeasonDetails returns an ordering's seasons by number.
func (s *Store) SeasonDetails(ctx context.Context, orderingID int64) (map[int]Season, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT season_number, name, overview, locked_fields FROM ordering_seasons WHERE ordering_id = ?`, orderingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]Season{}
	for rows.Next() {
		var se Season
		var locked string
		if err := rows.Scan(&se.Number, &se.Name, &se.Overview, &locked); err != nil {
			return nil, err
		}
		se.LockedFields = splitLocked(locked)
		out[se.Number] = se
	}
	return out, rows.Err()
}

// EditSeason sets a season's name and summary by hand in one ordering; fields sent are locked, and
// locked replaces the locked set when not nil.
func (s *Store) EditSeason(ctx context.Context, orderingID int64, season int, name, overview *string, locked []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var se Season
	var current string
	err = tx.QueryRowContext(ctx, `SELECT name, overview, locked_fields FROM ordering_seasons WHERE ordering_id = ? AND season_number = ?`,
		orderingID, season).Scan(&se.Name, &se.Overview, &current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	lock := map[string]bool{}
	if locked != nil {
		for _, f := range locked {
			if f != "name" && f != "overview" {
				return errors.New("unknown season field: " + f)
			}
			lock[f] = true
		}
	} else {
		for _, f := range splitLocked(current) {
			lock[f] = true
		}
	}
	if name != nil {
		se.Name, lock["name"] = *name, true
	}
	if overview != nil {
		se.Overview, lock["overview"] = *overview, true
	}
	var fields []string
	for _, f := range []string{"name", "overview"} {
		if lock[f] {
			fields = append(fields, f)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO ordering_seasons (ordering_id, season_number, name, overview, locked_fields)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (ordering_id, season_number) DO UPDATE SET name = excluded.name, overview = excluded.overview,
			locked_fields = excluded.locked_fields`, orderingID, season, se.Name, se.Overview, strings.Join(fields, ",")); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) OrderingSeasons(ctx context.Context, orderingID int64) (map[int]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT season_number, name FROM ordering_seasons WHERE ordering_id = ?`, orderingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var n int
		var name string
		if err := rows.Scan(&n, &name); err != nil {
			return nil, err
		}
		out[n] = name
	}
	return out, rows.Err()
}

// SetEntry manually places an episode in an ordering, replacing any position it had there.
func (s *Store) SetEntry(ctx context.Context, orderingID int64, episodeID string, season, episode int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM ordering_entries WHERE ordering_id = ? AND episode_id = ?`, orderingID, episodeID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ordering_entries (ordering_id, episode_id, season_number, episode_number, locked)
		VALUES (?, ?, ?, ?, 1)`, orderingID, episodeID, season, episode); err != nil {
		return err
	}
	return tx.Commit()
}

// AddEntry places an episode in an ordering if that exact position isn't already set.
func (s *Store) AddEntry(ctx context.Context, orderingID int64, episodeID string, season, episode int) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO ordering_entries (ordering_id, episode_id, season_number, episode_number, locked)
		VALUES (?, ?, ?, ?, 0)`, orderingID, episodeID, season, episode)
	return err
}

// ClearEntry removes an episode's position from an ordering. For provider orderings the
// automatic position returns on the next metadata refresh.
func (s *Store) ClearEntry(ctx context.Context, orderingID int64, episodeID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM ordering_entries WHERE ordering_id = ? AND episode_id = ?`, orderingID, episodeID)
	return err
}

// CopyOrdering creates a custom ordering for the same series with all entries of src, locked.
func (s *Store) CopyOrdering(ctx context.Context, src *Ordering, name string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO orderings (series_id, source, source_ref, name, description, updated_at)
		VALUES (?, 'custom', ?, ?, 'Custom', ?) RETURNING id`,
		src.SeriesID, NewID(), name, time.Now().Unix()).Scan(&id)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ordering_seasons (ordering_id, season_number, name)
		SELECT ?, season_number, name FROM ordering_seasons WHERE ordering_id = ?`, id, src.ID); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ordering_entries (ordering_id, episode_id, season_number, episode_number, locked)
		SELECT ?, episode_id, season_number, episode_number, 1 FROM ordering_entries WHERE ordering_id = ?`,
		id, src.ID); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) DeleteOrdering(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM orderings WHERE id = ?`, id)
	return err
}

// DisplayOrdering returns the ordering a user sees for a series: their own choice, else the
// server default, else the ordering matching the library's source ordering, else the first one.
// It returns 0 when the series has no orderings.
func (s *Store) DisplayOrdering(ctx context.Context, seriesID, userID, librarySource string) (int64, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT ordering_id FROM series_display WHERE series_id = ? AND user_id IN (?, '')
		ORDER BY user_id = '' LIMIT 1`, seriesID, userID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	err = s.db.QueryRowContext(ctx, `
		SELECT id FROM orderings WHERE series_id = ?
		ORDER BY source = ? DESC, source = 'tmdb_aired' DESC, source = 'file' DESC, id LIMIT 1`,
		seriesID, librarySource).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// SetDisplayOrdering sets which ordering a series is shown in. userID "" sets the server default.
func (s *Store) SetDisplayOrdering(ctx context.Context, seriesID, userID string, orderingID int64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO series_display (series_id, user_id, ordering_id) VALUES (?, ?, ?)
		ON CONFLICT (series_id, user_id) DO UPDATE SET ordering_id = excluded.ordering_id`,
		seriesID, userID, orderingID)
	return err
}

// DisplaysProvider reports whether any user (or the server default) displays the series in an
// ordering from the given provider ("tvdb" or "tmdb").
func (s *Store) DisplaysProvider(ctx context.Context, seriesID, provider string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM series_display d JOIN orderings o ON o.id = d.ordering_id
		WHERE d.series_id = ? AND o.source LIKE ? || '%'`, seriesID, provider).Scan(&n)
	return n > 0, err
}

// ClearDisplayOrdering removes a display choice so the next level of default applies.
func (s *Store) ClearDisplayOrdering(ctx context.Context, seriesID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM series_display WHERE series_id = ? AND user_id = ?`, seriesID, userID)
	return err
}
