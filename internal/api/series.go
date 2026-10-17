package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"ams/internal/store"
)

// seriesFromPath loads the series named in the URL and its library.
func (s *Server) seriesFromPath(w http.ResponseWriter, r *http.Request) (*store.Item, *store.Library, bool) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return nil, nil, false
	}
	if it.Kind != "series" {
		writeError(w, http.StatusBadRequest, "item is not a series")
		return nil, nil, false
	}
	lib, err := s.store.GetLibrary(r.Context(), it.LibraryID)
	if err != nil {
		s.fail(w, err)
		return nil, nil, false
	}
	return it, lib, true
}

// orderingFromPath loads the ordering named by {oid}, checking it belongs to the series.
func (s *Server) orderingFromPath(w http.ResponseWriter, r *http.Request, seriesID string) (*store.Ordering, bool) {
	oid, err := strconv.ParseInt(r.PathValue("oid"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ordering id")
		return nil, false
	}
	o, err := s.store.GetOrdering(r.Context(), oid)
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	if o == nil || o.SeriesID != seriesID {
		writeError(w, http.StatusNotFound, "ordering not found")
		return nil, false
	}
	return o, true
}

// listOrderings: GET /api/series/{id}/orderings
func (s *Server) listOrderings(w http.ResponseWriter, r *http.Request) {
	series, lib, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	orderings, err := s.store.ListOrderings(r.Context(), series.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	display, err := s.store.DisplayOrdering(r.Context(), series.ID, currentUserID(r), lib.SourceOrdering)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"display_ordering_id": display,
		"source_ordering":     lib.SourceOrdering,
		"orderings":           orderings,
	})
}

// setDisplayOrdering: PUT /api/series/{id}/display-ordering {"ordering_id": 12} (null resets).
// The choice is the current profile's own; the admin's choice is also the default for profiles
// that haven't chosen. Choosing an ordering from the other provider triggers a thorough episode
// mapping first.
func (s *Server) setDisplayOrdering(w http.ResponseWriter, r *http.Request) {
	series, lib, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		OrderingID *int64 `json:"ordering_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	user := currentUser(r)
	users := []string{user.ID}
	if user.IsAdmin {
		users = append(users, "")
	}
	if body.OrderingID == nil {
		for _, u := range users {
			if err := s.store.ClearDisplayOrdering(r.Context(), series.ID, u); err != nil {
				s.fail(w, err)
				return
			}
		}
		s.listOrderings(w, r)
		return
	}
	o, err := s.store.GetOrdering(r.Context(), *body.OrderingID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if o == nil || o.SeriesID != series.ID {
		writeError(w, http.StatusBadRequest, "ordering does not belong to this series")
		return
	}
	for _, u := range users {
		if err := s.store.SetDisplayOrdering(r.Context(), series.ID, u, o.ID); err != nil {
			s.fail(w, err)
			return
		}
	}
	if o.Source != "custom" && !strings.HasPrefix(o.Source, sourceProvider(lib.SourceOrdering)) {
		if err := s.scanner.SyncSeriesMetadata(r.Context(), lib, series.ID, true); err != nil {
			s.log.Warn("deep episode mapping failed", "series", series.Title, "err", err)
		}
	}
	s.listOrderings(w, r)
}

func sourceProvider(source string) string {
	if strings.HasPrefix(source, "tmdb") {
		return "tmdb"
	}
	return "tvdb"
}

type seasonEpisode struct {
	itemDTO
	SeasonNumber  int  `json:"season_number"`
	EpisodeNumber int  `json:"episode_number"`
	Available     bool `json:"available"`
	Locked        bool `json:"locked,omitempty"`
}

type seasonDTO struct {
	SeasonNumber int             `json:"season_number"`
	Name         string          `json:"name"`
	Episodes     []seasonEpisode `json:"episodes"`
}

// seasons: GET /api/series/{id}/seasons?ordering=<id>&missing=1
// Arranges the series' episodes by an ordering (the display ordering by default). Episodes you
// have that the ordering doesn't place are returned in "unplaced" rather than hidden.
func (s *Server) seasons(w http.ResponseWriter, r *http.Request) {
	series, lib, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	q := r.URL.Query()
	includeMissing := q.Get("missing") == "1" || q.Get("missing") == "true"

	oid, _ := strconv.ParseInt(q.Get("ordering"), 10, 64)
	if oid == 0 {
		var err error
		if oid, err = s.store.DisplayOrdering(ctx, series.ID, currentUserID(r), lib.SourceOrdering); err != nil {
			s.fail(w, err)
			return
		}
	}
	var ordering *store.Ordering
	if oid != 0 {
		var err error
		if ordering, err = s.store.GetOrdering(ctx, oid); err != nil {
			s.fail(w, err)
			return
		}
		if ordering == nil || ordering.SeriesID != series.ID {
			writeError(w, http.StatusNotFound, "ordering not found")
			return
		}
	}

	episodes, err := s.store.Children(ctx, series.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	available, err := s.store.AvailableChildren(ctx, series.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	byID := make(map[string]store.Item, len(episodes))
	for _, e := range episodes {
		byID[e.ID] = e
	}

	seasons := []*seasonDTO{}
	placed := map[string]bool{}
	if ordering != nil {
		entries, err := s.store.OrderingEntries(ctx, ordering.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		names, err := s.store.OrderingSeasons(ctx, ordering.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		bySeason := map[int]*seasonDTO{}
		for _, e := range entries { // sorted by season, episode
			ep, ok := byID[e.EpisodeID]
			// An episode can sit at several positions (e.g. a two-part pilot the file has as one);
			// show it once, at its first position.
			if !ok || placed[e.EpisodeID] {
				continue
			}
			placed[e.EpisodeID] = true
			if !available[e.EpisodeID] && !includeMissing {
				continue
			}
			sd := bySeason[e.Season]
			if sd == nil {
				sd = &seasonDTO{SeasonNumber: e.Season, Name: names[e.Season]}
				if sd.Name == "" {
					sd.Name = defaultSeasonName(e.Season)
				}
				bySeason[e.Season] = sd
				seasons = append(seasons, sd)
			}
			sd.Episodes = append(sd.Episodes, seasonEpisode{itemDTO: toDTO(ep), SeasonNumber: e.Season,
				EpisodeNumber: e.Episode, Available: available[e.EpisodeID], Locked: e.Locked})
		}
		// Specials last, like every other media app.
		if len(seasons) > 1 && seasons[0].SeasonNumber == 0 {
			seasons = append(seasons[1:], seasons[0])
		}
	}

	unplaced := []seasonEpisode{}
	for _, e := range episodes {
		if !placed[e.ID] && available[e.ID] {
			unplaced = append(unplaced, seasonEpisode{itemDTO: toDTO(e), Available: true})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"series":   toDTO(*series),
		"ordering": ordering,
		"seasons":  seasons,
		"unplaced": unplaced,
	})
}

func defaultSeasonName(n int) string {
	if n == 0 {
		return "Specials"
	}
	return "Season " + strconv.Itoa(n)
}

// createOrdering: POST /api/series/{id}/orderings {"name": "...", "copy_from": <ordering id>}
// creates a custom ordering starting as a copy of another.
func (s *Server) createOrdering(w http.ResponseWriter, r *http.Request) {
	series, _, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Name     string `json:"name"`
		CopyFrom int64  `json:"copy_from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	src, err := s.store.GetOrdering(r.Context(), body.CopyFrom)
	if err != nil {
		s.fail(w, err)
		return
	}
	if src == nil || src.SeriesID != series.ID {
		writeError(w, http.StatusBadRequest, "copy_from must be an ordering of this series")
		return
	}
	id, err := s.store.CopyOrdering(r.Context(), src, body.Name)
	if err != nil {
		s.fail(w, err)
		return
	}
	o, err := s.store.GetOrdering(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

// deleteOrdering: DELETE /api/series/{id}/orderings/{oid} (custom orderings only)
func (s *Server) deleteOrdering(w http.ResponseWriter, r *http.Request) {
	series, _, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	o, ok := s.orderingFromPath(w, r, series.ID)
	if !ok {
		return
	}
	if o.Source != "custom" {
		writeError(w, http.StatusBadRequest, "only custom orderings can be deleted")
		return
	}
	if err := s.store.DeleteOrdering(r.Context(), o.ID); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setEntry: PUT /api/series/{id}/orderings/{oid}/entries/{eid} {"season_number": 1, "episode_number": 5}
// Manually positions an episode. Manual positions survive metadata refreshes.
func (s *Server) setEntry(w http.ResponseWriter, r *http.Request) {
	series, _, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	o, ok := s.orderingFromPath(w, r, series.ID)
	if !ok {
		return
	}
	ep, err := s.store.GetItem(r.Context(), r.PathValue("eid"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if ep == nil || ep.ParentID != series.ID {
		writeError(w, http.StatusNotFound, "episode not found in this series")
		return
	}
	var body struct {
		Season  *int `json:"season_number"`
		Episode *int `json:"episode_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Season == nil || body.Episode == nil {
		writeError(w, http.StatusBadRequest, "season_number and episode_number are required")
		return
	}
	if *body.Season < 0 || *body.Episode < 1 {
		writeError(w, http.StatusBadRequest, "season_number must be 0 or more and episode_number 1 or more")
		return
	}
	if err := s.store.SetEntry(r.Context(), o.ID, ep.ID, *body.Season, *body.Episode); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// clearEntry: DELETE /api/series/{id}/orderings/{oid}/entries/{eid}
// Removes a manual position; provider orderings restore their own on the next refresh.
func (s *Server) clearEntry(w http.ResponseWriter, r *http.Request) {
	series, _, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	o, ok := s.orderingFromPath(w, r, series.ID)
	if !ok {
		return
	}
	if err := s.store.ClearEntry(r.Context(), o.ID, r.PathValue("eid")); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// refreshSeries: POST /api/series/{id}/refresh?deep=1 re-fetches metadata and orderings now.
func (s *Server) refreshSeries(w http.ResponseWriter, r *http.Request) {
	series, lib, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	deep := r.URL.Query().Get("deep") == "1"
	if err := s.scanner.SyncSeriesMetadata(r.Context(), lib, series.ID, deep); err != nil {
		s.log.Warn("series refresh failed", "series", series.Title, "err", err)
		writeError(w, http.StatusBadGateway, "refresh failed: "+err.Error())
		return
	}
	// Re-link files so episodes known only from file names get their positions back.
	s.scanner.Start(lib.ID)
	s.listOrderings(w, r)
}

// unmatched: GET /api/libraries/{id}/unmatched lists items that need attention.
func (s *Server) unmatched(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	items, err := s.store.Unmatched(r.Context(), lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]itemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, toDTO(it))
	}
	writeJSON(w, http.StatusOK, out)
}
