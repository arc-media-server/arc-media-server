package api

import (
	"errors"
	"net/http"

	"ams/docs"
	"ams/internal/activity"
	"ams/internal/background"
)

// openAPI: GET /api/openapi.json answers with the OpenAPI 3.1 description of this server's API,
// generated from the code, so apps and API tools can see exactly what this version offers.
func (s *Server) openAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(docs.OpenAPI)
}

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

// getMaintenance: GET /api/maintenance lists the scheduled tasks in Settings › Maintenance: each
// one's setting, whether it is running, waiting or paused, and its last result (admin).
func (s *Server) getMaintenance(w http.ResponseWriter, r *http.Request) {
	if s.background == nil {
		writeJSON(w, 200, []any{})
		return
	}
	writeJSON(w, 200, s.background.Snapshot(r.Context()))
}

// runMaintenance: POST /api/maintenance/{kind}/run starts a scheduled task now (Run now), outside
// the maintenance hours; playback and library work still go first (admin).
func (s *Server) runMaintenance(w http.ResponseWriter, r *http.Request) {
	if s.background == nil {
		writeError(w, 503, "Background scheduling unavailable")
		return
	}
	if err := s.background.RunNow(r.Context(), r.PathValue("kind")); err != nil {
		if errors.Is(err, background.ErrInvalidRequest) {
			err = errMsg(err.Error())
		}
		s.inputError(w, r, err)
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
