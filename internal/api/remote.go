package api

import "net/http"

// remoteStatus: GET /api/remote (admin) says how remote access stands: the outside address, the
// router's port forwarding and the HTTPS certificate.
func (s *Server) remoteStatus(w http.ResponseWriter, r *http.Request) {
	if s.remote == nil {
		writeError(w, http.StatusNotFound, "remote access isn't available")
		return
	}
	writeJSON(w, http.StatusOK, s.remote.Status(r.Context()))
}

// remoteCheck: POST /api/remote/check (admin) tries to reach Arc at its outside address.
func (s *Server) remoteCheck(w http.ResponseWriter, r *http.Request) {
	if s.remote == nil {
		writeError(w, http.StatusNotFound, "remote access isn't available")
		return
	}
	out := map[string]any{"reachable": true}
	if err := s.remote.Check(r.Context()); err != nil {
		out = map[string]any{"reachable": false, "error": err.Error()}
	}
	writeJSON(w, http.StatusOK, out)
}
