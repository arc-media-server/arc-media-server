package store

import (
	"cmp"
	"context"
	"slices"
)

// WatchedTitle is what a profile has played of one movie or show in a library (a show through
// its episodes).
type WatchedTitle struct {
	ID string
	// When any of it was last played or part-played (unix seconds), 0 if never recorded.
	LastPlayedAt int64
	// The same, counting only the title itself for a movie and only the episodes for a show:
	// what "last watched something they're in" means for the people credited on it.
	CreditsAt int64
	// How many of its items (the movie, or the show's episodes) are watched.
	Watches int
}

// WatchHistory is a profile's watch history in a library, one entry per movie or show, most
// recently played first. Recommended reads it once for the rows that start from what the
// profile watched (Because You Watched, favourite genres, featured people), instead of each
// of those reading the whole history again.
type WatchHistory struct {
	Titles []WatchedTitle
	byID   map[string]int
}

// LibraryWatchHistory reads a profile's watch history in a library, one entry per title.
func (s *Store) LibraryWatchHistory(ctx context.Context, userID string, libraryID int64) (*WatchHistory, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(CASE WHEN i.kind = 'episode' THEN i.parent_id END, i.id) AS title_id,
			COALESCE(MAX(u.last_played_at), 0),
			COALESCE(MAX(CASE WHEN i.kind <> 'series' THEN u.last_played_at END), 0),
			SUM(u.played = 1)
		FROM user_item_data u JOIN items i ON i.id = u.item_id
		WHERE u.user_id = ? AND i.library_id = ? AND (u.played = 1 OR u.position_ms > 0)
		GROUP BY 1`, userID, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	h := &WatchHistory{byID: map[string]int{}}
	for rows.Next() {
		var t WatchedTitle
		if err := rows.Scan(&t.ID, &t.LastPlayedAt, &t.CreditsAt, &t.Watches); err != nil {
			return nil, err
		}
		h.Titles = append(h.Titles, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.SortFunc(h.Titles, func(a, b WatchedTitle) int {
		return cmp.Or(cmp.Compare(b.LastPlayedAt, a.LastPlayedAt), cmp.Compare(a.ID, b.ID))
	})
	for i, t := range h.Titles {
		h.byID[t.ID] = i
	}
	return h, nil
}

// RecentlyWatchedTitles lists the movies and shows in a library a profile watched most recently
// (a show counts when any of its episodes was played), newest first.
func (s *Store) RecentlyWatchedTitles(ctx context.Context, userID string, libraryID int64, limit int) ([]Item, error) {
	h, err := s.LibraryWatchHistory(ctx, userID, libraryID)
	if err != nil {
		return nil, err
	}
	return s.RecentlyWatchedFrom(ctx, h, limit)
}

// RecentlyWatchedFrom is RecentlyWatchedTitles from a history already read.
func (s *Store) RecentlyWatchedFrom(ctx context.Context, h *WatchHistory, limit int) ([]Item, error) {
	var ids []string
	for _, t := range h.Titles {
		if len(ids) == limit || t.LastPlayedAt == 0 {
			break // sorted newest first, so the rest were never given a play time
		}
		ids = append(ids, t.ID)
	}
	found, err := s.ItemsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(ids))
	for _, id := range ids {
		if it, ok := found[id]; ok {
			out = append(out, it)
		}
	}
	return out, nil
}

// FavouriteGenres lists the genres of the titles a profile has watched in a library, most watched
// first.
func (s *Store) FavouriteGenres(ctx context.Context, userID string, libraryID int64, limit int) ([]string, error) {
	h, err := s.LibraryWatchHistory(ctx, userID, libraryID)
	if err != nil {
		return nil, err
	}
	return s.FavouriteGenresFrom(ctx, h, limit)
}

// FavouriteGenresFrom is FavouriteGenres from a history already read: each genre weighs as
// many watched items (movies or episodes) as its titles have.
func (s *Store) FavouriteGenresFrom(ctx context.Context, h *WatchHistory, limit int) ([]string, error) {
	var ids []string
	for _, t := range h.Titles {
		if t.Watches > 0 {
			ids = append(ids, t.ID)
		}
	}
	weight := map[string]int{}
	for chunk := range chunks(ids, 500) {
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		rows, err := s.db.QueryContext(ctx, `SELECT item_id, name FROM item_genres WHERE item_id IN (`+placeholders(len(chunk))+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, name string
			if err := rows.Scan(&id, &name); err != nil {
				rows.Close()
				return nil, err
			}
			weight[name] += h.Titles[h.byID[id]].Watches
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	var genres []string
	for g := range weight {
		genres = append(genres, g)
	}
	slices.SortFunc(genres, func(a, b string) int { return cmp.Or(cmp.Compare(weight[b], weight[a]), cmp.Compare(a, b)) })
	return genres[:min(limit, len(genres))], nil
}
