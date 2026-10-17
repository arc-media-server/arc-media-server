package api

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strconv"
	"time"

	"ams/internal/insights"
	"ams/internal/store"
)

// Insights: watch history and stats for the admin, and each profile's year in review.

// statsContext is what the stats pages need to show ids: the titles (with artwork), profiles and
// libraries they mention.
type statsContext struct {
	Items     map[string]itemDTO `json:"items"`
	Profiles  []profileDTO       `json:"profiles"`
	Libraries []libraryName      `json:"libraries"`
}

type libraryName struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func (s *Server) statsContext(ctx context.Context, itemIDs []string) (statsContext, error) {
	out := statsContext{Items: map[string]itemDTO{}, Profiles: []profileDTO{}, Libraries: []libraryName{}}
	items, err := s.store.ItemsByID(ctx, itemIDs)
	if err != nil {
		return out, err
	}
	for id, it := range items {
		out.Items[id] = toDTO(it)
	}
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return out, err
	}
	for _, u := range users {
		out.Profiles = append(out.Profiles, toProfile(u))
	}
	libs, err := s.store.ListLibraries(ctx)
	if err != nil {
		return out, err
	}
	for _, l := range libs {
		out.Libraries = append(out.Libraries, libraryName{ID: l.ID, Name: l.Name, Kind: l.Kind})
	}
	return out, nil
}

// playFilter reads ?profile=&library= from a request.
func playFilter(r *http.Request) store.PlayFilter {
	lib, _ := strconv.ParseInt(r.URL.Query().Get("library"), 10, 64)
	return store.PlayFilter{UserID: r.URL.Query().Get("profile"), LibraryID: lib}
}

// statsOverview: GET /api/stats?days=30&profile=&library= is watch activity over the last days
// (0 or none for all time): totals, watch time over time, the most watched titles and genres,
// profiles, libraries, devices and the busiest times.
func (s *Server) statsOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := playFilter(r)
	now := time.Now()
	var from time.Time
	if days, _ := strconv.Atoi(r.URL.Query().Get("days")); days > 0 {
		from = time.Date(now.Year(), now.Month(), now.Day()-days+1, 0, 0, 0, 0, now.Location())
		f.From = from.Unix()
	}
	plays, err := s.store.Plays(ctx, f)
	if err != nil {
		s.fail(w, err)
		return
	}
	ids := titleIDs(plays)
	genres, err := s.store.GenresFor(ctx, ids)
	if err != nil {
		s.fail(w, err)
		return
	}
	o := insights.Summarise(plays, genres, from, now, 10)
	var shown []string
	for _, t := range append(slices.Clone(o.Movies), o.Shows...) {
		shown = append(shown, t.ID)
	}
	sc, err := s.statsContext(ctx, shown)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		insights.Overview
		statsContext
	}{o, sc})
}

func titleIDs(plays []store.Play) []string {
	seen := map[string]bool{}
	var ids []string
	for _, p := range plays {
		if id := insights.TitleID(p); !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// statsHistory: GET /api/stats/history?profile=&library=&before=&limit=50 lists viewings, newest
// first. Pass the last one's id as before for the next page.
func (s *Server) statsHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	f := playFilter(r)
	f.BeforeID, _ = strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	f.Limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	plays, err := s.store.Plays(ctx, f)
	if err != nil {
		s.fail(w, err)
		return
	}
	var ids []string
	for _, p := range plays {
		ids = append(ids, p.ItemID)
		if p.SeriesID != "" {
			ids = append(ids, p.SeriesID)
		}
	}
	sc, err := s.statsContext(ctx, ids)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Plays []store.Play `json:"plays"`
		More  bool         `json:"more"`
		statsContext
	}{plays, len(plays) == f.Limit, sc})
}

type titleSizeDTO struct {
	ItemID     string `json:"item_id"`
	Size       int64  `json:"size"`
	Files      int    `json:"files"`
	AddedAt    int64  `json:"added_at"`
	Watched    bool   `json:"watched"`
	LastPlayed int64  `json:"last_played,omitempty"`
}

type librarySize struct {
	libraryName
	Size   int64 `json:"size"`
	Titles int   `json:"titles"`
	Files  int   `json:"files"`
	// Movies and shows no profile has watched or started, and the space they take.
	UnwatchedTitles int   `json:"unwatched_titles"`
	UnwatchedSize   int64 `json:"unwatched_size"`
}

// statsStorage: GET /api/stats/storage?library= is the space each library takes, the biggest
// titles, and titles nobody has watched (biggest first).
func (s *Server) statsStorage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lib, _ := strconv.ParseInt(r.URL.Query().Get("library"), 10, 64)
	sizes, err := s.store.TitleSizes(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	sc, err := s.statsContext(ctx, nil)
	if err != nil {
		s.fail(w, err)
		return
	}
	libs := []*librarySize{}
	byID := map[int64]*librarySize{}
	for _, l := range sc.Libraries {
		ls := &librarySize{libraryName: l}
		byID[l.ID] = ls
		libs = append(libs, ls)
	}
	biggest, unwatched := []titleSizeDTO{}, []titleSizeDTO{}
	var ids []string
	for _, t := range sizes {
		if ls := byID[t.LibraryID]; ls != nil {
			ls.Size += t.Size
			ls.Titles++
			ls.Files += t.Files
			if !t.Watched {
				ls.UnwatchedTitles++
				ls.UnwatchedSize += t.Size
			}
		}
		if lib != 0 && t.LibraryID != lib {
			continue
		}
		d := titleSizeDTO{ItemID: t.ItemID, Size: t.Size, Files: t.Files, AddedAt: t.AddedAt, Watched: t.Watched, LastPlayed: t.LastPlayed}
		added := false
		if len(biggest) < 50 {
			biggest = append(biggest, d)
			added = true
		}
		if !t.Watched && len(unwatched) < 200 {
			unwatched = append(unwatched, d)
			added = true
		}
		if added {
			ids = append(ids, t.ItemID)
		}
	}
	items, err := s.store.ItemsByID(ctx, ids)
	if err != nil {
		s.fail(w, err)
		return
	}
	for id, it := range items {
		sc.Items[id] = toDTO(it)
	}
	out := make([]librarySize, 0, len(libs))
	for _, l := range libs {
		out = append(out, *l)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"libraries": out, "biggest": biggest, "never_watched": unwatched, "items": sc.Items,
	})
}

type rankedPersonDTO struct {
	personDTO
	Titles    int   `json:"titles"`
	WatchedMS int64 `json:"watched_ms"`
}

// myYear: GET /api/me/year?year=2026 is the current profile's year in review (this year by
// default), with the years it has history for.
func (s *Server) myYear(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := currentUserID(r)
	years, err := s.store.PlayYears(ctx, userID)
	if err != nil {
		s.fail(w, err)
		return
	}
	year, _ := strconv.Atoi(r.URL.Query().Get("year"))
	if year == 0 {
		year = cmp.Or(firstOr(years), time.Now().Year())
	}
	from := time.Date(year, 1, 1, 0, 0, 0, 0, time.Local)
	plays, err := s.store.Plays(ctx, store.PlayFilter{UserID: userID, From: from.Unix(), To: from.AddDate(1, 0, 0).Unix()})
	if err != nil {
		s.fail(w, err)
		return
	}
	ids := titleIDs(plays)
	genres, err := s.store.GenresFor(ctx, ids)
	if err != nil {
		s.fail(w, err)
		return
	}
	actors, err := s.store.LeadActors(ctx, ids)
	if err != nil {
		s.fail(w, err)
		return
	}
	y := insights.Review(year, plays, genres, actors, 5)
	var shown []string
	for _, t := range append(slices.Clone(y.TopMovies), y.TopShows...) {
		shown = append(shown, t.ID)
	}
	if y.Binge != nil {
		shown = append(shown, y.Binge.SeriesID)
	}
	if y.First != nil {
		shown = append(shown, y.First.ItemID, y.First.SeriesID)
	}
	items, err := s.store.ItemsByID(ctx, shown)
	if err != nil {
		s.fail(w, err)
		return
	}
	dtos := map[string]itemDTO{}
	for id, it := range items {
		dtos[id] = toDTO(it)
	}
	people := []rankedPersonDTO{}
	for _, p := range y.People {
		people = append(people, rankedPersonDTO{personDTO: toPersonDTO(p.Person), Titles: p.Titles, WatchedMS: p.WatchedMS})
	}
	if !slices.Contains(years, time.Now().Year()) {
		years = append([]int{time.Now().Year()}, years...)
	}
	writeJSON(w, http.StatusOK, struct {
		insights.Year
		People []rankedPersonDTO  `json:"people"`
		Years  []int              `json:"years"`
		Items  map[string]itemDTO `json:"items"`
	}{y, people, years, dtos})
}

func firstOr(list []int) int {
	if len(list) == 0 {
		return 0
	}
	return list[0]
}
