package store

import (
	"cmp"
	"context"
	"slices"
)

// LibraryGenres lists the genres of at least minTitles movies or shows (kind) in a library that a
// profile can see, by name.
func (s *Store) LibraryGenres(ctx context.Context, libraryID int64, kind string, minTitles int, acc Access) ([]string, error) {
	q, args := acc.Where("items", `SELECT g.name FROM item_genres g JOIN items ON items.id = g.item_id
		WHERE items.library_id = ? AND items.kind = ? AND `+playableSQL, []any{libraryID, kind})
	rows, err := s.db.QueryContext(ctx, q+` GROUP BY g.name HAVING COUNT(*) >= ? ORDER BY g.name`, append(args, minTitles)...)
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

// FeaturedPerson is an actor or director credited on several titles of a library.
type FeaturedPerson struct {
	ID   int64
	Name string
	// actor or director
	Role   string
	Titles int
	// When the profile last played something they're credited on (unix seconds), 0 if never.
	LastWatchedAt int64
}

// FeaturedPeople lists the lead actors (the first ten credited) and directors of at least
// minTitles movies or shows (kind) in a library that a profile can see: those credited on what
// the profile watched most recently first, then those with the most titles.
func (s *Store) FeaturedPeople(ctx context.Context, userID string, libraryID int64, kind string, minTitles int, acc Access, limit int) ([]FeaturedPerson, error) {
	h, err := s.LibraryWatchHistory(ctx, userID, libraryID)
	if err != nil {
		return nil, err
	}
	return s.FeaturedPeopleFrom(ctx, h, libraryID, kind, minTitles, acc, limit)
}

// leadCreditSQL picks the credits FeaturedPeople counts: directors and the first ten actors.
const leadCreditSQL = `(p.role = 'director' OR (p.role = 'actor' AND p.ord < 10))`

// featuredPeopleSQL counts the lead credits of a library's playable titles that a profile can
// see, per person and role, keeping those on at least minTitles.
func featuredPeopleSQL(libraryID int64, kind string, minTitles int, acc Access) (string, []any) {
	q, args := acc.Where("items", `SELECT p.person_id, pe.name, p.role, COUNT(*) FROM items
		CROSS JOIN item_people p ON p.item_id = items.id JOIN people pe ON pe.id = p.person_id
		WHERE items.library_id = ? AND items.kind = ? AND `+leadCreditSQL+` AND `+playableSQL, []any{libraryID, kind})
	return q + ` GROUP BY p.person_id, pe.name, p.role HAVING COUNT(*) >= ?`, append(args, minTitles)
}

// featuredPeopleBatch is how many watched titles FeaturedPeopleFrom looks at a time. One batch
// almost always finds enough people.
var featuredPeopleBatch = 50

// FeaturedPeopleFrom is FeaturedPeople from a history already read. The people come from one
// count over the library's credits; when each was last watched comes from walking the
// history newest first, only as far as it takes to settle the first limit places, instead of
// working it out for every title (and every episode of every show) in the library.
func (s *Store) FeaturedPeopleFrom(ctx context.Context, h *WatchHistory, libraryID int64, kind string, minTitles int, acc Access, limit int) ([]FeaturedPerson, error) {
	type key struct {
		id   int64
		role string
	}
	q, args := featuredPeopleSQL(libraryID, kind, minTitles, acc)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	people := map[key]*FeaturedPerson{}
	for rows.Next() {
		p := &FeaturedPerson{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Role, &p.Titles); err != nil {
			rows.Close()
			return nil, err
		}
		people[key{p.ID, p.Role}] = p
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}

	// The watched titles by when they were last watched for their credits, newest first.
	watched := make([]WatchedTitle, 0, len(h.Titles))
	for _, t := range h.Titles {
		if t.CreditsAt > 0 {
			watched = append(watched, t)
		}
	}
	slices.SortStableFunc(watched, func(a, b WatchedTitle) int { return cmp.Compare(b.CreditsAt, a.CreditsAt) })
	dated := 0
	for start := 0; start < len(watched); start += featuredPeopleBatch {
		// A person is placed by the newest title they're on. Once limit people are dated, anyone
		// still undated can only be on older titles, so they rank below all of those: stop at
		// the first batch that starts older than the newest limit dates.
		if start > 0 && dated >= limit && watched[start].CreditsAt < watched[start-1].CreditsAt {
			break
		}
		batch := watched[start:min(start+featuredPeopleBatch, len(watched))]
		at := map[string]int64{}
		args := make([]any, 0, len(batch)+2)
		for _, t := range batch {
			at[t.ID] = t.CreditsAt
			args = append(args, t.ID)
		}
		q, args := acc.Where("items", `SELECT items.id, p.person_id, p.role FROM items
			CROSS JOIN item_people p ON p.item_id = items.id
			WHERE items.id IN (`+placeholders(len(batch))+`) AND items.library_id = ? AND items.kind = ?
				AND `+leadCreditSQL+` AND `+playableSQL, append(args, libraryID, kind))
		rows, err := s.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var k key
			if err := rows.Scan(&id, &k.id, &k.role); err != nil {
				rows.Close()
				return nil, err
			}
			if p := people[k]; p != nil && at[id] > p.LastWatchedAt {
				if p.LastWatchedAt == 0 {
					dated++
				}
				p.LastWatchedAt = at[id]
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}

	out := make([]FeaturedPerson, 0, len(people))
	for _, p := range people {
		out = append(out, *p)
	}
	slices.SortFunc(out, func(a, b FeaturedPerson) int {
		return cmp.Or(cmp.Compare(b.LastWatchedAt, a.LastWatchedAt), cmp.Compare(b.Titles, a.Titles),
			cmp.Compare(a.Name, b.Name), cmp.Compare(a.Role, b.Role))
	})
	return out[:min(limit, len(out))], nil
}
