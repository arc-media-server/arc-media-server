package api

import (
	"net/http"

	"ams/internal/activity"
)

// getActivity: GET /api/activity (admin) lists what the server is doing now, and what it finished
// recently (newest first).
func (s *Server) getActivity(w http.ResponseWriter, r *http.Request) {
	running, recent := s.activity.Snapshot()
	writeJSON(w, http.StatusOK, struct {
		Running []activity.Task `json:"running"`
		Recent  []activity.Task `json:"recent"`
	}{running, recent})
}
