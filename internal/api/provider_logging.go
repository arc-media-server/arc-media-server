package api

import "net/http"

// logProviderFailure owns one actionable diagnostic without treating an upstream
// outage as an Arc server failure. Labels are constants at each call site.
func (s *Server) logProviderFailure(w http.ResponseWriter, r *http.Request, provider, operation string) {
	markFailureLogged(w)
	if r.Context().Err() != nil {
		return
	}
	s.log.Warn("provider request failed", "provider", provider, "operation", operation, "request_id", w.Header().Get(requestIDHeader))
}
