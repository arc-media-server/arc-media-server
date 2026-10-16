package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

const itemCols = `id, library_id, kind, COALESCE(parent_id, ''), title, sort_title, original_title, year,
	overview, runtime_min, release_date, tmdb_id, tvdb_id, imdb_id, poster_path, backdrop_path,
	metadata_state, added_at, locked_fields`

func scanItem(row interface{ Scan(...any) error }, it *Item) error {
	var locked string
	err := row.Scan(&it.ID, &it.LibraryID, &it.Kind, &it.ParentID, &it.Title, &it.SortTitle,
		&it.OriginalTitle, &it.Year, &it.Overview, &it.RuntimeMin, &it.ReleaseDate, &it.TMDBID,
		&it.TVDBID, &it.IMDBID, &it.PosterPath, &it.BackdropPath, &it.MetadataState, &it.AddedAt, &locked)
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
	parts := strings.Split(cols, ",")
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
			`+keepIfLocked("poster")+`, `+keepIfLocked("backdrop")+`,
			metadata_state = ?, updated_at = ?
		WHERE id = ? AND metadata_state <> 'locked'`,
		it.Title, SortTitle(it.Title), it.OriginalTitle, it.Year, it.Overview, it.RuntimeMin,
		it.ReleaseDate, it.TMDBID, it.TVDBID, it.IMDBID, it.PosterPath, it.BackdropPath,
		it.MetadataState, time.Now().Unix(), it.ID)
	return err
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
	return tx.Commit()
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

// ListItems returns a page of items plus the total match count.
func (s *Store) ListItems(ctx context.Context, q ItemQuery) ([]Item, int, error) {
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
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := "sort_title, year"
	switch q.Sort {
	case "added":
		order = "added_at DESC, sort_title"
	case "year":
		order = "year DESC, sort_title"
	}
	limit := q.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+itemCols+` FROM items WHERE `+cond+
		` ORDER BY `+order+` LIMIT ? OFFSET ?`, append(args, limit, q.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, 0, err
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}
