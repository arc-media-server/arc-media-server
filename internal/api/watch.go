package api

import (
	"context"
	"encoding/json"
	"net/http"

	"ams/internal/store"
)

// fillUserData adds the current profile's watch state to items: progress and watched for movies
// and episodes, unwatched episode counts for series.
func (s *Server) fillUserData(r *http.Request, items []*itemDTO) error {
	userID := currentUserID(r)
	if userID == "" || len(items) == 0 {
		return nil
	}
	var ids, seriesIDs []string
	for _, d := range items {
		if d.Kind == "series" {
			seriesIDs = append(seriesIDs, d.ID)
		} else {
			ids = append(ids, d.ID)
		}
	}
	data, err := s.store.UserDataFor(r.Context(), userID, ids)
	if err != nil {
		return err
	}
	counts, err := s.store.UnwatchedCounts(r.Context(), userID, seriesIDs)
	if err != nil {
		return err
	}
	for _, d := range items {
		if d.Kind == "series" {
			n := counts[d.ID]
			d.UnwatchedCount = &n
		} else {
			u := data[d.ID]
			d.UserData = &u
		}
	}
	return nil
}

func ptrs(items []itemDTO) []*itemDTO {
	out := make([]*itemDTO, len(items))
	for i := range items {
		out[i] = &items[i]
	}
	return out
}

// reportProgress: PUT /api/items/{id}/progress {"position_ms", "duration_ms"} from the player, every
// few seconds and on pause or close.
func (s *Server) reportProgress(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if it.Kind != "movie" && it.Kind != "episode" {
		writeError(w, http.StatusBadRequest, "only movies and episodes are played")
		return
	}
	var body struct {
		PositionMS int64 `json:"position_ms"`
		DurationMS int64 `json:"duration_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PositionMS < 0 || body.DurationMS < 0 {
		writeError(w, http.StatusBadRequest, `send {"position_ms": ..., "duration_ms": ...}`)
		return
	}
	if body.DurationMS == 0 {
		// Fall back to what the file says.
		if files, err := s.store.ItemFiles(r.Context(), it.ID); err == nil && len(files) > 0 {
			body.DurationMS = files[0].DurationMS
		}
	}
	d, err := s.store.ReportProgress(r.Context(), currentUserID(r), it.ID, body.PositionMS, body.DurationMS)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// setPlayed: PUT /api/items/{id}/played {"played": true} marks a movie or episode watched or
// unwatched for the current profile. For a series it applies to all its episodes, or with
// "season_number" to one season of the profile's episode order.
func (s *Server) setPlayed(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Played *bool `json:"played"`
		Season *int  `json:"season_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Played == nil {
		writeError(w, http.StatusBadRequest, `send {"played": true or false}`)
		return
	}
	ids := []string{it.ID}
	if it.Kind == "series" {
		var err error
		if ids, err = s.seriesEpisodeIDs(r, it, body.Season); err != nil {
			s.fail(w, err)
			return
		}
	}
	if err := s.store.SetPlayed(r.Context(), currentUserID(r), ids, *body.Played); err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// seriesEpisodeIDs returns a series' episodes on disk: all of them, or one season of the
// profile's display ordering.
func (s *Server) seriesEpisodeIDs(r *http.Request, series *store.Item, season *int) ([]string, error) {
	ctx := r.Context()
	available, err := s.store.AvailableChildren(ctx, series.ID)
	if err != nil {
		return nil, err
	}
	var ids []string
	if season == nil {
		for id := range available {
			ids = append(ids, id)
		}
		return ids, nil
	}
	lib, err := s.store.GetLibrary(ctx, series.LibraryID)
	if err != nil {
		return nil, err
	}
	oid, err := s.store.DisplayOrdering(ctx, series.ID, currentUserID(r), lib.SourceOrdering)
	if err != nil || oid == 0 {
		return nil, err
	}
	entries, err := s.store.OrderingEntries(ctx, oid)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.Season == *season && available[e.EpisodeID] {
			ids = append(ids, e.EpisodeID)
		}
	}
	return ids, nil
}

// episodeDTO describes an episode on its own (e.g. in Continue Watching): with its series' title
// and its place in the profile's episode order.
func (s *Server) episodeDTO(ep store.Item, series *store.Item, positions map[string][2]int) itemDTO {
	d := toDTO(ep)
	if series != nil {
		d.SeriesTitle = series.Title
		// Show the series' backdrop behind an episode that has no still of its own.
		if ep.PosterPath == "" && series.BackdropPath != "" {
			d.Images["series_backdrop"] = "/api/items/" + series.ID + "/images/backdrop"
		}
		d.Images["series_primary"] = "/api/items/" + series.ID + "/images/primary"
	}
	if p, ok := positions[ep.ID]; ok {
		season, number := p[0], p[1]
		d.SeasonNumber, d.EpisodeNumber = &season, &number
	}
	return d
}

// orderPositions returns where each episode of a series sits in the profile's episode order
// (its first position), and the entries in order.
func (s *Server) orderPositions(ctx context.Context, userID string, series *store.Item) (map[string][2]int, []store.OrderingEntry, error) {
	lib, err := s.store.GetLibrary(ctx, series.LibraryID)
	if err != nil || lib == nil {
		return nil, nil, err
	}
	oid, err := s.store.DisplayOrdering(ctx, series.ID, userID, lib.SourceOrdering)
	if err != nil || oid == 0 {
		return map[string][2]int{}, nil, err
	}
	entries, err := s.store.OrderingEntries(ctx, oid)
	if err != nil {
		return nil, nil, err
	}
	pos := map[string][2]int{}
	for _, e := range entries {
		if _, ok := pos[e.EpisodeID]; !ok {
			pos[e.EpisodeID] = [2]int{e.Season, e.Episode}
		}
	}
	return pos, entries, nil
}

// continueWatching: GET /api/home/continue lists what the profile is part-way through.
func (s *Server) continueWatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := currentUserID(r)
	items, err := s.store.InProgress(ctx, userID, 30)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]itemDTO, 0, len(items))
	for _, it := range items {
		if it.Kind == "movie" {
			out = append(out, toDTO(it))
			continue
		}
		series, err := s.store.GetItem(ctx, it.ParentID)
		if err != nil {
			s.fail(w, err)
			return
		}
		pos := map[string][2]int{}
		if series != nil {
			if pos, _, err = s.orderPositions(ctx, userID, series); err != nil {
				s.fail(w, err)
				return
			}
		}
		out = append(out, s.episodeDTO(it, series, pos))
	}
	if err := s.fillUserData(r, ptrs(out)); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// nextUp: GET /api/home/next-up lists, for each show the profile has been watching, the next
// episode in their episode order that's on disk and not watched yet (skipping specials and
// anything already in Continue Watching).
func (s *Server) nextUp(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := currentUserID(r)
	watched, err := s.store.RecentlyWatchedSeries(ctx, userID, 40)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := []itemDTO{}
	for _, ws := range watched {
		series, err := s.store.GetItem(ctx, ws.SeriesID)
		if err != nil {
			s.fail(w, err)
			return
		}
		if series == nil {
			continue
		}
		pos, entries, err := s.orderPositions(ctx, userID, series)
		if err != nil {
			s.fail(w, err)
			return
		}
		available, err := s.store.AvailableChildren(ctx, series.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		var ids []string
		for _, e := range entries {
			ids = append(ids, e.EpisodeID)
		}
		data, err := s.store.UserDataFor(ctx, userID, ids)
		if err != nil {
			s.fail(w, err)
			return
		}
		// Start after the last episode watched; specials don't count as next.
		start := -1
		for i, e := range entries {
			if e.EpisodeID == ws.LastEpisodeID {
				start = i
				break
			}
		}
		if start < 0 {
			continue // the last one watched isn't in this order
		}
		for _, e := range entries[start+1:] {
			d := data[e.EpisodeID]
			if e.Season == 0 || !available[e.EpisodeID] || d.Played || e.EpisodeID == ws.LastEpisodeID {
				continue
			}
			if d.PositionMS == 0 { // part-watched ones are in Continue Watching
				ep, err := s.store.GetItem(ctx, e.EpisodeID)
				if err != nil {
					s.fail(w, err)
					return
				}
				if ep != nil {
					out = append(out, s.episodeDTO(*ep, series, pos))
				}
			}
			break
		}
		if len(out) >= 20 {
			break
		}
	}
	if err := s.fillUserData(r, ptrs(out)); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
