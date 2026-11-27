package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"

	"ams/internal/auth"
)

// PostgresConnection is the editable part of Arc's saved PostgreSQL connection.
type PostgresConnection struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	NoTLS    bool   `json:"no_tls"`
}

type PostgresConnectionUpdate struct {
	PostgresConnection
	Password string `json:"password"`
}

type DatabaseSettingsView struct {
	Engine          string              `json:"engine"`
	Connection      *PostgresConnection `json:"connection,omitempty"`
	RestartRequired bool                `json:"restart_required"`
	CanEdit         bool                `json:"can_edit"`
}

type DatabaseSettings interface {
	Saved() DatabaseSettingsView
	Update(context.Context, PostgresConnectionUpdate) error
}

func protectedDatabaseConnection(r *http.Request) bool {
	if auth.IsHTTPS(r) {
		return true
	}
	// A local browser or SSH tunnel can send credentials over loopback HTTP.
	// Check both ends so a reverse proxy's loopback connection is insufficient.
	peer, _, _ := net.SplitHostPort(r.RemoteAddr)
	host, err := url.Parse("http://" + r.Host)
	if err != nil || !net.ParseIP(peer).IsLoopback() {
		return false
	}
	return host.Hostname() == "localhost" || net.ParseIP(host.Hostname()).IsLoopback()
}

// databaseSettingsView: GET /api/settings/database returns the active engine and saved connection fields.
// PostgreSQL passwords and connection strings are omitted.
func (s *Server) databaseSettingsView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	view := DatabaseSettingsView{Engine: "sqlite"}
	if s.store.IsPostgres() {
		view.Engine = "postgres"
		if s.databaseSettings != nil {
			view = s.databaseSettings.Saved()
			view.CanEdit = view.Connection != nil && protectedDatabaseConnection(r)
		}
	}
	writeJSON(w, http.StatusOK, view)
}

// updateDatabaseSettings: PUT /api/settings/database validates the PostgreSQL connection and saves
// it for the next start. Only HTTPS or direct loopback requests can submit database credentials.
func (s *Server) updateDatabaseSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.databaseSettings == nil || !s.store.IsPostgres() {
		writeError(w, http.StatusConflict, "PostgreSQL connection settings are unavailable")
		return
	}
	if !protectedDatabaseConnection(r) {
		writeError(w, http.StatusForbidden, "open Arc over HTTPS or a local SSH tunnel to edit the connection")
		return
	}
	var input PostgresConnectionUpdate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid database connection settings")
		return
	}
	if err := s.databaseSettings.Update(r.Context(), input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
