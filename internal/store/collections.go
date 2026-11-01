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
	return `instr(',' || locked_fields || ',', ',` + field + `,') > 0`
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
			title = CASE WHEN `+collectionLocked("title")+` THEN title ELSE excluded.title END,
			sort_title = CASE WHEN `+collectionLocked("sort_title")+` OR `+collectionLocked("title")+` THEN sort_title ELSE excluded.sort_title END,
			poster_path = CASE WHEN `+collectionLocked("poster")+` THEN poster_path ELSE excluded.poster_path END,
			backdrop_path = CASE WHEN `+collectionLocked("backdrop")+` THEN backdrop_path ELSE excluded.backdrop_path END`,
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
	var min int
	if err := s.db.QueryRowContext(ctx, `SELECT collection_min FROM libraries WHERE id = ?`, libraryID).Scan(&min); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+collectionCols+` FROM collections
		WHERE library_id = ? AND hidden = 0 ORDER BY sort_title`, libraryID)
	if err != nil {
		return nil, err
	}
	var all []Collection
	for rows.Next() {
		var c Collection
		if err := scanCollection(rows, &c); err != nil {
			rows.Close()
			return nil, err
		}
		all = append(all, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []Collection{}
	for _, c := range all {
		if c.Kind == "auto" {
			dup, err := s.autoDuplicated(ctx, &c, all)
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
			if c.Kind == "auto" && min == 0 {
				continue
			}
			n, err := s.countMembers(ctx, &c, acc)
			if err != nil {
				return nil, err
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

// CreateCollection adds a manual or smart collection.
func (s *Store) CreateCollection(ctx context.Context, c *Collection) error {
	c.ID = newID()
	c.SortTitle = SortTitle(c.Title)
	_, err := s.db.ExecContext(ctx, `INSERT INTO collections (id, library_id, kind, title, sort_title, overview, filter, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, c.ID, c.LibraryID, c.Kind, c.Title, c.SortTitle, c.Overview, c.Filter, time.Now().Unix())
	return err
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
	all, err := s.Collections(ctx, it.LibraryID, acc)
	if err != nil {
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
	var tmdbColl int64
	s.db.QueryRowContext(ctx, `SELECT tmdb_collection_id FROM items WHERE id = ?`, it.ID).Scan(&tmdbColl)
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
