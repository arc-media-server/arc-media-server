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

type remoteCheckDTO struct {
	Reachable bool    `json:"reachable"`
	Error     *string `json:"error,omitempty"`
}

// remoteCheck: POST /api/remote/check (admin) tries to reach Arc at its outside address.
func (s *Server) remoteCheck(w http.ResponseWriter, r *http.Request) {
	if s.remote == nil {
		writeError(w, http.StatusNotFound, "remote access isn't available")
		return
	}
	out := remoteCheckDTO{Reachable: true}
	if err := s.remote.Check(r.Context()); err != nil {
		message := err.Error()
		out = remoteCheckDTO{Reachable: false, Error: &message}
	}
	writeJSON(w, http.StatusOK, out)
}
