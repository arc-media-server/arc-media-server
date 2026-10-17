package api

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"time"

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
		// playing (the default), paused, or stopped when the player closes.
		State string `json:"state"`
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
	// The progress is saved either way, but a player the admin stopped is told to close.
	why, play := s.playing.progress(r, it.ID, body.PositionMS, body.DurationMS, body.State)
	if play != nil {
		s.recordPlay(r.Context(), it, play)
	}
	if why != "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": why, "needs": "stopped"})
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// recordPlay adds a viewing to watch history, or updates it. Failures are only logged: they
// mustn't interrupt playback.
func (s *Server) recordPlay(ctx context.Context, it *store.Item, u *playUpdate) {
	p := store.Play{ID: u.historyID, UserID: u.userID, ItemID: it.ID, Kind: it.Kind, LibraryID: it.LibraryID, Title: it.Title,
		StartedAt: u.started.Unix(), EndedAt: time.Now().Unix(), WatchedMS: u.watchedMS, PositionMS: u.positionMS,
		DurationMS: u.durationMS, Completed: u.completed, Device: u.device, Method: u.method}
	if !u.add {
		if err := s.store.UpdatePlay(ctx, p); err != nil {
			s.log.Warn("updating watch history failed", "item", it.Title, "err", err)
		}
		return
	}
	if it.Kind == "episode" {
		p.SeriesID = it.ParentID
		if series, err := s.store.GetItem(ctx, it.ParentID); err == nil && series != nil {
			p.SeriesTitle = series.Title
		}
	}
	id, err := s.store.AddPlay(ctx, p)
	if err != nil {
		s.log.Warn("adding to watch history failed", "item", it.Title, "err", err)
	}
	s.playing.played(u.key, id)
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
		// The series' backdrop, for an episode with no still of its own or whose still would be a
		// spoiler.
		if b := toDTO(*series).Images["backdrop"]; b != "" {
			d.Images["series_backdrop"] = b
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

// homeEntry is a movie or episode on the home page, with when the profile was last watching it
// (or, for an episode up next, the show).
type homeEntry struct {
	item store.Item
	at   int64
}

// continueWatching: GET /api/home/continue lists what the profile is part-way through, most
// recent first. With ?with_up_next=1 it also has the next episode of each show they're watching,
// as Plex's Continue Watching does, one entry per show.
func (s *Server) continueWatching(w http.ResponseWriter, r *http.Request) {
	entries, err := s.inProgress(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	if r.URL.Query().Get("with_up_next") == "1" {
		next, err := s.upNext(r)
		if err != nil {
			s.fail(w, err)
			return
		}
		entries = mergeHome(entries, next, 30)
	}
	s.writeHome(w, r, entries)
}

// nextUp: GET /api/home/next-up lists, for each show the profile has been watching, the next
// episode in their episode order that's on disk and not watched yet (skipping specials and
// anything already in Continue Watching).
func (s *Server) nextUp(w http.ResponseWriter, r *http.Request) {
	entries, err := s.upNext(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.writeHome(w, r, entries)
}

// mergeHome puts part-watched items and up-next episodes together, most recent first, keeping
// only the most recent entry for each show.
func mergeHome(inProgress, upNext []homeEntry, limit int) []homeEntry {
	all := append(slices.Clone(inProgress), upNext...)
	slices.SortStableFunc(all, func(a, b homeEntry) int { return cmp.Compare(b.at, a.at) })
	out := []homeEntry{}
	shows := map[string]bool{}
	for _, e := range all {
		if e.item.Kind == "episode" {
			if shows[e.item.ParentID] {
				continue
			}
			shows[e.item.ParentID] = true
		}
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	return out
}

// writeHome sends home entries with their episode numbers (in the profile's order) and watch state.
func (s *Server) writeHome(w http.ResponseWriter, r *http.Request, entries []homeEntry) {
	ctx := r.Context()
	userID := currentUserID(r)
	out := make([]itemDTO, 0, len(entries))
	series := map[string]*store.Item{}
	positions := map[string]map[string][2]int{}
	for _, e := range entries {
		it := e.item
		if it.Kind != "episode" {
			out = append(out, toDTO(it))
			continue
		}
		sr, ok := series[it.ParentID]
		if !ok {
			var err error
			if sr, err = s.store.GetItem(ctx, it.ParentID); err != nil {
				s.fail(w, err)
				return
			}
			pos := map[string][2]int{}
			if sr != nil {
				if pos, _, err = s.orderPositions(ctx, userID, sr); err != nil {
					s.fail(w, err)
					return
				}
			}
			series[it.ParentID], positions[it.ParentID] = sr, pos
		}
		out = append(out, s.episodeDTO(it, sr, positions[it.ParentID]))
	}
	if err := s.fillUserData(r, ptrs(out)); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// inProgress lists the movies and episodes the profile is part-way through, most recent first.
func (s *Server) inProgress(r *http.Request) ([]homeEntry, error) {
	ctx := r.Context()
	items, err := s.store.InProgress(ctx, currentUserID(r), 30)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	data, err := s.store.UserDataFor(ctx, currentUserID(r), ids)
	if err != nil {
		return nil, err
	}
	out := []homeEntry{}
	for _, it := range items {
		if ok, err := s.store.AgeAllowed(ctx, &it, maxAge(r)); err != nil {
			return nil, err
		} else if ok {
			out = append(out, homeEntry{it, data[it.ID].LastPlayedAt})
		}
	}
	return out, nil
}

// upNext finds the next episode to watch of each show the profile has been watching.
func (s *Server) upNext(r *http.Request) ([]homeEntry, error) {
	ctx := r.Context()
	userID := currentUserID(r)
	watched, err := s.store.RecentlyWatchedSeries(ctx, userID, 40)
	if err != nil {
		return nil, err
	}
	out := []homeEntry{}
	for _, ws := range watched {
		series, err := s.store.GetItem(ctx, ws.SeriesID)
		if err != nil {
			return nil, err
		}
		if series == nil {
			continue
		}
		if ok, err := s.store.AgeAllowed(ctx, series, maxAge(r)); err != nil {
			return nil, err
		} else if !ok {
			continue
		}
		_, entries, err := s.orderPositions(ctx, userID, series)
		if err != nil {
			return nil, err
		}
		available, err := s.store.AvailableChildren(ctx, series.ID)
		if err != nil {
			return nil, err
		}
		var ids []string
		for _, e := range entries {
			ids = append(ids, e.EpisodeID)
		}
		data, err := s.store.UserDataFor(ctx, userID, ids)
		if err != nil {
			return nil, err
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
					return nil, err
				}
				if ep != nil {
					out = append(out, homeEntry{*ep, ws.LastPlayedAt})
				}
			}
			break
		}
		if len(out) >= 20 {
			break
		}
	}
	return out, nil
}
