package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"runtime"
	"runtime/debug"
	"time"
)

// getDiagnostics: GET /api/diagnostics reports temporary detailed logging and bounded retention (admin).
func (s *Server) getDiagnostics(w http.ResponseWriter, r *http.Request) {
	if s.logs == nil {
		writeError(w, http.StatusServiceUnavailable, "diagnostics unavailable")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, s.logs.Status())
}

// setDiagnostics: PATCH /api/diagnostics {"detailed":true} enables detailed logs for two hours; false stops them (admin).
func (s *Server) setDiagnostics(w http.ResponseWriter, r *http.Request) {
	if s.logs == nil {
		writeError(w, http.StatusServiceUnavailable, "diagnostics unavailable")
		return
	}
	var body struct {
		Detailed *bool `json:"detailed"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil || body.Detailed == nil {
		writeError(w, http.StatusBadRequest, "detailed must be true or false")
		return
	}
	s.logs.SetDetailed(*body.Detailed)
	s.log.Info("detailed logging changed", "enabled", *body.Detailed)
	s.getDiagnostics(w, r)
}

// downloadDiagnostics: GET /api/diagnostics/download exports redacted logs, build information, tracked request stages and bounded goroutine call stacks as a ZIP (admin). No database, settings dump or media files are included.
func (s *Server) downloadDiagnostics(w http.ResponseWriter, r *http.Request) {
	if s.logs == nil {
		writeError(w, http.StatusServiceUnavailable, "diagnostics unavailable")
		return
	}
	data, err := s.logs.Export()
	if err != nil {
		s.fail(w, err)
		return
	}
	revision := s.revision
	dirty := false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && (revision == "" || revision == "unknown") {
				revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				dirty = setting.Value == "true"
			}
		}
	}
	var database any
	if s.store != nil {
		database = s.store.DatabaseStats()
	}
	manifest, _ := json.MarshalIndent(map[string]any{
		"version": s.version, "revision": revision, "modified_build": dirty, "go": runtime.Version(),
		"os": runtime.GOOS, "architecture": runtime.GOARCH, "exported_at": time.Now().UTC(),
		"logging": s.logs.Status(), "contents": "Redacted logs, build information, tracked request stages and bounded goroutine call stacks. Names and filesystem paths may be included.",
		"transcoder": s.transcoderCaps(), "database": database, "cpu_count": runtime.NumCPU(), "goroutines": runtime.NumGoroutine(),
	}, "", "  ")
	manifest = []byte(s.logs.Redact(string(manifest)))
	requests, _ := json.MarshalIndent(s.requests.snapshot(), "", "  ")
	stacks := []byte(s.logs.Redact(string(goroutineSnapshot())))
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for _, entry := range []struct {
		name string
		data []byte
	}{{"manifest.json", manifest}, {"logs.jsonl", data}, {"requests.json", requests}, {"goroutines.txt", stacks}} {
		f, e := z.Create(entry.name)
		if e != nil {
			s.fail(w, e)
			return
		}
		if _, e := f.Write(entry.data); e != nil {
			s.fail(w, e)
			return
		}
	}
	if err := z.Close(); err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="arc-diagnostics.zip"`)
	w.Write(out.Bytes())
}
