package api

import (
	"net/http"
	"slices"
	"time"

	"ams/internal/store"
)

// similar: GET /api/items/{id}/similar is "More like this": movies or shows from the same library,
// TMDB's recommendations first, then titles sharing its collection, genres and cast.
func (s *Server) similar(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if it.Kind != "movie" && it.Kind != "series" {
		writeJSON(w, http.StatusOK, []itemDTO{})
		return
	}
	items, err := s.store.Similar(r.Context(), it, access(r), 20)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := toDTOs(items)
	if err := s.fillUserData(r, ptrs(out)); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// recommendedRow is one row of a library's Recommended view. Clients title it from its type and
// parameters, in the profile's language.
type recommendedRow struct {
	Style string `json:"style,omitempty"`
	// What the row is: continue, recently_released, recently_added, start_watching,
	// because_you_watched, top_rated, recently_watched, rediscover, more_in_genre, top_in_genre,
	// top_by_person or top_unwatched.
	Type  string    `json:"type"`
	Items []itemDTO `json:"items"`
	// The title (because_you_watched), genre or person a row is about, or the library's name
	// (recently_added).
	Name string `json:"name,omitempty"`
	// The genre of more_in_genre and top_in_genre, for a library page showing more.
	Genre string `json:"genre,omitempty"`
	// The person of top_by_person (for their page) and how they're credited: actor or director.
	PersonID int64  `json:"person_id,omitempty"`
	Role     string `json:"role,omitempty"`
}

// Recommended rows show this many titles each.
const recommendedLimit = 20

// Titles last finished longer ago than this can be rediscovered.
const rediscoverAfter = 182 * 24 * time.Hour

// recommended: GET /api/libraries/{id}/recommended is a library's Recommended view for the
// profile, in this order: Continue Watching (what it's part-way through here and the next
// episode of shows it's watching), Recently Released, Recently Added, Start Watching (shows not
// started), "Because you watched" rows for its recent titles, Top Rated, Recently Watched
// (episodes, in a TV library), Rediscover (finished more than six months ago), more of its
// favourite genres, the top titles of a genre and of an actor or director (both changing
// daily), and the best rated it hasn't seen. Each row has a type and parameters (name, genre,
// person_id, role) rather than a title; rows without anything are left out.
func (s *Server) recommended(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	rows, err := s.recommendedRows(r, lib)
	if err != nil {
		s.fail(w, err)
		return
	}
	layout, err := s.readRecommendedLayout(r, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": arrangeRecommended(rows, layout)})
}

func (s *Server) recommendedRows(r *http.Request, lib *store.Library) ([]recommendedRow, error) {
	ctx := r.Context()
	userID := currentUserID(r)
	acc := access(r)
	kind := "movie"
	if lib.Kind == "shows" {
		kind = "series"
	}
	now := time.Now()
	day := int(now.Unix() / 86400)
	rows := []recommendedRow{}
	add := func(row recommendedRow) {
		if len(row.Items) > 0 {
			rows = append(rows, row)
		}
	}
	// list lists the library's titles (or episodes) with the profile's watch state.
	list := func(q store.ItemQuery) ([]itemDTO, error) {
		q.LibraryID, q.UserID, q.Access, q.Limit = lib.ID, userID, acc, recommendedLimit
		if q.Kind == "" {
			q.Kind = kind
		}
		items, _, err := s.store.ListItems(ctx, q)
		if err != nil {
			return nil, err
		}
		if q.Kind == "episode" {
			entries := make([]homeEntry, len(items))
			for i, it := range items {
				entries[i] = homeEntry{item: it}
			}
			return s.homeDTOs(r, entries)
		}
		dtos := toDTOs(items)
		return dtos, s.fillUserData(r, ptrs(dtos))
	}
	// unseen keeps the titles the profile hasn't finished, with their watch state.
	unseen := func(items []store.Item) ([]itemDTO, error) {
		dtos := toDTOs(items)
		if err := s.fillUserData(r, ptrs(dtos)); err != nil {
			return nil, err
		}
		out := []itemDTO{}
		for _, d := range dtos {
			if d.Kind == "series" && d.UnwatchedCount != nil && *d.UnwatchedCount == 0 {
				continue
			}
			if d.Kind == "movie" && d.UserData != nil && d.UserData.Played {
				continue
			}
			out = append(out, d)
		}
		return out, nil
	}

	// Continue Watching, for this library only.
	here := acc
	here.Libraries = []int64{lib.ID}
	inProgress, err := s.inProgress(r, here)
	if err != nil {
		return nil, err
	}
	var next []homeEntry
	if lib.Kind == "shows" {
		if next, err = s.upNext(r, here); err != nil {
			return nil, err
		}
	}
	items, err := s.homeDTOs(r, mergeHome(inProgress, next, recommendedLimit))
	if err != nil {
		return nil, err
	}
	add(recommendedRow{Type: "continue", Items: items})

	if items, err = list(store.ItemQuery{Sort: "release"}); err != nil {
		return nil, err
	}
	add(recommendedRow{Type: "recently_released", Items: items})
	if items, err = list(store.ItemQuery{Sort: "added"}); err != nil {
		return nil, err
	}
	add(recommendedRow{Type: "recently_added", Items: items, Name: lib.Name})
	if lib.Kind == "shows" {
		if items, err = list(store.ItemQuery{Unstarted: true, Sort: "shuffle"}); err != nil {
			return nil, err
		}
		add(recommendedRow{Type: "start_watching", Items: items})
	}

	recent, err := s.store.RecentlyWatchedTitles(ctx, userID, lib.ID, 3)
	if err != nil {
		return nil, err
	}
	for _, t := range recent {
		similar, err := s.store.Similar(ctx, &t, acc, recommendedLimit)
		if err != nil {
			return nil, err
		}
		if items, err = unseen(similar); err != nil {
			return nil, err
		}
		add(recommendedRow{Type: "because_you_watched", Items: items, Name: t.Title})
	}

	if items, err = list(store.ItemQuery{Sort: "rating"}); err != nil {
		return nil, err
	}
	add(recommendedRow{Type: "top_rated", Items: items})
	watched := store.ItemQuery{Played: true, Sort: "watched"}
	if lib.Kind == "shows" {
		watched.Kind = "episode"
	}
	if items, err = list(watched); err != nil {
		return nil, err
	}
	add(recommendedRow{Type: "recently_watched", Items: items})
	if items, err = list(store.ItemQuery{PlayedBefore: now.Add(-rediscoverAfter).Unix(), Sort: "shuffle"}); err != nil {
		return nil, err
	}
	add(recommendedRow{Type: "rediscover", Items: items})

	favourites, err := s.store.FavouriteGenres(ctx, userID, lib.ID, 2)
	if err != nil {
		return nil, err
	}
	for _, g := range favourites {
		found, _, err := s.store.ListItems(ctx, store.ItemQuery{LibraryID: lib.ID, Kind: kind, Genre: g, Sort: "shuffle",
			Unwatched: true, UserID: userID, Access: acc, Limit: recommendedLimit})
		if err != nil {
			return nil, err
		}
		if items, err = unseen(found); err != nil {
			return nil, err
		}
		add(recommendedRow{Type: "more_in_genre", Items: items, Name: g, Genre: g})
	}

	// A genre with enough titles that isn't a favourite above, a different one each day.
	genres, err := s.store.LibraryGenres(ctx, lib.ID, kind, 5, acc)
	if err != nil {
		return nil, err
	}
	genres = slices.DeleteFunc(genres, func(g string) bool { return slices.Contains(favourites, g) })
	if len(genres) > 0 {
		g := genres[day%len(genres)]
		if items, err = list(store.ItemQuery{Genre: g, Sort: "rating"}); err != nil {
			return nil, err
		}
		add(recommendedRow{Type: "top_in_genre", Items: items, Name: g, Genre: g})
	}

	// An actor or director with a few titles here: one credited on what the profile watched
	// lately when there is one, changing daily.
	people, err := s.store.FeaturedPeople(ctx, userID, lib.ID, kind, 3, acc, 10)
	if err != nil {
		return nil, err
	}
	pool := people
	if len(people) > 0 && people[0].LastWatchedAt > 0 {
		pool = slices.DeleteFunc(slices.Clone(people), func(p store.FeaturedPerson) bool { return p.LastWatchedAt == 0 })
		pool = pool[:min(3, len(pool))]
	}
	if len(pool) > 0 {
		p := pool[day%len(pool)]
		if items, err = list(store.ItemQuery{PersonID: p.ID, PersonRole: p.Role, Sort: "rating"}); err != nil {
			return nil, err
		}
		add(recommendedRow{Type: "top_by_person", Items: items, Name: p.Name, PersonID: p.ID, Role: p.Role})
	}

	top, _, err := s.store.ListItems(ctx, store.ItemQuery{LibraryID: lib.ID, Kind: kind, Sort: "rating", Unwatched: true,
		UserID: userID, Access: acc, Limit: recommendedLimit})
	if err != nil {
		return nil, err
	}
	if items, err = unseen(top); err != nil {
		return nil, err
	}
	add(recommendedRow{Type: "top_unwatched", Items: items})
	return rows, nil
}
