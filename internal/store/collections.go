package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
)

// Collection groups titles in a library. Automatic ones come from TMDB (every movie TMDB puts in
// the collection); manual ones hold the titles the admin chose, in order; smart ones hold whatever
// their filter matches.
type Collection struct {
	ID           string `json:"id"`
	LibraryID    int64  `json:"library_id"`
	Kind         string `json:"kind"` // auto, manual, smart, folder (an Other videos library's folder)
	TMDBID       int64  `json:"tmdb_id,omitempty"`
	Title        string `json:"title"`
	SortTitle    string `json:"sort_title"`
	Overview     string `json:"overview,omitempty"`
	PosterPath   string `json:"-"`
	BackdropPath string `json:"-"`
	// Smart collections: the filter, as JSON.
	Filter string `json:"filter,omitempty"`
	Hidden bool   `json:"-"`
	// Fields the admin edited, which TMDB refreshes leave alone.
	LockedFields []string `json:"locked_fields"`
	// How many titles the profile can see in it (filled in when listing).
	Count int `json:"count"`
	// How it shows in the library's grid: "" as the library says, or "items", "collection", "both".
	GridMode string `json:"grid_mode"`
}

const collectionCols = `id, library_id, kind, tmdb_id, title, sort_title, overview, poster_path, backdrop_path, filter, hidden, locked_fields, grid_mode`

func scanCollection(row interface{ Scan(...any) error }, c *Collection) error {
	var locked string
	if err := row.Scan(&c.ID, &c.LibraryID, &c.Kind, &c.TMDBID, &c.Title, &c.SortTitle, &c.Overview, &c.PosterPath,
		&c.BackdropPath, &c.Filter, &c.Hidden, &locked, &c.GridMode); err != nil {
		return err
	}
	c.LockedFields = splitLocked(locked)
	return nil
}

// collectionLocked is an SQL condition that a collection field is in the row's locked_fields.
func collectionLocked(field string) string {
	return `instr(',' || collections.locked_fields || ',', ',` + field + `,') > 0`
}

// playableSQL keeps titles with a present file (for a series, an episode with one).
const playableSQL = `(EXISTS (
		SELECT 1 FROM item_files f JOIN media_files m ON m.id = f.file_id
		WHERE f.item_id = items.id AND m.missing_since IS NULL)
	OR (items.kind = 'series' AND EXISTS (
		SELECT 1 FROM items c JOIN item_files f ON f.item_id = c.id JOIN media_files m ON m.id = f.file_id
		WHERE c.parent_id = items.id AND m.missing_since IS NULL)))`

func newID() string {
	var b [12]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// SetMovieCollection records which TMDB collection a movie is in (0 for none), and keeps the
// library's automatic collection's name and artwork up to date.
func (s *Store) SetMovieCollection(ctx context.Context, itemID string, libraryID int64, ref *CollectionRef) error {
	var tmdbID int64
	if ref != nil {
		tmdbID = ref.TMDBID
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE items SET tmdb_collection_id = ? WHERE id = ?`, tmdbID, itemID); err != nil {
		return err
	}
	if ref == nil || ref.TMDBID == 0 {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO collections (id, library_id, kind, tmdb_id, title, sort_title, poster_path, backdrop_path, created_at)
		VALUES (?, ?, 'auto', ?, ?, ?, ?, ?, ?)
		ON CONFLICT (library_id, tmdb_id) WHERE tmdb_id <> 0 DO UPDATE SET
			title = CASE WHEN `+collectionLocked("title")+` THEN collections.title ELSE excluded.title END,
			sort_title = CASE WHEN `+collectionLocked("sort_title")+` OR `+collectionLocked("title")+` THEN collections.sort_title ELSE excluded.sort_title END,
			poster_path = CASE WHEN `+collectionLocked("poster")+` THEN collections.poster_path ELSE excluded.poster_path END,
			backdrop_path = CASE WHEN `+collectionLocked("backdrop")+` THEN collections.backdrop_path ELSE excluded.backdrop_path END`,
		newID(), libraryID, ref.TMDBID, ref.Name, SortTitle(ref.Name), ref.PosterPath, ref.BackdropPath, time.Now().Unix())
	return err
}

// CollectionRef is the TMDB collection a movie belongs to.
type CollectionRef struct {
	TMDBID                         int64
	Name, PosterPath, BackdropPath string
}

// memberSQL selects a collection's titles (as items), for automatic and manual collections.
func memberSQL(c *Collection) (string, []any) {
	if c.Kind == "auto" {
		return `items.library_id = ? AND items.kind = 'movie' AND items.tmdb_collection_id = ?`, []any{c.LibraryID, c.TMDBID}
	}
	return `items.id IN (SELECT item_id FROM collection_items WHERE collection_id = ?)`, []any{c.ID}
}

// Collections lists a library's collections a profile can see, with how many of their titles it
// can see. Automatic ones need the library's minimum number of titles; smart ones are counted by
// the caller (their count is -1 here).
func (s *Store) Collections(ctx context.Context, libraryID int64, acc Access) ([]Collection, error) {
	all, err := s.readCollections(ctx, `WHERE library_id = ? AND hidden = 0`, libraryID)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return []Collection{}, nil
	}
	counts, err := s.libraryCollectionCounts(ctx, libraryID, acc)
	if err != nil {
		return nil, err
	}
	return s.visibleCollectionsWithCounts(ctx, libraryID, all, all, acc, counts)
}

func (s *Store) readCollections(ctx context.Context, where string, args ...any) ([]Collection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+collectionCols+` FROM collections `+where+` ORDER BY sort_title`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var all []Collection
	for rows.Next() {
		var c Collection
		if err := scanCollection(rows, &c); err != nil {
			return nil, err
		}
		all = append(all, c)
	}
	return all, rows.Err()
}

func (s *Store) visibleCollections(ctx context.Context, libraryID int64, candidates, peers []Collection, acc Access) ([]Collection, error) {
	return s.visibleCollectionsWithCounts(ctx, libraryID, candidates, peers, acc, nil)
}

func (s *Store) visibleCollectionsWithCounts(ctx context.Context, libraryID int64, candidates, peers []Collection, acc Access, counts map[string]int) ([]Collection, error) {
	out := []Collection{}
	if len(candidates) == 0 {
		return out, nil
	}
	var min int
	if err := s.db.QueryRowContext(ctx, `SELECT collection_min FROM libraries WHERE id = ?`, libraryID).Scan(&min); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	for _, c := range candidates {
		if c.Kind == "auto" {
			if min == 0 {
				continue
			}
			dup, err := s.autoDuplicated(ctx, &c, peers)
			if err != nil {
				return nil, err
			}
			if dup {
				continue
			}
		}
		switch c.Kind {
		case "smart":
			c.Count = -1
		default:
			n := counts[c.ID]
			if counts == nil {
				var err error
				n, err = s.countMembers(ctx, &c, acc)
				if err != nil {
					return nil, err
				}
			}
			if c.Kind == "auto" && n < min {
				continue
			}
			c.Count = n
		}
		out = append(out, c)
	}
	return out, nil
}

// autoDuplicated says whether an automatic (TMDB) collection is already covered by a manual one
// of the same name in its library that holds every one of its titles, as happens when a Plex
// import brings over Plex's copy of the same collection. Showing both would put the same row
// twice on a title's page and two identical tiles in the library, so only the manual one shows;
// an automatic collection with any title the manual one lacks stays, so nothing disappears.
func (s *Store) autoDuplicated(ctx context.Context, auto *Collection, all []Collection) (bool, error) {
	name := strings.TrimSpace(auto.Title)
	for _, m := range all {
		if m.Kind != "manual" || !strings.EqualFold(strings.TrimSpace(m.Title), name) {
			continue
		}
		var missing int
		err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items
			WHERE library_id = ? AND kind = 'movie' AND tmdb_collection_id = ?
			AND id NOT IN (SELECT item_id FROM collection_items WHERE collection_id = ?) AND `+playableSQL,
			auto.LibraryID, auto.TMDBID, m.ID).Scan(&missing)
		if err != nil {
			return false, err
		}
		if missing == 0 {
			return true, nil
		}
	}
	return false, nil
}

func (s *Store) countMembers(ctx context.Context, c *Collection, acc Access) (int, error) {
	cond, args := memberSQL(c)
	q := `SELECT COUNT(*) FROM items WHERE ` + cond + ` AND ` + playableSQL
	q, args = acc.Where("items", q, args)
	var n int
	err := s.db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

// staticCollectionMembers describes both membership models. Smart collections
// have individual filters and continue to use their existing query.
const staticCollectionMembers = `
	WITH members AS (
		SELECT c.id AS collection_id, i.id AS item_id, c.kind, c.grid_mode, 0 AS position
		FROM collections c JOIN items i ON i.library_id = c.library_id AND i.tmdb_collection_id = c.tmdb_id
		WHERE c.library_id = ?1 AND c.hidden = 0 AND c.kind = 'auto' AND i.kind = 'movie'
		UNION ALL
		SELECT c.id, ci.item_id, c.kind, c.grid_mode, ci.position
		FROM collections c JOIN collection_items ci ON ci.collection_id = c.id
		WHERE c.library_id = ?1 AND c.hidden = 0 AND c.kind NOT IN ('auto', 'smart')
	)`

func (s *Store) libraryCollectionCounts(ctx context.Context, libraryID int64, acc Access) (map[string]int, error) {
	q, args := acc.Where("items", staticCollectionMembers+`
		SELECT m.collection_id, COUNT(*) FROM members m JOIN items ON items.id = m.item_id
		WHERE `+playableSQL, []any{libraryID})
	rows, err := s.db.QueryContext(ctx, q+` GROUP BY m.collection_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var id string
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}
	return counts, rows.Err()
}

type CollectionPreview struct {
	Count int
	Items []Item
}

// CollectionPreviews returns the first visible title for each static collection and the full
// ordered member list for collections placed in a library grid. The list already has counts.
func (s *Store) CollectionPreviews(ctx context.Context, libraryID int64, gridMode string, acc Access) (map[string]CollectionPreview, error) {
	q, args := acc.Where("items", staticCollectionMembers+`, ranked AS (
		SELECT m.collection_id, items.id AS item_id, m.kind, m.grid_mode,
			COUNT(*) OVER (PARTITION BY m.collection_id) AS member_count,
			ROW_NUMBER() OVER (PARTITION BY m.collection_id ORDER BY
				CASE WHEN m.kind = 'auto' AND items.release_date = '' THEN 1 ELSE 0 END,
				CASE WHEN m.kind = 'auto' THEN items.release_date END,
				CASE WHEN m.kind = 'auto' THEN items.year END,
				m.position, items.sort_title) AS member_order
		FROM members m JOIN items ON items.id = m.item_id WHERE `+playableSQL, []any{libraryID})
	q += `)
		SELECT ` + prefixCols("items.", itemCols) + `, ranked.collection_id, ranked.member_count
		FROM ranked JOIN items ON items.id = ranked.item_id
		WHERE ranked.member_order = 1 OR
			(ranked.kind <> 'folder' AND COALESCE(NULLIF(ranked.grid_mode, ''), NULLIF(?, ''), 'items') <> 'items')
		ORDER BY ranked.collection_id, ranked.member_order`
	rows, err := s.db.QueryContext(ctx, q, append(args, gridMode)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	previews := map[string]CollectionPreview{}
	for rows.Next() {
		var id string
		var count int
		var item Item
		if err := scanItemWith(rows, &item, &id, &count); err != nil {
			return nil, err
		}
		preview := previews[id]
		preview.Count = count
		preview.Items = append(preview.Items, item)
		previews[id] = preview
	}
	return previews, rows.Err()
}

// GetCollection returns a collection, or nil.
func (s *Store) GetCollection(ctx context.Context, id string) (*Collection, error) {
	var c Collection
	err := scanCollection(s.db.QueryRowContext(ctx, `SELECT `+collectionCols+` FROM collections WHERE id = ?`, id), &c)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CollectionItems returns the titles of an automatic or manual collection a profile can see:
// automatic ones by release date, manual ones in their chosen order.
func (s *Store) CollectionItems(ctx context.Context, c *Collection, acc Access) ([]Item, error) {
	cond, args := memberSQL(c)
	q := `SELECT ` + itemCols + ` FROM items WHERE ` + cond + ` AND ` + playableSQL
	q, args = acc.Where("items", q, args)
	if c.Kind == "auto" {
		q += ` ORDER BY CASE WHEN release_date = '' THEN 1 ELSE 0 END, release_date, year, sort_title`
	} else {
		q += ` ORDER BY (SELECT position FROM collection_items WHERE collection_id = ? AND item_id = items.id), sort_title`
		args = append(args, c.ID)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		var it Item
		if err := scanItem(rows, &it); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

var ErrCollectionItems = errors.New("only movies and shows from the collection's library can be added")

// CreateCollection adds a manual or smart collection and optional manual members atomically.
// Validate every member under the writer transaction before making the collection visible.
func (s *Store) CreateCollection(ctx context.Context, c *Collection, itemIDs ...string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if len(itemIDs) > 0 && c.Kind != "manual" {
		return ErrCollectionItems
	}
	unique := make([]string, 0, len(itemIDs))
	seen := map[string]bool{}
	for _, id := range itemIDs {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	for part := range chunks(unique, 100) {
		args := []any{c.LibraryID}
		for _, id := range part {
			args = append(args, id)
		}
		var valid int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE library_id=? AND kind IN ('movie','series') AND id IN (`+placeholders(len(part))+`)`, args...).Scan(&valid); err != nil {
			return err
		}
		if valid != len(part) {
			return ErrCollectionItems
		}
	}
	created := *c
	created.ID = newID()
	if created.SortTitle == "" {
		created.SortTitle = SortTitle(created.Title)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO collections(id,library_id,kind,title,sort_title,overview,filter,locked_fields,grid_mode,created_at)
 VALUES (?,?,?,?,?,?,?,?,?,?)`, created.ID, created.LibraryID, created.Kind, created.Title, created.SortTitle, created.Overview, created.Filter, strings.Join(created.LockedFields, ","), created.GridMode, time.Now().Unix()); err != nil {
		return err
	}
	for position, id := range unique {
		if _, err = tx.ExecContext(ctx, `INSERT INTO collection_items(collection_id,item_id,position) VALUES (?,?,?)`, created.ID, id, position); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	*c = created
	return nil
}

// UpdateCollection changes a collection's title, sort title (following the title when empty),
// description, (smart ones) filter and locked fields.
func (s *Store) UpdateCollection(ctx context.Context, c *Collection) error {
	if c.SortTitle == "" {
		c.SortTitle = SortTitle(c.Title)
	}
	_, err := s.db.ExecContext(ctx, `UPDATE collections SET title = ?, sort_title = ?, overview = ?, filter = ?, locked_fields = ?, grid_mode = ? WHERE id = ?`,
		c.Title, c.SortTitle, c.Overview, c.Filter, strings.Join(c.LockedFields, ","), c.GridMode, c.ID)
	return err
}

// SetCollectionArt sets a collection's poster or backdrop ("poster" or "backdrop") to an image
// path (a TMDB path or an upload) and locks it; "" clears it and unlocks it.
func (s *Store) SetCollectionArt(ctx context.Context, id, field, path string) error {
	col := map[string]string{"poster": "poster_path", "backdrop": "backdrop_path"}[field]
	if col == "" {
		return errors.New("unknown collection artwork: " + field)
	}
	c, err := s.GetCollection(ctx, id)
	if err != nil || c == nil {
		return err
	}
	locked := slices.DeleteFunc(slices.Clone(c.LockedFields), func(f string) bool { return f == field })
	if path != "" {
		locked = append(locked, field)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE collections SET `+col+` = ?, locked_fields = ? WHERE id = ?`, path, strings.Join(locked, ","), id)
	return err
}

// SetCollectionInfo fills in an automatic collection's description and artwork from TMDB, except
// what the admin edited.
func (s *Store) SetCollectionInfo(ctx context.Context, id, overview, poster, backdrop string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE collections SET
		overview = CASE WHEN `+collectionLocked("overview")+` THEN overview ELSE ? END,
		poster_path = CASE WHEN ? <> '' AND NOT `+collectionLocked("poster")+` THEN ? ELSE poster_path END,
		backdrop_path = CASE WHEN ? <> '' AND NOT `+collectionLocked("backdrop")+` THEN ? ELSE backdrop_path END WHERE id = ?`,
		overview, poster, poster, backdrop, backdrop, id)
	return err
}

// DeleteCollection removes a manual or smart collection. An automatic one is hidden instead, so
// it doesn't come back with the next refresh.
func (s *Store) DeleteCollection(ctx context.Context, c *Collection) error {
	if c.Kind == "auto" || c.Kind == "folder" {
		_, err := s.db.ExecContext(ctx, `UPDATE collections SET hidden = 1 WHERE id = ?`, c.ID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM collections WHERE id = ?`, c.ID)
	return err
}

// AddToCollection appends titles to a manual collection (ones already in it stay where they are).
func (s *Store) AddToCollection(ctx context.Context, id string, itemIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var next int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) + 1 FROM collection_items WHERE collection_id = ?`, id).Scan(&next); err != nil {
		return err
	}
	for _, it := range itemIDs {
		res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO collection_items (collection_id, item_id, position) VALUES (?, ?, ?)`, id, it, next)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			next++
		}
	}
	return tx.Commit()
}

// RemoveFromCollection takes a title out of a manual collection.
func (s *Store) RemoveFromCollection(ctx context.Context, id, itemID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM collection_items WHERE collection_id = ? AND item_id = ?`, id, itemID)
	return err
}

// OrderCollection puts a manual collection's titles in this order (any left out keep their
// relative order after them).
func (s *Store) OrderCollection(ctx context.Context, id string, itemIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE collection_items SET position = position + 100000 WHERE collection_id = ?`, id); err != nil {
		return err
	}
	for i, it := range itemIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE collection_items SET position = ? WHERE collection_id = ? AND item_id = ?`, i, id, it); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ItemCollections lists the automatic and manual collections a title is in (automatic ones only
// when the library shows them).
func (s *Store) ItemCollections(ctx context.Context, it *Item, acc Access) ([]Collection, error) {
	var tmdbColl int64
	if err := s.db.QueryRowContext(ctx, `SELECT tmdb_collection_id FROM items WHERE id = ?`, it.ID).Scan(&tmdbColl); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []Collection{}, nil
		}
		return nil, err
	}
	var manual = map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT collection_id FROM collection_items WHERE item_id = ?`, it.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		manual[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if tmdbColl == 0 && len(manual) == 0 {
		return []Collection{}, nil
	}

	// A same-name manual copy can suppress the automatic collection even for a
	// missing title it doesn't contain. Load those peers without counting them.
	peers, err := s.readCollections(ctx, `WHERE library_id = ? AND hidden = 0 AND (
		(kind = 'auto' AND tmdb_id <> 0 AND tmdb_id = ?)
		OR (kind = 'manual' AND ?)
		OR id IN (SELECT collection_id FROM collection_items WHERE item_id = ?))`,
		it.LibraryID, tmdbColl, tmdbColl != 0, it.ID)
	if err != nil {
		return nil, err
	}
	var candidates []Collection
	for _, c := range peers {
		if (c.Kind == "auto" && c.TMDBID != 0 && c.TMDBID == tmdbColl) || manual[c.ID] {
			candidates = append(candidates, c)
		}
	}
	all, err := s.visibleCollections(ctx, it.LibraryID, candidates, peers, acc)
	if err != nil {
		return nil, err
	}
	// A title in a manual collection and an automatic one of the same name (Plex's copy of a TMDB
	// collection, imported) gets one row for them, the manual one's.
	inManual := map[string]bool{}
	for _, c := range all {
		if manual[c.ID] {
			inManual[strings.ToLower(strings.TrimSpace(c.Title))] = true
		}
	}
	out := []Collection{}
	for _, c := range all {
		if c.Kind == "auto" && tmdbColl != 0 && c.TMDBID == tmdbColl && !inManual[strings.ToLower(strings.TrimSpace(c.Title))] || manual[c.ID] {
			out = append(out, c)
		}
	}
	return out, nil
}

// SetLibraryCollectionMin sets how many titles a TMDB collection needs before it's shown
// (0 turns automatic collections off).
func (s *Store) SetLibraryCollectionMin(ctx context.Context, id int64, n int) error {
	_, err := s.db.ExecContext(ctx, `UPDATE libraries SET collection_min = ? WHERE id = ?`, n, id)
	return err
}

// SharedTMDBCollection is the TMDB collection a manual collection's titles have in common (more
// than half of them, at least two), for a collection made by hand or imported from Plex or
// Jellyfin, which has no TMDB link of its own; an automatic one's own. 0 for none.
func (s *Store) SharedTMDBCollection(ctx context.Context, c *Collection) (int64, error) {
	if c.Kind == "auto" {
		return c.TMDBID, nil
	}
	if c.Kind != "manual" {
		return 0, nil
	}
	var id, n, total int64
	err := s.db.QueryRowContext(ctx, `
		WITH members AS (SELECT items.tmdb_collection_id AS t FROM collection_items
			JOIN items ON items.id = collection_items.item_id WHERE collection_items.collection_id = ?)
		SELECT COALESCE((SELECT t FROM members WHERE t <> 0 GROUP BY t ORDER BY COUNT(*) DESC LIMIT 1), 0),
			COALESCE((SELECT COUNT(*) FROM members WHERE t = (SELECT t FROM members WHERE t <> 0 GROUP BY t ORDER BY COUNT(*) DESC LIMIT 1)), 0),
			(SELECT COUNT(*) FROM members)`, c.ID).Scan(&id, &n, &total)
	if err != nil {
		return 0, err
	}
	if id == 0 || n < 2 || n*2 <= total {
		return 0, nil
	}
	return id, nil
}

// FillCollectionInfo sets a collection's description, poster and background where it has none
// and they aren't set by hand; it never changes what's already there.
func (s *Store) FillCollectionInfo(ctx context.Context, id, overview, poster, backdrop string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE collections SET
		overview = CASE WHEN overview = '' AND ? <> '' AND NOT `+collectionLocked("overview")+` THEN ? ELSE overview END,
		poster_path = CASE WHEN poster_path = '' AND ? <> '' AND NOT `+collectionLocked("poster")+` THEN ? ELSE poster_path END,
		backdrop_path = CASE WHEN backdrop_path = '' AND ? <> '' AND NOT `+collectionLocked("backdrop")+` THEN ? ELSE backdrop_path END
		WHERE id = ?`, overview, overview, poster, poster, backdrop, backdrop, id)
	return err
}
