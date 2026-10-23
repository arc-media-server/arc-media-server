package api

import (
	"net/http"

	"ams/internal/activity"
	"ams/internal/background"
)

// getActivity: GET /api/activity (admin) lists what the server is doing now, and what it finished
// recently (newest first).
func (s *Server) getActivity(w http.ResponseWriter, r *http.Request) {
	running, recent := s.activity.Snapshot()
	writeJSON(w, http.StatusOK, struct {
		Running   []activity.Task     `json:"running"`
		Recent    []activity.Task     `json:"recent"`
		Scheduled []background.Status `json:"scheduled,omitempty"`
	}{running, recent, s.scheduled(r)})
}

func (s *Server) getMaintenance(w http.ResponseWriter, r *http.Request) {
	if s.background == nil {
		writeJSON(w, 200, []any{})
		return
	}
	writeJSON(w, 200, s.background.Snapshot(r.Context()))
}
func (s *Server) runMaintenance(w http.ResponseWriter, r *http.Request) {
	if s.background == nil {
		writeError(w, 503, "Background scheduling unavailable")
		return
	}
	if err := s.background.RunNow(r.Context(), r.PathValue("kind")); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 202, map[string]bool{"queued": true})
}

func (s *Server) scheduled(r *http.Request) []background.Status {
	if s.background == nil {
		return nil
	}
	return s.background.Snapshot(r.Context())
}
