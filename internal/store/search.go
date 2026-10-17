package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// SearchTitles returns every movie and show a profile can see, for ranking against a search.
func (s *Store) SearchTitles(ctx context.Context, acc Access) ([]Item, error) {
	q := `SELECT ` + itemCols + ` FROM items WHERE kind IN ('movie', 'series') AND ` + playableSQL
	var args []any
	q, args = acc.Where("items", q, args)
	return s.queryItems(ctx, q, args...)
}

// SetAltTitlesKeywords replaces the other names and keywords of a movie or show.
func (s *Store) SetAltTitlesKeywords(ctx context.Context, itemID string, titles, keywords []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []string{`DELETE FROM item_alt_titles WHERE item_id = ?`, `DELETE FROM item_keywords WHERE item_id = ?`} {
		if _, err := tx.ExecContext(ctx, q, itemID); err != nil {
			return err
		}
	}
	for _, t := range titles {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_alt_titles (item_id, title) VALUES (?, ?)`, itemID, t); err != nil {
			return err
		}
	}
	for _, k := range keywords {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO item_keywords (item_id, keyword) VALUES (?, ?)`, itemID, strings.ToLower(k)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AltTitles returns the other names of every movie and show, by item id.
func (s *Store) AltTitles(ctx context.Context) (map[string][]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT item_id, title FROM item_alt_titles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var id, t string
		if err := rows.Scan(&id, &t); err != nil {
			return nil, err
		}
		out[id] = append(out[id], t)
	}
	return out, rows.Err()
}

// SearchEpisodes returns episodes whose titles contain the text, newest shows' first.
func (s *Store) SearchEpisodes(ctx context.Context, text string, acc Access, limit int) ([]Item, error) {
	q := `SELECT ` + itemCols + ` FROM items WHERE kind = 'episode' AND title LIKE ? AND ` + playableSQL
	args := []any{"%" + strings.ReplaceAll(text, "%", "") + "%"}
	q, args = acc.Where("items", q, args)
	q += ` ORDER BY sort_title LIMIT ?`
	return s.queryItems(ctx, q, append(args, limit)...)
}

func (s *Store) queryItems(ctx context.Context, q string, args ...any) ([]Item, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
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

// Person is someone in the cast or crew of titles in the libraries.
type Person struct {
	ID          int64  `json:"id"`
	TMDBID      int64  `json:"tmdb_id"`
	Name        string `json:"name"`
	ProfilePath string `json:"-"`
	// How many titles in the libraries they're in.
	Count int `json:"count"`
	// What they mostly do: acting, directing, writing or creating.
	KnownFor string `json:"known_for"`
}

// SearchPeople returns everyone credited on a present title, with how many they're in.
func (s *Store) SearchPeople(ctx context.Context) ([]Person, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT p.id, p.tmdb_id, p.name, p.profile_path, COUNT(DISTINCT ip.item_id),
			(SELECT role FROM item_people r WHERE r.person_id = p.id GROUP BY role ORDER BY COUNT(*) DESC LIMIT 1)
		FROM people p JOIN item_people ip ON ip.person_id = p.id JOIN items ON items.id = ip.item_id
		WHERE `+playableSQL+`
		GROUP BY p.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		var known sql.NullString
		if err := rows.Scan(&p.ID, &p.TMDBID, &p.Name, &p.ProfilePath, &p.Count, &known); err != nil {
			return nil, err
		}
		p.KnownFor = known.String
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetPerson returns a person, or nil.
func (s *Store) GetPerson(ctx context.Context, id int64) (*Person, error) {
	var p Person
	err := s.db.QueryRowContext(ctx, `SELECT id, tmdb_id, name, profile_path FROM people WHERE id = ?`, id).
		Scan(&p.ID, &p.TMDBID, &p.Name, &p.ProfilePath)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// PersonCredit is a title someone is in, and what they did on it.
type PersonCredit struct {
	Item      Item
	Roles     []string
	Character string
}

// PersonCredits lists the titles a profile can see that someone is in, newest first.
func (s *Store) PersonCredits(ctx context.Context, personID int64, acc Access) ([]PersonCredit, error) {
	q := `SELECT ` + itemCols + `,
			(SELECT GROUP_CONCAT(role) FROM item_people r WHERE r.item_id = items.id AND r.person_id = ?),
			COALESCE((SELECT character FROM item_people r WHERE r.item_id = items.id AND r.person_id = ? AND role = 'actor'), '')
		FROM items WHERE id IN (SELECT item_id FROM item_people WHERE person_id = ?) AND ` + playableSQL
	args := []any{personID, personID, personID}
	q, args = acc.Where("items", q, args)
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY year DESC, sort_title`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PersonCredit{}
	for rows.Next() {
		var c PersonCredit
		var roles string
		if err := scanItemWith(rows, &c.Item, &roles, &c.Character); err != nil {
			return nil, err
		}
		c.Roles = strings.Split(roles, ",")
		out = append(out, c)
	}
	return out, rows.Err()
}

// GenreCount is how many titles a library has in a genre.
type GenreCount struct {
	LibraryID int64  `json:"library_id"`
	Name      string `json:"name"`
	Count     int    `json:"count"`
}

// GenreCounts lists every genre in each library, with how many titles a profile can see in it.
func (s *Store) GenreCounts(ctx context.Context, acc Access) ([]GenreCount, error) {
	q := `SELECT items.library_id, g.name, COUNT(*) FROM item_genres g JOIN items ON items.id = g.item_id
		WHERE items.kind IN ('movie', 'series') AND ` + playableSQL
	var args []any
	q, args = acc.Where("items", q, args)
	rows, err := s.db.QueryContext(ctx, q+` GROUP BY items.library_id, g.name`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GenreCount
	for rows.Next() {
		var g GenreCount
		if err := rows.Scan(&g.LibraryID, &g.Name, &g.Count); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}
