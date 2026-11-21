package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"ams/internal/auth"
)

// createToolToken: POST /api/devices/tools {"name"} (admin) returns an ordinary device token
// once, with the creating administrator already selected. Send Authorization: Bearer <token>.
// List/revoke it through /devices, where is_tool marks it as a tool. The same session checks,
// last-used bookkeeping, stale-device cleanup and password-change rules apply as for other devices.
func (s *Server) createToolToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid tool name")
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || utf8.RuneCountInString(body.Name) > 100 || strings.ContainsFunc(body.Name, unicode.IsControl) {
		writeError(w, http.StatusBadRequest, "name must be 1–100 characters without control characters")
		return
	}
	token, hash := auth.NewToken()
	if err := s.store.CreateToolSession(r.Context(), hash, currentUserID(r), body.Name); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("tool device token created")
	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}
