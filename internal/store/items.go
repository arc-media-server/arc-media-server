package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"sort"
	"strings"
	"time"
)

const itemCols = `id, library_id, kind, COALESCE(parent_id, ''), title, sort_title, original_title, year,
	overview, runtime_min, release_date, tmdb_id, tvdb_id, imdb_id, poster_path, backdrop_path,
	metadata_state, added_at, locked_fields, tagline, content_rating, studio, rating, local_poster, local_backdrop,
	logo_path, local_logo, source_ordering, original_language, show_status, last_aired, next_aired,
	imdb_rating, imdb_votes, rt_critics, metacritic, local_theme, details_provider`

func scanItem(row interface{ Scan(...any) error }, it *Item) error {
	return scanItemWith(row, it)
}

// scanItemWith scans an item's columns followed by extra ones.
func scanItemWith(row interface{ Scan(...any) error }, it *Item, extra ...any) error {
	var locked string
	dest := []any{&it.ID, &it.LibraryID, &it.Kind, &it.ParentID, &it.Title, &it.SortTitle,
		&it.OriginalTitle, &it.Year, &it.Overview, &it.RuntimeMin, &it.ReleaseDate, &it.TMDBID,
		&it.TVDBID, &it.IMDBID, &it.PosterPath, &it.BackdropPath, &it.MetadataState, &it.AddedAt, &locked,
		&it.Tagline, &it.ContentRating, &it.Studio, &it.Rating, &it.LocalPoster, &it.LocalBackdrop,
		&it.LogoPath, &it.LocalLogo, &it.SourceOrdering, &it.OriginalLanguage, &it.ShowStatus, &it.LastAired, &it.NextAired,
		&it.IMDbRating, &it.IMDbVotes, &it.RottenTomatoes, &it.Metacritic, &it.LocalTheme, &it.DetailsProvider}
	err := row.Scan(append(dest, extra...)...)
	it.LockedFields = splitLocked(locked)
	return err
}

// LockableFields maps the metadata fields a user can lock to their columns.
var LockableFields = map[string]string{
	"title":          "title",
	"sort_title":     "sort_title",
	"original_title": "original_title",
	"year":           "year",
	"overview":       "overview",
	"release_date":   "release_date",
	"runtime_min":    "runtime_min",
	"poster":         "poster_path",
	"backdrop":       "backdrop_path",
	"logo":           "logo_path",
	"genres":         "item_genres", // its own table, not a column of items
	"tagline":        "tagline",
	"content_rating": "content_rating",
	"studio":         "studio",
}

func splitLocked(s string) []string {
	out := []string{}
	for _, f := range strings.Split(s, ",") {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// lockedSQL is an SQL condition that is true when field is in the row's locked_fields.
func lockedSQL(field string) string {
	return `instr(',' || locked_fields || ',', ',` + field + `,') > 0`
}

// keepIfLocked is an SQL assignment that sets a column from a parameter unless its field is locked.
func keepIfLocked(field string) string {
	col := LockableFields[field]
	return col + ` = CASE WHEN ` + lockedSQL(field) + ` THEN ` + col + ` ELSE ? END`
}

// prefixCols qualifies each column in a comma-separated list with a table alias.
func prefixCols(prefix, cols string) string {
	// Split on the commas between columns, not those inside COALESCE(...).
	var parts []string
	depth, start := 0, 0
	for i, c := range cols {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, cols[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, cols[start:])
	for i, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "COALESCE(") {
			p = "COALESCE(" + prefix + strings.TrimPrefix(p, "COALESCE(")
		} else {
			p = prefix + p
		}
		parts[i] = p
	}
	return strings.Join(parts, ", ")
}

// EnsureItem returns the id of the item with matchKey in the library, creating it from it if absent.
func (s *Store) EnsureItem(ctx context.Context, it *Item, matchKey string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM items WHERE library_id = ? AND match_key = ?`, it.LibraryID, matchKey).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	// A show merged into another keeps its folder's key as an alias of the kept one.
	err = s.db.QueryRowContext(ctx,
		`SELECT item_id FROM item_aliases WHERE library_id = ? AND match_key = ?`, it.LibraryID, matchKey).Scan(&id)
	if err == nil {
		return id, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	id = NewID()
	now := time.Now().Unix()
	var parent any
	if it.ParentID != "" {
		parent = it.ParentID
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO items (id, library_id, kind, parent_id, match_key, title, sort_title, year,
			tmdb_id, tvdb_id, imdb_id, added_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, it.LibraryID, it.Kind, parent, matchKey, it.Title, SortTitle(it.Title), it.Year,
		it.TMDBID, it.TVDBID, it.IMDBID, now, now)
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// GetItem returns nil when no item has the given id.
func (s *Store) GetItem(ctx context.Context, id string) (*Item, error) {
	var it Item
	err := scanItem(s.db.QueryRowContext(ctx, `SELECT `+itemCols+` FROM items WHERE id = ?`, id), &it)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// StaleItems lists matched items of a kind in a library whose metadata was last fetched before
// a time (unix seconds), oldest first.
func (s *Store) StaleItems(ctx context.Context, libraryID int64, kind string, before int64, limit int) ([]Item, error) {
	return s.queryItems(ctx, `SELECT `+itemCols+` FROM items WHERE library_id = ? AND kind = ? AND metadata_state = 'matched'
		AND synced_at > 0 AND synced_at < ? ORDER BY synced_at LIMIT ?`, libraryID, kind, before, limit)
}

// ItemsWithMetadataState lists items of a kind in a library with the given metadata state.
func (s *Store) ItemsWithMetadataState(ctx context.Context, libraryID int64, kind, state string) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+itemCols+` FROM items
		WHERE library_id = ? AND kind = ? AND metadata_state = ? ORDER BY sort_title`, libraryID, kind, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// RetryUnmatched revisits old uncertain/no-result titles during reconciliation, not every
// watcher event. Locked metadata and user edits are preserved by the normal match writer.
func (s *Store) RetryUnmatched(ctx context.Context, libraryID int64, before int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET metadata_state = 'pending' WHERE library_id = ? AND kind IN ('movie', 'series') AND metadata_state = 'unmatched' AND updated_at < ?`, libraryID, before)
	return err
}

// UnsyncedItems lists matched items of a kind whose metadata has never been fully fetched.
func (s *Store) UnsyncedItems(ctx context.Context, libraryID int64, kind string, limit int) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+itemCols+` FROM items
		WHERE library_id = ? AND kind = ? AND metadata_state = 'matched' AND synced_at = 0
		ORDER BY added_at DESC LIMIT ?`, libraryID, kind, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Item
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// MetadataRefreshPage lists known matched movies due for refresh. The ID cursor
// lets a pass drain its backlog without repeatedly retrying a failed first page.
func (s *Store) MetadataRefreshPage(ctx context.Context, libraryID int64, before int64, after string, limit int) ([]Item, error) {
	return s.queryItems(ctx, `SELECT `+itemCols+` FROM items WHERE library_id = ? AND kind = 'movie' AND metadata_state = 'matched' AND (synced_at = 0 OR synced_at < ?) AND id > ? ORDER BY id LIMIT ?`, libraryID, before, after, limit)
}

// UpdateMetadata writes provider metadata onto an item. Fields the user locked keep their values,
// and items whose whole metadata is locked are left untouched.
func (s *Store) UpdateMetadata(ctx context.Context, it *Item) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE items SET `+keepIfLocked("title")+`,
			-- The sort title follows the title, unless either was set by the user.
			sort_title = CASE WHEN `+lockedSQL("sort_title")+` OR `+lockedSQL("title")+` THEN sort_title ELSE ? END,
			`+keepIfLocked("original_title")+`, `+keepIfLocked("year")+`, `+keepIfLocked("overview")+`,
			`+keepIfLocked("runtime_min")+`, `+keepIfLocked("release_date")+`,
			tmdb_id = ?, tvdb_id = ?, imdb_id = ?,
			`+keepIfLocked("poster")+`, `+keepIfLocked("backdrop")+`, `+keepIfLocked("logo")+`,
			`+keepIfLocked("tagline")+`, `+keepIfLocked("content_rating")+`, `+keepIfLocked("studio")+`, rating = ?,
			original_language = ?, show_status = ?, last_aired = ?, next_aired = ?,
			metadata_state = ?, updated_at = ?
		WHERE id = ? AND metadata_state <> 'locked'`,
		it.Title, SortTitle(it.Title), it.OriginalTitle, it.Year, it.Overview, it.RuntimeMin,
		it.ReleaseDate, it.TMDBID, it.TVDBID, it.IMDBID, it.PosterPath, it.BackdropPath, it.LogoPath,
		it.Tagline, it.ContentRating, it.Studio, it.Rating,
		it.OriginalLanguage, it.ShowStatus, it.LastAired, it.NextAired,
		it.MetadataState, time.Now().Unix(), it.ID)
	if err != nil {
		return err
	}
	return s.syncMinAges(ctx, it.ID)
}

// SetLocalTheme records the theme tune found next to a show or movie ("" for none).
func (s *Store) SetLocalTheme(ctx context.Context, id, path string) error {
	var same bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM items WHERE id = ? AND local_theme = ?)`, id, path).Scan(&same); err != nil {
		return err
	}
	if same {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE items SET local_theme = ? WHERE id = ? AND local_theme <> ?`, path, id, path)
	return err
}

// SetLocalArt records the artwork files found next to an item's media ("" for none).
func (s *Store) SetLocalArt(ctx context.Context, id, poster, backdrop, logo string) error {
	var same bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM items WHERE id = ? AND local_poster = ? AND local_backdrop = ? AND local_logo = ?)`, id, poster, backdrop, logo).Scan(&same); err != nil {
		return err
	}
	if same {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE items SET local_poster = ?, local_backdrop = ?, local_logo = ?
		WHERE id = ? AND (local_poster <> ? OR local_backdrop <> ? OR local_logo <> ?)`,
		poster, backdrop, logo, id, poster, backdrop, logo)
	return err
}

// AdoptIDs takes provider ids from a local .nfo file for an item that isn't identified yet (or
// couldn't be), so the next lookup finds exactly that title. Identified items keep their match.
func (s *Store) AdoptIDs(ctx context.Context, id string, tmdbID, tvdbID int64, imdbID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE items SET
			tmdb_id = CASE WHEN ? <> 0 THEN ? ELSE tmdb_id END,
			tvdb_id = CASE WHEN ? <> 0 THEN ? ELSE tvdb_id END,
			imdb_id = CASE WHEN ? <> '' THEN ? ELSE imdb_id END,
			metadata_state = 'pending', synced_at = 0
		WHERE id = ? AND metadata_state IN ('pending', 'unmatched')
			AND (tmdb_id <> ? OR tvdb_id <> ? OR imdb_id <> ?)
			AND NOT (? = 0 AND ? = 0 AND ? = '')`,
		tmdbID, tmdbID, tvdbID, tvdbID, imdbID, imdbID, id, tmdbID, tvdbID, imdbID, tmdbID, tvdbID, imdbID)
	return err
}

// SetItemDetails replaces an item's genres and credits (people are shared between items).
func (s *Store) SetItemDetails(ctx context.Context, itemID string, genres []string, credits []Credit) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Genres the user edited are kept.
	var locked string
	if err := tx.QueryRowContext(ctx, `SELECT locked_fields FROM items WHERE id = ?`, itemID).Scan(&locked); err != nil {
		return err
	}
	if !slices.Contains(splitLocked(locked), "genres") {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_genres WHERE item_id = ?`, itemID); err != nil {
			return err
		}
		for _, g := range genres {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_genres (item_id, name) VALUES (?, ?)`, itemID, g); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_people WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	for _, c := range credits {
		if c.TMDBID <= 0 || strings.TrimSpace(c.Name) == "" {
			continue
		}
		var pid int64
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO people (tmdb_id, name, profile_path) VALUES (?, ?, ?)
			ON CONFLICT (tmdb_id) DO UPDATE SET name = excluded.name,
				profile_path = CASE WHEN excluded.profile_path <> '' THEN excluded.profile_path ELSE people.profile_path END
			RETURNING id`, c.TMDBID, c.Name, c.ProfilePath).Scan(&pid); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO item_people (item_id, person_id, role, character, ord, episode_count) VALUES (?, ?, ?, ?, ?, ?)`,
			itemID, pid, c.Role, c.Character, c.Order, max(0, c.EpisodeCount)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ItemGenres returns an item's genres.
func (s *Store) ItemGenres(ctx context.Context, itemID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM item_genres WHERE item_id = ? ORDER BY rowid`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ItemCredits returns an item's cast and crew in billing order.
func (s *Store) ItemCredits(ctx context.Context, itemID string) ([]Credit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.tmdb_id, p.name, p.profile_path, ip.role, ip.character, ip.ord, ip.episode_count
		FROM item_people ip JOIN people p ON p.id = ip.person_id
		WHERE ip.item_id = ? ORDER BY ip.ord`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Credit{}
	for rows.Next() {
		var c Credit
		if err := rows.Scan(&c.PersonID, &c.TMDBID, &c.Name, &c.ProfilePath, &c.Role, &c.Character, &c.Order, &c.EpisodeCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PersonImage returns a person's TMDB image path, or "".
func (s *Store) PersonImage(ctx context.Context, personID int64) (string, error) {
	var p string
	err := s.db.QueryRowContext(ctx, `SELECT profile_path FROM people WHERE id = ?`, personID).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return p, err
}

// ItemEdit is a user's change to an item's metadata. Nil fields are left as they are; every
// field that is set is also locked, so refreshes don't undo the edit.
type ItemEdit struct {
	Title         *string
	SortTitle     *string
	OriginalTitle *string
	Year          *int
	Overview      *string
	ReleaseDate   *string
	RuntimeMin    *int
	Tagline       *string
	// Chosen artwork: a provider image path or an uploaded file ("upload:...").
	Poster   *string
	Backdrop *string
	Logo     *string
	Genres   *[]string
	// Labels are the user's own, so they're never locked or replaced by refreshes.
	Labels        *[]string
	ContentRating *string
	Studio        *string
	// Locked, when set, replaces the item's locked fields (before the edited fields are added).
	// Unlocking a field lets the next refresh replace its value.
	Locked []string
}

// EditItem applies a user's edit. It returns an error naming any unknown locked field.
func (s *Store) EditItem(ctx context.Context, id string, e ItemEdit) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var title, current string
	if err := tx.QueryRowContext(ctx, `SELECT title, locked_fields FROM items WHERE id = ?`, id).Scan(&title, &current); err != nil {
		return err
	}
	locked := map[string]bool{}
	if e.Locked != nil {
		for _, f := range e.Locked {
			if LockableFields[f] == "" {
				return errors.New("unknown field: " + f)
			}
			locked[f] = true
		}
	} else {
		for _, f := range splitLocked(current) {
			locked[f] = true
		}
	}

	sets := []string{}
	args := []any{}
	set := func(field string, v any) {
		sets = append(sets, LockableFields[field]+" = ?")
		args = append(args, v)
		locked[field] = true
	}
	if e.Title != nil {
		set("title", *e.Title)
		title = *e.Title
	}
	switch {
	case e.SortTitle != nil && strings.TrimSpace(*e.SortTitle) != "":
		set("sort_title", strings.ToLower(strings.TrimSpace(*e.SortTitle)))
	case e.SortTitle != nil || (e.Title != nil && !locked["sort_title"]):
		// A cleared sort title goes back to following the title.
		delete(locked, "sort_title")
		sets = append(sets, "sort_title = ?")
		args = append(args, SortTitle(title))
	}
	if e.OriginalTitle != nil {
		set("original_title", *e.OriginalTitle)
	}
	if e.Year != nil {
		set("year", *e.Year)
	}
	if e.Overview != nil {
		set("overview", *e.Overview)
	}
	if e.ReleaseDate != nil {
		set("release_date", *e.ReleaseDate)
	}
	if e.RuntimeMin != nil {
		set("runtime_min", *e.RuntimeMin)
	}
	if e.Tagline != nil {
		set("tagline", *e.Tagline)
	}
	if e.ContentRating != nil {
		set("content_rating", *e.ContentRating)
	}
	if e.Studio != nil {
		set("studio", *e.Studio)
	}
	if e.Poster != nil {
		set("poster", *e.Poster)
	}
	if e.Backdrop != nil {
		set("backdrop", *e.Backdrop)
	}
	if e.Logo != nil {
		set("logo", *e.Logo)
	}
	if e.Genres != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_genres WHERE item_id = ?`, id); err != nil {
			return err
		}
		for _, g := range *e.Genres {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_genres (item_id, name) VALUES (?, ?)`, id, g); err != nil {
				return err
			}
		}
		locked["genres"] = true
	}
	if e.Labels != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_labels WHERE item_id = ?`, id); err != nil {
			return err
		}
		for _, l := range *e.Labels {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_labels (item_id, label) VALUES (?, ?)`, id, l); err != nil {
				return err
			}
		}
	}

	fields := make([]string, 0, len(locked))
	for f := range LockableFields {
		if locked[f] {
			fields = append(fields, f)
		}
	}
	sort.Strings(fields)
	sets = append(sets, "locked_fields = ?", "updated_at = ?")
	args = append(args, strings.Join(fields, ","), time.Now().Unix(), id)
	if _, err := tx.ExecContext(ctx, `UPDATE items SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if e.ContentRating != nil {
		return s.syncMinAges(ctx, id)
	}
	return nil
}

// SetMatchInfo records how an item was identified (JSON written by the scanner).
func (s *Store) SetMatchInfo(ctx context.Context, id, info string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE items SET match_info = ? WHERE id = ?`, info, id)
	return err
}

// MatchInfo returns what SetMatchInfo stored, or "" if the item was never looked up.
func (s *Store) MatchInfo(ctx context.Context, id string) (string, error) {
	var info string
	err := s.db.QueryRowContext(ctx, `SELECT match_info FROM items WHERE id = ?`, id).Scan(&info)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return info, err
}

// SetProviderIDs replaces an item's provider ids, e.g. when the user picks the right match. The
// item goes back to pending until its metadata is fetched with the new ids.
func (s *Store) SetProviderIDs(ctx context.Context, id string, tmdbID, tvdbID int64, imdbID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE items SET tmdb_id = ?, tvdb_id = ?, imdb_id = ?, synced_at = 0,
			metadata_state = CASE metadata_state WHEN 'locked' THEN 'locked' ELSE 'pending' END, updated_at = ?
		WHERE id = ?`, tmdbID, tvdbID, imdbID, time.Now().Unix(), id)
	return err
}

func (s *Store) SetMetadataState(ctx context.Context, id, state string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE items SET metadata_state = ?, updated_at = ? WHERE id = ? AND metadata_state <> 'locked'`,
		state, time.Now().Unix(), id)
	return err
}

// ItemImage is one piece of artwork an item uses.
type ItemImage struct {
	Kind string // item kind
	Type string // primary or backdrop
	Path string
}

// ItemImages lists the artwork of every item, including ones whose files are missing (so their
// artwork survives a drive going offline). Posters come first, then backdrops, then episode stills.
func (s *Store) ItemImages(ctx context.Context) ([]ItemImage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT kind, 'primary', poster_path, CASE kind WHEN 'episode' THEN 2 ELSE 0 END FROM items WHERE poster_path <> ''
		UNION ALL
		SELECT kind, 'backdrop', backdrop_path, 1 FROM items WHERE backdrop_path <> ''
		UNION ALL
		SELECT kind, 'logo', logo_path, 1 FROM items WHERE logo_path <> ''
		UNION ALL
		SELECT 'season', 'primary', poster_path, 2 FROM season_art WHERE poster_path <> ''
		UNION ALL
		SELECT 'season', 'primary', chosen_path, 2 FROM season_art WHERE chosen_path <> '' AND chosen_path NOT LIKE 'upload:%'
		UNION ALL
		SELECT 'person', 'primary', profile_path, 3 FROM people
			WHERE profile_path <> '' AND id IN (SELECT person_id FROM item_people)
		UNION ALL
		SELECT 'collection', 'primary', poster_path, 4 FROM collections WHERE poster_path <> ''
		UNION ALL
		SELECT 'collection', 'backdrop', backdrop_path, 4 FROM collections WHERE backdrop_path <> ''
		ORDER BY 4`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ItemImage
	for rows.Next() {
		var im ItemImage
		var rank int
		if err := rows.Scan(&im.Kind, &im.Type, &im.Path, &rank); err != nil {
			return nil, err
		}
		out = append(out, im)
	}
	return out, rows.Err()
}

// Video heights (or width-derived heights, for widescreen) each resolution filter covers.
var resolutionHeights = map[string][2]int{"4k": {1600, 100000}, "1080": {1000, 1600}, "720": {700, 1000}, "sd": {1, 700}}

// LibraryFacets are the values a library's titles can be filtered by.
type LibraryFacets struct {
	// How many titles the Unmatched filter finds.
	Unmatched   int     `json:"unmatched"`
	Genres      []Facet `json:"genres"`
	Decades     []Facet `json:"decades"`
	Resolutions []Facet `json:"resolutions"`
	// Original languages (2-letter codes), most titles first.
	Languages []Facet `json:"languages"`
	Labels    []Facet `json:"labels"`
}

// Facet is one filter value and how many titles have it.
type Facet struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Facets lists a library's genres, decades and (for movies) resolutions, with counts, for the
// titles a profile with maxAge may see.
func (s *Store) Facets(ctx context.Context, libraryID int64, kind string, acc Access) (LibraryFacets, error) {
	base := `items.library_id = ? AND items.kind = ?`
	args := []any{libraryID, kind}
	base, args = acc.Where("items", base, args)
	query := func(q string) ([]Facet, error) {
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []Facet{}
		for rows.Next() {
			var f Facet
			if err := rows.Scan(&f.Value, &f.Count); err != nil {
				return nil, err
			}
			out = append(out, f)
		}
		return out, rows.Err()
	}
	var out LibraryFacets
	var err error
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE `+base+` AND `+playableSQL+` AND `+unmatchedSQL, args...).Scan(&out.Unmatched); err != nil {
		return out, err
	}
	if out.Genres, err = query(`SELECT g.name, COUNT(*) FROM item_genres g JOIN items ON items.id = g.item_id
		WHERE ` + base + ` GROUP BY g.name ORDER BY g.name`); err != nil {
		return out, err
	}
	if out.Labels, err = query(`SELECT l.label, COUNT(*) FROM item_labels l JOIN items ON items.id = l.item_id
		WHERE ` + base + ` GROUP BY l.label ORDER BY l.label COLLATE NOCASE`); err != nil {
		return out, err
	}
	if out.Languages, err = query(`SELECT original_language, COUNT(*) FROM items
		WHERE ` + base + ` AND original_language <> '' GROUP BY original_language ORDER BY COUNT(*) DESC, original_language`); err != nil {
		return out, err
	}
	if out.Decades, err = query(`SELECT CAST(year / 10 * 10 AS TEXT), COUNT(*) FROM items
		WHERE ` + base + ` AND year > 0 GROUP BY year / 10 ORDER BY year / 10 DESC`); err != nil {
		return out, err
	}
	out.Resolutions = []Facet{}
	if kind == "movie" {
		if out.Resolutions, err = query(`SELECT CASE
				WHEN h >= 1600 THEN '4k' WHEN h >= 1000 THEN '1080' WHEN h >= 700 THEN '720' ELSE 'sd' END AS r, COUNT(DISTINCT id)
			FROM (SELECT items.id, MAX(st.height, st.width * 9 / 16) AS h FROM items
				JOIN item_files f ON f.item_id = items.id JOIN media_files m ON m.id = f.file_id
				JOIN media_streams st ON st.file_id = m.id
				WHERE ` + base + ` AND m.missing_since IS NULL AND st.kind = 'video' AND st.height > 0)
			GROUP BY r ORDER BY MIN(h) DESC`); err != nil {
			return out, err
		}
	}
	return out, nil
}

// ListItems returns a page of items plus the total match count.
func (s *Store) ListItems(ctx context.Context, q ItemQuery) ([]Item, int, error) {
	items, total, _, err := s.ListItemsPage(ctx, q)
	return items, total, err
}

// ListItemsPage also returns the actual start offset, including a requested letter jump.
// Jumping changes the starting position, never the filters or the total match count.
func (s *Store) ListItemsPage(ctx context.Context, q ItemQuery) ([]Item, int, int, error) {
	if q.Jump != "" && (len(q.Jump) != 1 || q.Jump[0] < 'a' || q.Jump[0] > 'z' || (q.Sort != "" && q.Sort != "title")) {
		return nil, 0, 0, errors.New("jump requires a letter a–z and title sort")
	}
	cond, args := itemFilter(q)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, 0, err
	}
	offset := max(0, q.Offset)
	if q.Jump != "" {
		boundary, comparison := q.Jump, "<"
		if q.Reverse {
			// Descending starts with the last title in the chosen letter (or next
			// available earlier letter), not with the first title at its lower bound.
			boundary, comparison = string([]byte{q.Jump[0] + 1}), ">="
		}
		jumpArgs := append(append([]any{}, args...), boundary)
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE `+cond+
			` AND sort_title `+comparison+` ?`, jumpArgs...).Scan(&offset); err != nil {
			return nil, 0, 0, err
		}
	}

	order := "sort_title ASC, year ASC"
	var orderArgs []any
	switch q.Sort {
	case "added":
		order = "added_at DESC, sort_title"
	case "year":
		order = "year DESC, sort_title"
	case "release":
		order = "release_date DESC, sort_title"
	case "rating":
		order = "rating DESC, sort_title"
	case "imdb":
		order = "imdb_rating DESC, imdb_votes DESC, sort_title"
	case "critics":
		order = "rt_critics IS NULL, rt_critics DESC, metacritic DESC, sort_title"
	case "runtime":
		order = "runtime_min DESC, sort_title"
	case "watched":
		// Most recently watched first (a show by its episodes); never watched last.
		order = `(SELECT MAX(u.last_played_at) FROM user_item_data u JOIN items w ON w.id = u.item_id
			WHERE u.user_id = ? AND (w.id = items.id OR w.parent_id = items.id)) DESC NULLS LAST, sort_title`
		orderArgs = append(orderArgs, q.UserID)
	case "shuffle":
		// A different order each day, but the same one all day so rows don't jump around.
		order = "(items.rowid * 2654435761 + ?) % 4294967291"
		orderArgs = append(orderArgs, time.Now().Unix()/86400*7919)
	}
	if q.Reverse && q.Sort != "shuffle" {
		order = strings.ReplaceAll(order, "sort_title", "sort_title ASC")
		order = strings.ReplaceAll(order, " ASC ASC", " ASC")
		order = strings.ReplaceAll(order, " DESC", " __DESC__")
		order = strings.ReplaceAll(order, " ASC", " DESC")
		order = strings.ReplaceAll(order, " __DESC__", " ASC")
	}
	order += ", id"
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	all := append(append(append([]any{}, args...), orderArgs...), limit, offset)
	rows, err := s.db.QueryContext(ctx, `SELECT `+itemCols+` FROM items WHERE `+cond+
		` ORDER BY `+order+` LIMIT ? OFFSET ?`, all...)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, 0, 0, err
		}
		items = append(items, it)
	}
	return items, total, offset, rows.Err()
}

// itemFilter is the WHERE clause (and its arguments) for a listing's filters and access.
func itemFilter(q ItemQuery) (string, []any) {
	// Only list things that can be played: items with a present file, or series with at least one
	// such episode.
	where := []string{`(EXISTS (
		SELECT 1 FROM item_files f JOIN media_files m ON m.id = f.file_id
		WHERE f.item_id = items.id AND m.missing_since IS NULL)
	OR (items.kind = 'series' AND EXISTS (
		SELECT 1 FROM items c JOIN item_files f ON f.item_id = c.id JOIN media_files m ON m.id = f.file_id
		WHERE c.parent_id = items.id AND m.missing_since IS NULL)))`}
	var args []any
	if q.LibraryID != 0 {
		where = append(where, "library_id = ?")
		args = append(args, q.LibraryID)
	}
	if q.Kind != "" {
		where = append(where, "kind = ?")
		args = append(args, q.Kind)
	}
	if q.Search != "" {
		where = append(where, "(title LIKE ? OR original_title LIKE ?)")
		like := "%" + q.Search + "%"
		args = append(args, like, like)
	}
	conds, more := q.Access.Conds("items")
	where = append(where, conds...)
	args = append(args, more...)
	if q.Genre != "" {
		where = append(where, "EXISTS (SELECT 1 FROM item_genres g WHERE g.item_id = items.id AND g.name = ?)")
		args = append(args, q.Genre)
	}
	if q.Label != "" {
		where = append(where, "EXISTS (SELECT 1 FROM item_labels l WHERE l.item_id = items.id AND l.label = ?)")
		args = append(args, q.Label)
	}
	if q.Language != "" {
		where = append(where, "original_language = ?")
		args = append(args, q.Language)
	}
	if q.Decade > 0 {
		where = append(where, "year BETWEEN ? AND ?")
		args = append(args, q.Decade, q.Decade+9)
	}
	if h, ok := resolutionHeights[q.Resolution]; ok {
		where = append(where, `EXISTS (SELECT 1 FROM item_files f JOIN media_files m ON m.id = f.file_id
			JOIN media_streams st ON st.file_id = m.id
			WHERE f.item_id = items.id AND m.missing_since IS NULL AND st.kind = 'video'
				AND MAX(st.height, st.width * 9 / 16) >= ? AND MAX(st.height, st.width * 9 / 16) < ?)`)
		args = append(args, h[0], h[1])
	}
	if q.Unmatched {
		where = append(where, unmatchedSQL)
	}
	if q.Unwatched && q.UserID != "" {
		// Movies not watched; shows with an episode on disk not watched.
		where = append(where, `CASE WHEN items.kind = 'series' THEN EXISTS (
				SELECT 1 FROM items e JOIN item_files f ON f.item_id = e.id JOIN media_files m ON m.id = f.file_id
				WHERE e.parent_id = items.id AND m.missing_since IS NULL AND NOT EXISTS (
					SELECT 1 FROM user_item_data u WHERE u.user_id = ? AND u.item_id = e.id AND u.played = 1))
			ELSE NOT EXISTS (SELECT 1 FROM user_item_data u WHERE u.user_id = ? AND u.item_id = items.id AND u.played = 1) END`)
		args = append(args, q.UserID, q.UserID)
	}
	if q.UserID != "" && (q.Unstarted || q.Played || q.PlayedBefore > 0) {
		if q.Unstarted {
			c, a := watchedSQL(q.UserID, "(u.played = 1 OR u.position_ms > 0)")
			where = append(where, "NOT "+c)
			args = append(args, a...)
		}
		if q.Played || q.PlayedBefore > 0 {
			c, a := watchedSQL(q.UserID, "u.played = 1")
			where = append(where, c)
			args = append(args, a...)
		}
		if q.PlayedBefore > 0 {
			c, a := watchedSQL(q.UserID, "u.last_played_at >= ?", q.PlayedBefore)
			where = append(where, "NOT "+c)
			args = append(args, a...)
		}
	}
	if q.PersonID != 0 {
		c := "EXISTS (SELECT 1 FROM item_people p WHERE p.item_id = items.id AND p.person_id = ?"
		args = append(args, q.PersonID)
		if q.PersonRole != "" {
			c += " AND p.role = ?"
			args = append(args, q.PersonRole)
		}
		where = append(where, c+")")
	}
	if q.AddedWithinDays > 0 {
		where = append(where, "added_at >= ?")
		args = append(args, time.Now().Add(-time.Duration(q.AddedWithinDays)*24*time.Hour).Unix())
	}
	if q.MinRuntime > 0 {
		where = append(where, "runtime_min >= ?")
		args = append(args, q.MinRuntime)
	}
	if q.MaxRuntime > 0 {
		where = append(where, "runtime_min > 0 AND runtime_min <= ?")
		args = append(args, q.MaxRuntime)
	}
	if q.MinRating > 0 {
		where = append(where, "rating >= ?")
		args = append(args, q.MinRating)
	}
	var about []string
	for _, w := range q.About {
		if w = strings.TrimSpace(w); w != "" {
			about = append(about, `title LIKE ? OR tagline LIKE ? OR overview LIKE ?
				OR EXISTS (SELECT 1 FROM item_keywords k WHERE k.item_id = items.id AND k.keyword LIKE ?)`)
			like := "%" + w + "%"
			args = append(args, like, like, like, like)
		}
	}
	if len(about) > 0 {
		where = append(where, "("+strings.Join(about, " OR ")+")")
	}
	return strings.Join(where, " AND "), args
}

// watchedSQL is a condition that a profile has a watch-state row matching cond (on alias u) for a
// title: for a show, any of its episodes; otherwise the item itself.
func watchedSQL(userID, cond string, condArgs ...any) (string, []any) {
	sql := `(CASE WHEN items.kind = 'series' THEN EXISTS (
			SELECT 1 FROM items e JOIN user_item_data u ON u.item_id = e.id AND u.user_id = ?
			WHERE e.parent_id = items.id AND ` + cond + `)
		ELSE EXISTS (SELECT 1 FROM user_item_data u WHERE u.user_id = ? AND u.item_id = items.id AND ` + cond + `) END)`
	args := append([]any{userID}, condArgs...)
	return sql, append(append(args, userID), condArgs...)
}

// LetterCount is how many titles in a listing start with one character of their sort title:
// "a" to "z", or "#" for anything else (digits, punctuation, other scripts).
type LetterCount struct {
	Letter string `json:"letter"`
	Count  int    `json:"count"`
}

// TitleLetters counts a listing's titles by the first character of their sort title, in title
// order, so a client can tell where each letter starts. "#" can appear twice: before "a" for
// digits and punctuation, after "z" for other scripts.
func (s *Store) TitleLetters(ctx context.Context, q ItemQuery) ([]LetterCount, error) {
	cond, args := itemFilter(q)
	rows, err := s.db.QueryContext(ctx, `SELECT substr(sort_title, 1, 1) AS c, COUNT(*) FROM items WHERE `+cond+
		` GROUP BY c ORDER BY c`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LetterCount{}
	for rows.Next() {
		var c string
		var n int
		if err := rows.Scan(&c, &n); err != nil {
			return nil, err
		}
		letter := strings.ToLower(c)
		if len(letter) != 1 || letter[0] < 'a' || letter[0] > 'z' {
			letter = "#"
		}
		if last := len(out) - 1; last >= 0 && out[last].Letter == letter {
			out[last].Count += n
			continue
		}
		out = append(out, LetterCount{letter, n})
	}
	return out, rows.Err()
}

// unmatchedSQL is the Unmatched filter: a movie or show Arc couldn't identify, or a show with an
// episode on disk it couldn't place (Library Health lists those too). Plex's filter only finds
// the first.
const unmatchedSQL = `(items.metadata_state = 'unmatched' OR (items.kind = 'series' AND EXISTS (
	SELECT 1 FROM items e WHERE e.parent_id = items.id AND e.kind = 'episode' AND e.metadata_state = 'unmatched'
		AND EXISTS (SELECT 1 FROM item_files f WHERE f.item_id = e.id))))`

// HasUnmatchedEpisodes checks only playable unmatched episodes, not orphan/provider-only rows.
func (s *Store) HasUnmatchedEpisodes(ctx context.Context, seriesID string) (bool, error) {
	var found bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM items i WHERE i.parent_id = ? AND i.kind = 'episode' AND i.metadata_state = 'unmatched' AND EXISTS(SELECT 1 FROM item_files f WHERE f.item_id = i.id))`, seriesID).Scan(&found)
	return found, err
}
