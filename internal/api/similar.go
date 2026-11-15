package api

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
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
	ID       string `json:"id"`
	ItemsURL string `json:"items_url"`
	HasMore  bool   `json:"has_more"`
	sourceID string
	query    *store.ItemQuery

	Style string `json:"style,omitempty"`
	// What the row is: continue, recently_released, recently_added, start_watching,
	// because_you_watched, top_rated, recently_watched, rediscover, more_in_genre, top_in_genre,
	// top_by_person or top_unwatched.
	Type string `json:"type"`
	// An English title, kept for apps written against v1 before rows had a type. Apps should title
	// rows themselves from type and the fields below, in the viewer's language.
	Title string    `json:"title"`
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

// recommendedResponse is a library's Recommended view.
type recommendedResponse struct {
	Rows []recommendedRow `json:"rows"`
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
// person_id, role) for the app to title in the viewer's language, plus an English title for apps
// written before rows had a type; rows without anything are left out.
func (s *Server) recommended(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	started := time.Now()
	layout, err := s.readRecommendedLayout(r, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	rows, err := s.recommendedRowsWithLayout(r, lib, recommendedLimit, "", layout)
	if err != nil {
		s.fail(w, err)
		return
	}
	rows = arrangeRecommended(rows, layout)
	// Per request, so detail only (recorded while detailed logging is on).
	s.log.Debug("recommended rows", "library", lib.Name, "rows", len(rows), "took", time.Since(started).Round(time.Millisecond))
	writeJSON(w, http.StatusOK, recommendedResponse{Rows: rows})
}

func (s *Server) recommendedRows(r *http.Request, lib *store.Library) ([]recommendedRow, error) {
	return s.recommendedRowsLimit(r, lib, recommendedLimit)
}

func (s *Server) recommendedRowsLimit(r *http.Request, lib *store.Library, limit int) ([]recommendedRow, error) {
	return s.recommendedRowsLimitTarget(r, lib, limit, "")
}

func recommendedRowID(row recommendedRow) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d/%s/%s", row.Type, row.Name, row.Genre, row.PersonID, row.Role, row.sourceID)))
	return fmt.Sprintf("%x", sum[:12])
}

func (s *Server) recommendedRowsLimitTarget(r *http.Request, lib *store.Library, limit int, targetID string) ([]recommendedRow, error) {
	layout, err := s.readRecommendedLayout(r, lib.ID)
	if err != nil {
		return nil, err
	}
	return s.recommendedRowsWithLayout(r, lib, limit, targetID, layout)
}

func (s *Server) recommendedRowsWithLayout(r *http.Request, lib *store.Library, limit int, targetID string, layout []recommendedLook) ([]recommendedRow, error) {
	hidden := map[string]bool{}
	for _, look := range layout {
		if _, seen := hidden[look.Type]; !seen {
			hidden[look.Type] = look.Hidden
		}
	}
	data := s.itemData(r)
	contexts, posters := s.episodeContexts(r.Context()), seasonPosterCache{}
	home := func(entries []homeEntry) ([]itemDTO, error) {
		return s.homeDTOsWith(r, entries, contexts, posters, data.fill)
	}
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
	// With detailed logging on, how long each row (or lookup) took, to find slow ones on a big
	// library. Only the full view is timed; one row's continuation is a single step.
	timed := targetID == "" && s.log != nil && s.log.Enabled(ctx, slog.LevelDebug)
	var steps []any
	mark := now
	step := func(name string) {
		if timed {
			at := time.Now()
			steps = append(steps, name, at.Sub(mark).Round(time.Millisecond))
			mark = at
		}
	}
	// The profile's history in this library, read once for the rows that start from it
	// (Because You Watched, favourite genres, featured people) when the first of them needs it.
	var history *store.WatchHistory
	watchHistory := func() (*store.WatchHistory, error) {
		if history == nil {
			h, err := s.store.LibraryWatchHistory(ctx, userID, lib.ID)
			if err != nil {
				return nil, err
			}
			history = h
			step("watch_history")
		}
		return history, nil
	}
	defer func() {
		if timed {
			s.log.Debug("recommended row timings", append([]any{"library", lib.Name}, steps...)...)
		}
	}()
	wanted := func(row recommendedRow) bool {
		return !hidden[row.Type] && (targetID == "" || targetID == recommendedRowID(row))
	}
	var nextQuery *store.ItemQuery
	var nextMore bool
	add := func(row recommendedRow) {
		row.query = nextQuery
		hasMore := nextMore
		nextQuery, nextMore = nil, false
		if len(row.Items) > 0 {
			row.ID = recommendedRowID(row)
			row.ItemsURL = fmt.Sprintf("/api/v1/libraries/%d/recommended/rows/%s", lib.ID, row.ID)
			row.HasMore = hasMore || (row.query == nil && (len(row.Items) >= limit || row.Type == "because_you_watched"))
			row.Title = englishRowTitle(row, kind == "series")
			rows = append(rows, row)
		}
	}
	// list lists the library's titles (or episodes) with the profile's watch state.
	list := func(q store.ItemQuery, rowType string) ([]itemDTO, error) {
		q.LibraryID, q.UserID, q.Access, q.Limit = lib.ID, userID, acc, limit
		if q.Kind == "" {
			q.Kind = kind
		}
		queryStart := time.Time{}
		if timed {
			queryStart = time.Now()
		}
		items, more, err := s.store.ListItemsWindow(ctx, q)
		if timed {
			steps = append(steps, rowType+"_query", time.Since(queryStart).Round(time.Millisecond))
		}
		nextQuery, nextMore = &q, more
		detailStart := time.Time{}
		if timed {
			detailStart = time.Now()
		}
		defer func() {
			if timed {
				steps = append(steps, rowType+"_details", time.Since(detailStart).Round(time.Millisecond))
			}
		}()
		if err != nil {
			return nil, err
		}
		if q.Kind == "episode" {
			entries := make([]homeEntry, len(items))
			for i, it := range items {
				entries[i] = homeEntry{item: it}
			}
			return home(entries)
		}
		dtos := toDTOs(items)
		return dtos, data.fill(ptrs(dtos))
	}
	// unseen keeps the titles the profile hasn't finished, with their watch state.
	unseen := func(items []store.Item) ([]itemDTO, error) {
		dtos := toDTOs(items)
		if err := data.fill(ptrs(dtos)); err != nil {
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

	// A continuation request evaluates only its current row. Row descriptors,
	// selection windows and daily identities remain the same as the full view.
	queryRow := func(row recommendedRow, q store.ItemQuery) (bool, error) {
		if !wanted(row) {
			return false, nil
		}
		items, err := list(q, row.Type)
		if err != nil {
			return false, err
		}
		row.Items = items
		add(row)
		step(row.Type)
		return targetID != "", nil
	}
	continueRow := recommendedRow{Type: "continue"}
	if wanted(continueRow) {
		here := acc
		here.Libraries = []int64{lib.ID}
		inProgress, err := s.inProgress(r, here)
		if err != nil {
			return nil, err
		}
		var next []homeEntry
		if lib.Kind == "shows" {
			if next, err = s.upNextWithContexts(r, here, contexts); err != nil {
				return nil, err
			}
		}
		items, err := home(mergeHome(inProgress, next, limit))
		if err != nil {
			return nil, err
		}
		continueRow.Items = items
		add(continueRow)
		step("continue")
		if targetID != "" {
			return rows, nil
		}
	}
	for _, entry := range []struct {
		row   recommendedRow
		query store.ItemQuery
	}{
		{recommendedRow{Type: "recently_released"}, store.ItemQuery{Sort: "release"}},
		{recommendedRow{Type: "recently_added", Name: lib.Name}, store.ItemQuery{Sort: "added"}},
	} {
		if stop, err := queryRow(entry.row, entry.query); stop || err != nil {
			return rows, err
		}
	}
	if lib.Kind == "shows" {
		if stop, err := queryRow(recommendedRow{Type: "start_watching"}, store.ItemQuery{Unstarted: true, Sort: "shuffle"}); stop || err != nil {
			return rows, err
		}
	}
	if !hidden["because_you_watched"] {
		h, err := watchHistory()
		if err != nil {
			return nil, err
		}
		recent, err := s.store.RecentlyWatchedFrom(ctx, h, 2)
		if err != nil {
			return nil, err
		}
		step("recently_watched_titles")
		for _, source := range recent {
			row := recommendedRow{Type: "because_you_watched", Name: source.Title, sourceID: source.ID}
			if !wanted(row) {
				continue
			}
			visible, err := s.store.CanSee(ctx, &source, acc)
			if err != nil {
				return nil, err
			}
			if !visible {
				if targetID != "" {
					return rows, nil
				}
				continue
			}
			similar, err := s.store.Similar(ctx, &source, acc, limit)
			if err != nil {
				return nil, err
			}
			row.Items, err = unseen(similar)
			if err != nil {
				return nil, err
			}
			add(row)
			step(row.Type)
			if targetID != "" {
				return rows, nil
			}
		}
	}
	watched := store.ItemQuery{Played: true, Sort: "watched"}
	if lib.Kind == "shows" {
		watched.Kind = "episode"
	}
	for _, entry := range []struct {
		row   recommendedRow
		query store.ItemQuery
	}{
		{recommendedRow{Type: "top_rated"}, store.ItemQuery{Sort: "rating"}},
		{recommendedRow{Type: "recently_watched"}, watched},
		{recommendedRow{Type: "rediscover"}, store.ItemQuery{PlayedBefore: now.Add(-rediscoverAfter).Unix(), Sort: "shuffle"}},
	} {
		if stop, err := queryRow(entry.row, entry.query); stop || err != nil {
			return rows, err
		}
	}
	if !hidden["more_in_genre"] || !hidden["top_in_genre"] {
		h, err := watchHistory()
		if err != nil {
			return nil, err
		}
		favourites, err := s.store.FavouriteGenresFrom(ctx, h, 1)
		if err != nil {
			return nil, err
		}
		step("favourite_genres")
		for _, genre := range favourites {
			row := recommendedRow{Type: "more_in_genre", Name: genre, Genre: genre}
			if stop, err := queryRow(row, store.ItemQuery{Genre: genre, Sort: "shuffle", Unwatched: true}); stop || err != nil {
				return rows, err
			}
		}
		if !hidden["top_in_genre"] {
			genres, err := s.store.LibraryGenres(ctx, lib.ID, kind, 5, acc)
			if err != nil {
				return nil, err
			}
			step("library_genres")
			genres = slices.DeleteFunc(genres, func(g string) bool { return slices.Contains(favourites, g) })
			if len(genres) > 0 {
				genre := genres[day%len(genres)]
				row := recommendedRow{Type: "top_in_genre", Name: genre, Genre: genre}
				if stop, err := queryRow(row, store.ItemQuery{Genre: genre, Sort: "rating"}); stop || err != nil {
					return rows, err
				}
			}
		}
	}
	if !hidden["top_by_person"] {
		h, err := watchHistory()
		if err != nil {
			return nil, err
		}
		people, err := s.store.FeaturedPeopleFrom(ctx, h, lib.ID, kind, 3, acc, 10)
		if err != nil {
			return nil, err
		}
		step("featured_people")
		pool := people
		if len(people) > 0 && people[0].LastWatchedAt > 0 {
			pool = slices.DeleteFunc(slices.Clone(people), func(p store.FeaturedPerson) bool { return p.LastWatchedAt == 0 })
			pool = pool[:min(3, len(pool))]
		}
		if len(pool) > 0 {
			person := pool[day%len(pool)]
			row := recommendedRow{Type: "top_by_person", Name: person.Name, PersonID: person.ID, Role: person.Role}
			if stop, err := queryRow(row, store.ItemQuery{PersonID: person.ID, PersonRole: person.Role, Sort: "rating"}); stop || err != nil {
				return rows, err
			}
		}
	}
	if _, err := queryRow(recommendedRow{Type: "top_unwatched"}, store.ItemQuery{Sort: "rating", Unwatched: true}); err != nil {
		return nil, err
	}
	return rows, nil
}

// englishRowTitle is a row's title in English, as the web app's English catalog words it.
func englishRowTitle(r recommendedRow, shows bool) string {
	what := "Movies"
	if shows {
		what = "Shows"
	}
	switch r.Type {
	case "continue":
		return "Continue Watching"
	case "recently_released":
		return "Recently Released"
	case "recently_added":
		return "Recently Added in " + r.Name
	case "start_watching":
		return "Start Watching"
	case "because_you_watched":
		return "Because you watched " + r.Name
	case "top_rated":
		return "Top Rated " + what
	case "recently_watched":
		if shows {
			return "Recently Watched Episodes"
		}
		return "Recently Watched"
	case "rediscover":
		return "Rediscover"
	case "more_in_genre":
		return "More in " + r.Genre
	case "top_in_genre":
		return "Top " + what + " in " + r.Genre
	case "top_by_person":
		if r.Role == "director" {
			return "Top " + what + " by " + r.Name
		}
		return "Top " + what + " with " + r.Name
	case "top_unwatched":
		return "Top Unwatched"
	}
	return ""
}
