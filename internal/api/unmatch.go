package api

import (
	"ams/internal/scanner"
	"errors"
	"net/http"
)

// unmatchItem: POST /api/items/{id}/unmatch (admin) removes all metadata and artwork references,
// manual metadata overrides and display orders from a show and its episodes. Restores filename
// labels and file order, preserves IDs, files, source numbering, history, manual collections and
// playlists. Scans/refreshes keep it unmatched until an explicit Fix Match succeeds. No media,
// uploaded artwork files, probes or markers are deleted. Only series IDs are accepted.
func (s *Server) unmatchItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if it.Kind != "series" {
		writeError(w, 400, "only shows can be unmatched")
		return
	}
	if s.scanner == nil {
		writeError(w, 503, "scanning unavailable")
		return
	}
	if err := s.scanner.Unmatch(r.Context(), it); err != nil {
		if errors.Is(err, scanner.ErrLibraryBusy) {
			writeError(w, 409, "the library or show match is busy; retry when work finishes")
			return
		}
		if errors.Is(err, r.Context().Err()) {
			return
		}
		s.fail(w, err)
		return
	}
	s.getItem(w, r)
}
