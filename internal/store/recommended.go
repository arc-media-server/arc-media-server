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
	// When the profile last played each title of the library (a show by its episodes).
	q, args := acc.Where("items", `WITH watched AS (
			SELECT COALESCE(CASE WHEN w.kind = 'episode' THEN w.parent_id END, w.id) AS item_id, MAX(u.last_played_at) AS at
			FROM user_item_data u JOIN items w ON w.id = u.item_id
			WHERE u.user_id = ? AND w.library_id = ? AND u.last_played_at IS NOT NULL AND (u.played = 1 OR u.position_ms > 0)
			GROUP BY 1)
		SELECT p.person_id, pe.name, p.role, COUNT(*), MAX(COALESCE(watched.at, 0)) FROM item_people p
		JOIN items ON items.id = p.item_id JOIN people pe ON pe.id = p.person_id
		LEFT JOIN watched ON watched.item_id = items.id
		WHERE items.library_id = ? AND items.kind = ? AND (p.role = 'director' OR (p.role = 'actor' AND p.ord < 10))
			AND `+playableSQL, []any{userID, libraryID, libraryID, kind})
	rows, err := s.db.QueryContext(ctx, q+` GROUP BY p.person_id, p.role HAVING COUNT(*) >= ?
		ORDER BY 5 DESC, 4 DESC, pe.name, p.role LIMIT ?`, append(args, minTitles, limit)...)
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
