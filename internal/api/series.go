package api

import (
	"cmp"
	"context"
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
	display, err := s.store.DisplayOrdering(r.Context(), series.ID, currentUserID(r), cmp.Or(series.SourceOrdering, lib.SourceOrdering))
	if err != nil {
		s.fail(w, err)
		return
	}
	inline, overridden, err := s.store.InlineSpecials(r.Context(), series.ID, currentUserID(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"display_ordering_id": display,
		"source_ordering":     cmp.Or(series.SourceOrdering, lib.SourceOrdering),
		"orderings":           orderings,
		"inline_specials":     inline,
		"specials_overridden": overridden,
	})
}

// setSpecials: PATCH /api/series/{id}/specials {"inline_specials": true|false|null}.
// null restores inheritance (or Off for the admin's server default). This changes display/
// playback traversal only; manual/custom placements and original numbering remain authoritative.
func (s *Server) setSpecials(w http.ResponseWriter, r *http.Request) {
	series, _, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Inline json.RawMessage `json:"inline_specials"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Inline) == 0 {
		writeError(w, 400, "inline_specials must be true, false or null")
		return
	}
	var enabled *bool
	if err := json.Unmarshal(body.Inline, &enabled); err != nil {
		writeError(w, 400, "inline_specials must be true, false or null")
		return
	}
	if err := s.store.SetInlineSpecials(r.Context(), series.ID, currentUserID(r), currentUser(r).IsAdmin, enabled); err != nil {
		s.fail(w, err)
		return
	}
	s.listOrderings(w, r)
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
	if o.Source != "custom" && !strings.HasPrefix(o.Source, sourceProvider(cmp.Or(series.SourceOrdering, lib.SourceOrdering))) {
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
	InlineSpecial bool `json:"inline_special,omitempty"`
}

type seasonDTO struct {
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview,omitempty"`
	// Season fields edited by hand ("name", "overview").
	LockedFields []string `json:"locked_fields"`
	// The season's poster, when the order is one season posters belong to.
	Poster   string          `json:"poster,omitempty"`
	Episodes []seasonEpisode `json:"episodes"`
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
		if oid, err = s.store.DisplayOrdering(ctx, series.ID, currentUserID(r), cmp.Or(series.SourceOrdering, lib.SourceOrdering)); err != nil {
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
		entries, err := s.store.DisplayEntries(ctx, ordering.ID, currentUserID(r))
		if err != nil {
			s.fail(w, err)
			return
		}
		details, err := s.store.SeasonDetails(ctx, ordering.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		var posters map[int]store.SeasonArt
		if ordering.Source == "tmdb_aired" || ordering.Source == "tvdb_aired" {
			if posters, err = s.store.SeasonPosters(ctx, series.ID); err != nil {
				s.fail(w, err)
				return
			}
			// The two aired orders number seasons alike, so a season without a summary of its own
			// (TheTVDB's have none) uses the other's.
			if other, err := s.otherAiredSeasons(ctx, series.ID, ordering.Source); err == nil {
				for n, o := range other {
					if d, ok := details[n]; ok && d.Overview == "" {
						d.Overview = o.Overview
						details[n] = d
					}
				}
			}
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
			sd := bySeason[e.GroupSeason]
			if sd == nil {
				d := details[e.GroupSeason]
				sd = &seasonDTO{SeasonNumber: e.GroupSeason, Name: d.Name, Overview: d.Overview, LockedFields: d.LockedFields,
					Poster: seasonPosterURL(series.ID, e.GroupSeason, posters[e.GroupSeason])}
				if sd.LockedFields == nil {
					sd.LockedFields = []string{}
				}
				if sd.Name == "" {
					sd.Name = defaultSeasonName(e.GroupSeason)
				}
				bySeason[e.GroupSeason] = sd
				seasons = append(seasons, sd)
			}
			sd.Episodes = append(sd.Episodes, seasonEpisode{itemDTO: toDTO(ep), SeasonNumber: e.Season,
				EpisodeNumber: e.Episode, Available: available[e.EpisodeID], Locked: e.Locked, InlineSpecial: e.InlineSpecial})
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

	seriesDTO := toDTO(*series)
	if err := s.addDetails(r, &seriesDTO); err != nil {
		s.fail(w, err)
		return
	}
	all := []*itemDTO{&seriesDTO}
	for _, sd := range seasons {
		for i := range sd.Episodes {
			all = append(all, &sd.Episodes[i].itemDTO)
		}
	}
	for i := range unplaced {
		all = append(all, &unplaced[i].itemDTO)
	}
	if err := s.fillUserData(r, all); err != nil {
		s.fail(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"series":   seriesDTO,
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
	task := s.activity.Start("refresh", "Refreshing "+series.Title)
	err := s.scanner.SyncSeriesMetadata(r.Context(), lib, series.ID, deep)
	task.Finish(err)
	if err != nil {
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

// editSeason: PATCH /api/series/{id}/seasons/{n}?ordering=<id> {"name", "overview", "locked_fields"}
// changes a season's name and summary in an episode order (the profile's display order by default).
// Fields sent are locked so refreshes keep them; "locked_fields" replaces the locked set.
func (s *Server) editSeason(w http.ResponseWriter, r *http.Request) {
	series, lib, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 0 {
		writeError(w, http.StatusBadRequest, "invalid season number")
		return
	}
	oid, _ := strconv.ParseInt(r.URL.Query().Get("ordering"), 10, 64)
	if oid == 0 {
		if oid, err = s.store.DisplayOrdering(ctx, series.ID, currentUserID(r), cmp.Or(series.SourceOrdering, lib.SourceOrdering)); err != nil {
			s.fail(w, err)
			return
		}
	}
	if o, err := s.store.GetOrdering(ctx, oid); err != nil {
		s.fail(w, err)
		return
	} else if o == nil || o.SeriesID != series.ID {
		writeError(w, http.StatusNotFound, "ordering not found")
		return
	}
	var body struct {
		Name         *string   `json:"name"`
		Overview     *string   `json:"overview"`
		LockedFields *[]string `json:"locked_fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name != nil {
		v := strings.TrimSpace(*body.Name)
		if v == "" {
			writeError(w, http.StatusBadRequest, "a season needs a name")
			return
		}
		body.Name = &v
	}
	if body.Overview != nil {
		v := strings.TrimSpace(*body.Overview)
		body.Overview = &v
	}
	var locked []string
	if body.LockedFields != nil {
		locked = append([]string{}, *body.LockedFields...)
	}
	if err := s.store.EditSeason(ctx, oid, n, body.Name, body.Overview, locked); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Unlocked fields go back to the provider's on the next refresh; do it now.
	if body.LockedFields != nil {
		if err := s.scanner.SyncSeriesMetadata(ctx, lib, series.ID, false); err != nil {
			s.log.Warn("refreshing series after a season edit failed", "series", series.Title, "err", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// otherAiredSeasons returns the seasons of the show's other provider's aired order (TMDB's for
// TheTVDB's and the other way round), or none.
func (s *Server) otherAiredSeasons(ctx context.Context, seriesID, source string) (map[int]store.Season, error) {
	other := "tmdb_aired"
	if source == "tmdb_aired" {
		other = "tvdb_aired"
	}
	orderings, err := s.store.ListOrderings(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	for _, o := range orderings {
		if o.Source == other {
			return s.store.SeasonDetails(ctx, o.ID)
		}
	}
	return nil, nil
}
