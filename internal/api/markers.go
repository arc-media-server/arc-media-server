package api

import (
	"encoding/json"
	"net/http"
	"slices"

	"ams/internal/store"
)

// playableMarkers are a file's skip markers for the player: the ones that skip something (a
// marker of length zero is the user saying there's nothing to skip).
func (s *Server) playableMarkers(r *http.Request, libraryID, fileID int64) ([]store.Marker, error) {
	all, err := s.store.FileMarkers(r.Context(), fileID)
	if err != nil {
		return nil, err
	}
	lib, err := s.store.GetLibrary(r.Context(), libraryID)
	if err != nil || lib == nil {
		return []store.Marker{}, err
	}
	out := []store.Marker{}
	for _, m := range all {
		// Each library chooses whether to offer Skip Intro (and Skip Recap) and Skip Credits.
		if m.Kind == "credits" && !lib.SkipCredits || m.Kind != "credits" && !lib.SkipIntros {
			continue
		}
		if m.EndMS > m.StartMS {
			out = append(out, m)
		}
	}
	return out, nil
}

// markersFile is the file of a movie or episode whose markers are shown and edited: the first
// one present.
func (s *Server) markersFile(w http.ResponseWriter, r *http.Request) (*store.MediaFile, bool) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return nil, false
	}
	files, err := s.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	for i := range files {
		if !files[i].Missing {
			return &files[i], true
		}
	}
	writeError(w, http.StatusNotFound, "no file to set markers for")
	return nil, false
}

// itemMarkers: GET /api/items/{id}/markers (admin) gives a movie's or episode's skip markers,
// where each came from, and the file's length.
func (s *Server) itemMarkers(w http.ResponseWriter, r *http.Request) {
	f, ok := s.markersFile(w, r)
	if !ok {
		return
	}
	ms, err := s.store.FileMarkers(r.Context(), f.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file_id": f.ID, "duration_ms": f.DurationMS, "markers": ms})
}

// setItemMarkers: PUT /api/items/{id}/markers {"markers": [{"kind", "start_ms", "end_ms"}]} (admin)
// sets the markers by hand. Kinds left out have nothing to skip; found ones never replace these.
func (s *Server) setItemMarkers(w http.ResponseWriter, r *http.Request) {
	f, ok := s.markersFile(w, r)
	if !ok {
		return
	}
	var body struct {
		Markers []store.Marker `json:"markers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	for _, m := range body.Markers {
		switch {
		case !slices.Contains(store.MarkerKinds, m.Kind):
			writeError(w, http.StatusBadRequest, "kind is intro, recap or credits")
			return
		case m.StartMS < 0 || m.EndMS <= m.StartMS || (f.DurationMS > 0 && m.EndMS > f.DurationMS+1000):
			writeError(w, http.StatusBadRequest, "each "+m.Kind+" must end after it starts, within the file")
			return
		}
	}
	if err := s.store.SetManualMarkers(r.Context(), f.ID, body.Markers); err != nil {
		s.fail(w, err)
		return
	}
	s.itemMarkers(w, r)
}

// resetItemMarkers: DELETE /api/items/{id}/markers (admin) forgets the markers, so they're found
// again from chapters, online and by detection.
func (s *Server) resetItemMarkers(w http.ResponseWriter, r *http.Request) {
	f, ok := s.markersFile(w, r)
	if !ok {
		return
	}
	if err := s.store.ResetMarkers(r.Context(), f.ID); err != nil {
		s.fail(w, err)
		return
	}
	if s.markers != nil {
		s.markers.Wake()
	}
	w.WriteHeader(http.StatusNoContent)
}
