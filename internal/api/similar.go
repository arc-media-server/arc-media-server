package api

import (
	"net/http"

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

type recommendedRow struct {
	Title string    `json:"title"`
	Items []itemDTO `json:"items"`
	// A library page showing more, when there is one (e.g. a genre).
	Genre string `json:"genre,omitempty"`
}

// recommended: GET /api/libraries/{id}/recommended is a library's Recommended view for the
// profile, as Plex's: "Because you watched" rows for its recent titles, the best rated it hasn't
// seen, and its favourite genres. Rows without anything are left out.
func (s *Server) recommended(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	userID := currentUserID(r)
	acc := access(r)
	kind := "movie"
	if lib.Kind == "shows" {
		kind = "series"
	}
	rows := []recommendedRow{}
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
	add := func(row recommendedRow) {
		if len(row.Items) > 0 {
			rows = append(rows, row)
		}
	}

	recent, err := s.store.RecentlyWatchedTitles(ctx, userID, lib.ID, 3)
	if err != nil {
		s.fail(w, err)
		return
	}
	for _, t := range recent {
		similar, err := s.store.Similar(ctx, &t, acc, 20)
		if err != nil {
			s.fail(w, err)
			return
		}
		items, err := unseen(similar)
		if err != nil {
			s.fail(w, err)
			return
		}
		add(recommendedRow{Title: "Because you watched " + t.Title, Items: items})
	}

	top, _, err := s.store.ListItems(ctx, store.ItemQuery{LibraryID: lib.ID, Kind: kind, Sort: "rating", Unwatched: true,
		UserID: userID, Access: acc, Limit: 20})
	if err != nil {
		s.fail(w, err)
		return
	}
	items, err := unseen(top)
	if err != nil {
		s.fail(w, err)
		return
	}
	add(recommendedRow{Title: "Top rated you haven't seen", Items: items})

	genres, err := s.store.FavouriteGenres(ctx, userID, lib.ID, 2)
	if err != nil {
		s.fail(w, err)
		return
	}
	for _, g := range genres {
		list, _, err := s.store.ListItems(ctx, store.ItemQuery{LibraryID: lib.ID, Kind: kind, Genre: g, Sort: "shuffle",
			Unwatched: true, UserID: userID, Access: acc, Limit: 20})
		if err != nil {
			s.fail(w, err)
			return
		}
		items, err := unseen(list)
		if err != nil {
			s.fail(w, err)
			return
		}
		add(recommendedRow{Title: "More " + g, Items: items, Genre: g})
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}
