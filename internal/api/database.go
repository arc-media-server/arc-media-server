package api

import (
	"ams/internal/databaseconfig"
	"encoding/json"
	"io"
	"net/http"
)

// databaseStatus: GET /api/database (admin) reports engine and portable backup/transfer status.
// Connection passwords and bootstrap contents are never returned. SQLite remains the default.
func (s *Server) databaseStatus(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeError(w, 503, "database management is unavailable")
		return
	}
	out, err := s.database.Status()
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, out)
}

// databaseTest: POST /api/database/test (admin) checks PostgreSQL connectivity, verified TLS by
// default, supported version, target emptiness and schema-create privilege without changing data.
func (s *Server) databaseTest(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeError(w, 503, "database management is unavailable")
		return
	}
	var in databaseconfig.Connection
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		writeError(w, 400, "invalid database request")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		writeError(w, 400, "unexpected trailing request content")
		return
	}
	out, err := s.database.Test(r.Context(), in)
	if err != nil {
		writeError(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, out)
}

type databaseTransferInput struct {
	Connection      databaseconfig.Connection `json:"connection" api:"required"`
	ConfirmDowntime bool                      `json:"confirm_downtime" api:"required"`
}
type databaseTransferResult struct {
	State string `json:"state"`
}

// databaseTransfer: POST /api/database/transfer (admin) explicitly confirms downtime, journals
// a SQLite-to-PostgreSQL move and restarts Arc through maintenance after requests/workers drain.
// The empty target is committed only after content/relationship validation; SQLite safety state
// is retained. No rescan/rematch is required. This endpoint never switches an active PostgreSQL
// installation back to stale SQLite. Poll database status after reconnect; passwords are write-only.
func (s *Server) databaseTransfer(w http.ResponseWriter, r *http.Request) {
	if s.database == nil {
		writeError(w, 503, "database management is unavailable")
		return
	}
	var in databaseTransferInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		writeError(w, 400, "invalid database request")
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		writeError(w, 400, "unexpected trailing request content")
		return
	}
	if !in.ConfirmDowntime {
		writeError(w, 400, "confirm maintenance downtime; active playback and background work will stop")
		return
	}
	if err := s.database.Request(r.Context(), in.Connection); err != nil {
		writeError(w, 409, err.Error())
		return
	}
	writeJSON(w, 202, databaseTransferResult{State: "requested"})
}
