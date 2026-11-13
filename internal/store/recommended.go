package store

import "context"

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
	q, args := featuredPeopleSQL(userID, libraryID, kind, minTitles, limit, acc)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FeaturedPerson{}
	for rows.Next() {
		var p FeaturedPerson
		if err := rows.Scan(&p.ID, &p.Name, &p.Role, &p.Titles, &p.LastWatchedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// featuredPeopleSQL is FeaturedPeople's query.
func featuredPeopleSQL(userID string, libraryID int64, kind string, minTitles int, limit int, acc Access) (string, []any) {
	// The library's playable titles with when the profile last played each (a show by its
	// episodes), worked out once per title by key, then joined to their cast and crew by title.
	// A summary of the whole history joined to every credit was scanned in full for each credit:
	// hundreds of milliseconds for a couple of thousand films, more on a slower server.
	titles, args := acc.Where("items", `SELECT items.id, COALESCE(CASE WHEN items.kind = 'series' THEN (
				SELECT MAX(u.last_played_at) FROM items w CROSS JOIN user_item_data u ON u.item_id = w.id AND u.user_id = ?
				WHERE w.parent_id = items.id AND u.last_played_at IS NOT NULL AND (u.played = 1 OR u.position_ms > 0))
			ELSE (SELECT u.last_played_at FROM user_item_data u WHERE u.user_id = ? AND u.item_id = items.id
				AND u.last_played_at IS NOT NULL AND (u.played = 1 OR u.position_ms > 0)) END, 0) AS at
		FROM items WHERE items.library_id = ? AND items.kind = ? AND `+playableSQL, []any{userID, userID, libraryID, kind})
	q := `WITH titles AS MATERIALIZED (` + titles + `)
		SELECT p.person_id, pe.name, p.role, COUNT(*), MAX(titles.at) FROM titles
		CROSS JOIN item_people p ON p.item_id = titles.id JOIN people pe ON pe.id = p.person_id
		WHERE (p.role = 'director' OR (p.role = 'actor' AND p.ord < 10))`
	return q + ` GROUP BY p.person_id, p.role HAVING COUNT(*) >= ?
		ORDER BY 5 DESC, 4 DESC, pe.name, p.role LIMIT ?`, append(args, minTitles, limit)
}
