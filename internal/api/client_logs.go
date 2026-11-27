package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"ams/internal/store"
)

const clientLogMaxBytes = 256 << 10
const clientLogMaxEntries = 3000

var clientLogPlatforms = []string{"android", "apple", "kodi", "roku", "tv"}
var clientLogLevels = []string{"debug", "info", "warn", "error"}

// Only fixed diagnostic event names cross the wire. In particular, there is no message,
// exception text, URL, body, stack trace, hostname or arbitrary key/value field.
var clientLogEvents = []string{
	"app_started", "profile_picker", "automatic_sign_in", "sign_in_failed",
	"request_failed", "request_timeout", "session_expired",
	"library_load_failed", "recommendations_load_failed",
	"playback_requested", "playback_started", "playback_paused", "playback_resumed",
	"playback_ended", "playback_failed", "playback_buffering", "playback_seek_failed",
	"subtitles_failed", "download_failed", "crash_detected",
	"detailed_logging_enabled", "detailed_logging_expired",
}

type clientLogUpload struct {
	Platform string                 `json:"platform"`
	Entries  []store.ClientLogEntry `json:"entries"`
}

// uploadClientLogs: POST /api/client-logs appends a user-approved batch for its bearer-authenticated device.
// Send only events from the documented vocabulary; upload through a signed-in app, never automatically.
func (s *Server) uploadClientLogs(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Authorization")), "bearer ") || currentDevice(r) == "" {
		writeError(w, http.StatusForbidden, "a signed-in app is required")
		return
	}
	if r.ContentLength > clientLogMaxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "client logs can be up to 256 KiB")
		return
	}
	var body clientLogUpload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, clientLogMaxBytes))
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&body)
	if err == nil {
		err = decoder.Decode(new(any))
		if err == nil {
			err = errors.New("multiple JSON values")
		} else if errors.Is(err, io.EOF) {
			err = nil
		}
	}
	var tooLarge *http.MaxBytesError
	now := time.Now()
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "client logs can be up to 256 KiB")
		return
	}
	if err != nil || !validClientLogs(body, now) {
		writeError(w, http.StatusBadRequest, "invalid client log batch")
		return
	}
	ok, err := s.store.SaveClientLogs(r.Context(), currentSession(r), body.Platform, body.Entries, now)
	if err != nil {
		s.fail(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "a signed-in app is required")
		return
	}
	s.log.Debug("device logs received", "platform", body.Platform, "entries", len(body.Entries))
	w.WriteHeader(http.StatusNoContent)
}

func validClientLogs(body clientLogUpload, now time.Time) bool {
	if !slices.Contains(clientLogPlatforms, body.Platform) || len(body.Entries) == 0 || len(body.Entries) > clientLogMaxEntries {
		return false
	}
	for _, entry := range body.Entries {
		if entry.TimeMS <= 0 || entry.TimeMS > now.Add(24*time.Hour).UnixMilli() ||
			!slices.Contains(clientLogLevels, entry.Level) || !slices.Contains(clientLogEvents, entry.Event) {
			return false
		}
	}
	return true
}

// listClientLogDevices: GET /api/client-logs (admin) lists devices with unexpired entries.
func (s *Server) listClientLogDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	devices, err := s.store.ListClientLogDevices(r.Context(), time.Now())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, devices)
}

// getClientLogs: GET /api/client-logs/{id} (admin) returns recent batches for the selected device.
func (s *Server) getClientLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "device logs not found")
		return
	}
	batches, err := s.store.ClientLogs(r.Context(), id, time.Now())
	if err != nil {
		s.fail(w, err)
		return
	}
	if len(batches) == 0 {
		writeError(w, http.StatusNotFound, "device logs not found")
		return
	}
	writeJSON(w, http.StatusOK, batches)
}
