package api

import (
	"net/http"
	"time"

	"ams/internal/store"
)

// WriteDatabaseUnavailable is shared by the main API and the startup status listener.
func WriteDatabaseUnavailable(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "3")
	writeError(w, http.StatusServiceUnavailable, "Arc cannot reach its database. Try again shortly.", "database_unavailable")
}

func (s *Server) databaseUnavailable(w http.ResponseWriter, err error) bool {
	if s.store == nil || !s.store.IsPostgres() || !store.IsPostgresUnavailable(err) {
		return false
	}
	markFailureLogged(w)
	if s.postgresOutage.CompareAndSwap(0, time.Now().UnixNano()) {
		s.log.Warn("PostgreSQL unavailable")
	}
	WriteDatabaseUnavailable(w)
	return true
}

func (s *Server) databaseRecovered() {
	if since := s.postgresOutage.Swap(0); since != 0 {
		s.log.Info("PostgreSQL connection recovered", "unavailable_for", time.Since(time.Unix(0, since)).Round(time.Millisecond))
	}
}
