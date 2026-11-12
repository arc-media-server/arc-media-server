package api

import (
	"net/http"
	"strings"
)

func catalogMutationRoute(r *http.Request) bool {
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/backups/") && strings.HasSuffix(r.URL.Path, "/restore") {
		return true
	}
	if r.Method == http.MethodDelete {
		rest, ok := strings.CutPrefix(r.URL.Path, "/api/libraries/")
		return ok && rest != "" && !strings.Contains(rest, "/")
	}
	return false
}
func (s *Server) beginCatalogMutation(w http.ResponseWriter, r *http.Request) (func(), bool) {
	release, ok := s.store.ReserveCatalogMutation()
	if !ok {
		writeError(w, http.StatusConflict, "library or background work is running; try again when it finishes")
		return nil, false
	}
	s.scanner.InvalidateTrashProof()
	return func() {
		release()
		s.scanner.ResumeQueued()
		s.background.WakeWorkers()
		if s.optimizer != nil {
			s.optimizer.Wake()
		}
	}, true
}
