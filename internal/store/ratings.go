package store

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

// ratingAges maps age-rating labels from around the world to the minimum age they mean.
var ratingAges = map[string]int{
	// UK (BBFC)
	"U": 0, "UC": 0, "PG": 8, "12A": 12, "12": 12, "15": 15, "18": 18, "R18": 18,
	// US film (MPA) and TV
	"G": 0, "PG-13": 13, "R": 17, "NC-17": 18,
	"TV-Y": 0, "TV-G": 0, "TV-Y7": 7, "TV-Y7-FV": 7, "TV-PG": 10, "TV-14": 14, "TV-MA": 17,
	// Common elsewhere
	"AL": 0, "MA15+": 15, "MA 15+": 15, "R18+": 18,
	"FSK 0": 0, "FSK 6": 6, "FSK 12": 12, "FSK 16": 16, "FSK 18": 18,
}

// RatingAge is the minimum age an age rating means (e.g. "12A" → 12, "TV-MA" → 17), or -1 when
// it's unknown or the title is unrated.
func RatingAge(rating string) int {
	r := strings.ToUpper(strings.TrimSpace(rating))
	if r == "" || r == "NR" || r == "UNRATED" || r == "NOT RATED" {
		return -1
	}
	if age, ok := ratingAges[r]; ok {
		return age
	}
	// Plain ages, as many countries use: "16", "K-12", "+13", "16+".
	digits := strings.TrimFunc(r, func(c rune) bool { return c < '0' || c > '9' })
	if i := strings.IndexFunc(digits, func(c rune) bool { return c < '0' || c > '9' }); i >= 0 {
		digits = digits[:i]
	}
	if n, err := strconv.Atoi(digits); err == nil && n >= 0 && n <= 21 {
		return n
	}
	return -1
}

// syncMinAges recomputes min_age from content_rating for the given items, or every item that has
// a rating when none are given.
func (s *Store) syncMinAges(ctx context.Context, ids ...string) error {
	q := `SELECT id, content_rating FROM items WHERE content_rating <> ''`
	args := []any{}
	if len(ids) > 0 {
		q = `SELECT id, content_rating FROM items WHERE id IN (` + placeholders(len(ids)) + `)`
		for _, id := range ids {
			args = append(args, id)
		}
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	ages := map[string]int{}
	for rows.Next() {
		var id, rating string
		if err := rows.Scan(&id, &rating); err != nil {
			rows.Close()
			return err
		}
		ages[id] = RatingAge(rating)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for id, age := range ages {
		if _, err := s.db.ExecContext(ctx, `UPDATE items SET min_age = ? WHERE id = ? AND min_age <> ?`, age, id, age); err != nil {
			return err
		}
	}
	return nil
}

// AgeAllowedSQL is an SQL condition that an item (alias i; episodes by their series) is suitable
// for a profile limited to maxAge. Use it only when there is a limit.
func AgeAllowedSQL(alias string) string {
	a := alias + "."
	return `(CASE WHEN ` + a + `kind = 'episode' THEN (SELECT p.min_age FROM items p WHERE p.id = ` + a +
		`parent_id) ELSE ` + a + `min_age END) BETWEEN 0 AND ?`
}

// Access is what a profile may see: titles rated for its age limit, in the libraries it's allowed.
type Access struct {
	// The oldest age rating allowed; 0 for no limit.
	MaxAge int
	// The libraries allowed; nil for all of them.
	Libraries []int64
}

// Conds are SQL conditions (and their arguments) that an item (alias, e.g. "items") is visible.
func (a Access) Conds(alias string) ([]string, []any) {
	var conds []string
	var args []any
	if a.MaxAge > 0 {
		conds = append(conds, AgeAllowedSQL(alias))
		args = append(args, a.MaxAge)
	}
	if a.Libraries != nil {
		ids := make([]string, 0, len(a.Libraries)+1)
		for _, id := range a.Libraries {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
		if len(ids) == 0 {
			ids = append(ids, "-1") // no libraries at all
		}
		conds = append(conds, alias+".library_id IN ("+strings.Join(ids, ",")+")")
	}
	return conds, args
}

// Where adds the access conditions to a query that already has a WHERE clause.
func (a Access) Where(alias, q string, args []any) (string, []any) {
	conds, more := a.Conds(alias)
	for _, c := range conds {
		q += " AND " + c
	}
	return q, append(args, more...)
}

// LibraryAllowed reports whether a library is one the profile may see.
func (a Access) LibraryAllowed(id int64) bool {
	return a.Libraries == nil || slices.Contains(a.Libraries, id)
}

// CanSee reports whether a profile with this access may see an item (episodes by their show's
// rating).
func (s *Store) CanSee(ctx context.Context, it *Item, a Access) (bool, error) {
	if !a.LibraryAllowed(it.LibraryID) {
		return false, nil
	}
	if a.MaxAge <= 0 {
		return true, nil
	}
	id := it.ID
	if it.Kind == "episode" {
		id = it.ParentID
	}
	var age int
	if err := s.db.QueryRowContext(ctx, `SELECT min_age FROM items WHERE id = ?`, id).Scan(&age); err != nil {
		return false, err
	}
	return age >= 0 && age <= a.MaxAge, nil
}
