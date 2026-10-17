package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// Collection groups titles in a library. Automatic ones come from TMDB (every movie TMDB puts in
// the collection); manual ones hold the titles the admin chose, in order; smart ones hold whatever
// their filter matches.
type Collection struct {
	ID           string `json:"id"`
	LibraryID    int64  `json:"library_id"`
	Kind         string `json:"kind"` // auto, manual, smart
	TMDBID       int64  `json:"tmdb_id,omitempty"`
	Title        string `json:"title"`
	SortTitle    string `json:"sort_title"`
	Overview     string `json:"overview,omitempty"`
	PosterPath   string `json:"-"`
	BackdropPath string `json:"-"`
	// Smart collections: the filter, as JSON.
	Filter string `json:"filter,omitempty"`
	Hidden bool   `json:"-"`
	// How many titles the profile can see in it (filled in when listing).
	Count int `json:"count"`
}

const collectionCols = `id, library_id, kind, tmdb_id, title, sort_title, overview, poster_path, backdrop_path, filter, hidden`

func scanCollection(row interface{ Scan(...any) error }, c *Collection) error {
	return row.Scan(&c.ID, &c.LibraryID, &c.Kind, &c.TMDBID, &c.Title, &c.SortTitle, &c.Overview, &c.PosterPath,
		&c.BackdropPath, &c.Filter, &c.Hidden)
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
			title = excluded.title, sort_title = excluded.sort_title,
			poster_path = excluded.poster_path, backdrop_path = excluded.backdrop_path`,
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
func (s *Store) Collections(ctx context.Context, libraryID int64, maxAge int) ([]Collection, error) {
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
		switch c.Kind {
		case "smart":
			c.Count = -1
		default:
			if c.Kind == "auto" && min == 0 {
				continue
			}
			n, err := s.countMembers(ctx, &c, maxAge)
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

func (s *Store) countMembers(ctx context.Context, c *Collection, maxAge int) (int, error) {
	cond, args := memberSQL(c)
	q := `SELECT COUNT(*) FROM items WHERE ` + cond + ` AND ` + playableSQL
	if maxAge > 0 {
		q += ` AND ` + AgeAllowedSQL("items")
		args = append(args, maxAge)
	}
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
func (s *Store) CollectionItems(ctx context.Context, c *Collection, maxAge int) ([]Item, error) {
	cond, args := memberSQL(c)
	q := `SELECT ` + itemCols + ` FROM items WHERE ` + cond + ` AND ` + playableSQL
	if maxAge > 0 {
		q += ` AND ` + AgeAllowedSQL("items")
		args = append(args, maxAge)
	}
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

// UpdateCollection changes a collection's title, description and (smart ones) filter.
func (s *Store) UpdateCollection(ctx context.Context, c *Collection) error {
	_, err := s.db.ExecContext(ctx, `UPDATE collections SET title = ?, sort_title = ?, overview = ?, filter = ? WHERE id = ?`,
		c.Title, SortTitle(c.Title), c.Overview, c.Filter, c.ID)
	return err
}

// SetCollectionInfo fills in an automatic collection's description and artwork from TMDB.
func (s *Store) SetCollectionInfo(ctx context.Context, id, overview, poster, backdrop string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE collections SET overview = ?,
		poster_path = CASE WHEN ? <> '' THEN ? ELSE poster_path END,
		backdrop_path = CASE WHEN ? <> '' THEN ? ELSE backdrop_path END WHERE id = ?`,
		overview, poster, poster, backdrop, backdrop, id)
	return err
}

// DeleteCollection removes a manual or smart collection. An automatic one is hidden instead, so
// it doesn't come back with the next refresh.
func (s *Store) DeleteCollection(ctx context.Context, c *Collection) error {
	if c.Kind == "auto" {
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
func (s *Store) ItemCollections(ctx context.Context, it *Item, maxAge int) ([]Collection, error) {
	all, err := s.Collections(ctx, it.LibraryID, maxAge)
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
	out := []Collection{}
	for _, c := range all {
		if (c.Kind == "auto" && tmdbColl != 0 && c.TMDBID == tmdbColl) || manual[c.ID] {
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
