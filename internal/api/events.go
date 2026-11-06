package api

import (
	"ams/internal/events"
	"ams/internal/store"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const eventBatchInterval = 500 * time.Millisecond

func (s *Server) eventPrincipal(r *http.Request, expected string) (*store.User, bool) {
	se := s.session(r)
	if se == nil || se.UserID != expected {
		return nil, false
	}
	u, err := s.store.GetUser(r.Context(), expected)
	return u, err == nil && u != nil
}
func eventAllowed(e events.Event, u *store.User) bool {
	if e.UserID != "" && e.UserID != u.ID {
		return false
	}
	if e.Admin && !u.IsAdmin {
		return false
	}
	return e.LibraryID == 0 || u.IsAdmin || (store.Access{Libraries: u.LibraryIDs}).LibraryAllowed(e.LibraryID)
}

// eventStream: GET /api/events is a profile-authenticated Server-Sent Events stream. "invalidate"
// hints identify a topic and optionally an allowed library, never titles/paths/item bodies. Personal
// hints only reach that profile; activity/sessions/maintenance are admin-only. "resync" requires refetching all
// client state. Send Last-Event-ID to resume; stale/process-restart cursors trigger resync. No tokens
// in URLs. Background catalog/artwork/optimized/previews/markers hints are cache invalidations,
// not job-success or playback commands. Max 32 streams/server; retries use 429 Retry-After. Heartbeats/revalidation every 10 seconds.
func (s *Server) eventStream(w http.ResponseWriter, r *http.Request) {
	if len(r.Header.Get("Last-Event-ID")) > 80 {
		writeError(w, 400, "invalid event cursor")
		return
	}
	if !s.events.Open() {
		w.Header().Set("Retry-After", "5")
		writeError(w, 429, "too many event streams")
		return
	}
	defer s.events.Close()
	userID := currentUserID(r)
	if _, ok := s.eventPrincipal(r, userID); !ok {
		writeError(w, 401, "sign in again")
		return
	}
	controller := http.NewResponseController(w)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(200)
	defer controller.SetWriteDeadline(time.Time{})
	send := func(kind, id string, payload any) bool {
		data, err := json.Marshal(payload)
		if err != nil {
			return false
		}
		_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", id, kind, data); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	cursor := r.Header.Get("Last-Event-ID")
	pending, next, resync := s.events.Read(cursor)
	cursor = next
	if resync {
		if !send("resync", cursor, map[string]string{"topic": "resync"}) {
			return
		}
	}
	// Valid replay is sent through the same fresh authorization checks as live hints.
	deliver := func(batch []events.Event) bool {
		u, ok := s.eventPrincipal(r, userID)
		if !ok {
			return false
		}
		for _, e := range batch {
			if eventAllowed(e, u) {
				kind := "invalidate"
				if e.Topic == "resync" {
					kind = "resync"
				}
				if !send(kind, e.ID, e) {
					return false
				}
			}
		}
		return true
	}
	if !deliver(pending) {
		return
	}
	if err := controller.Flush(); err != nil {
		return
	}
	heartbeat := s.eventHeartbeat
	if heartbeat <= 0 {
		heartbeat = 10 * time.Second
	}
	ticks := time.NewTicker(eventBatchInterval)
	defer ticks.Stop()
	beats := time.NewTicker(heartbeat)
	defer beats.Stop()
	lifetime := time.NewTimer(30 * time.Minute)
	defer lifetime.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-lifetime.C:
			return
		case <-beats.C:
			if _, ok := s.eventPrincipal(r, userID); !ok {
				return
			}
			_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			if controller.Flush() != nil {
				return
			}
		case <-ticks.C:
			batch, next, reset := s.events.Read(cursor)
			cursor = next
			if reset {
				if _, ok := s.eventPrincipal(r, userID); !ok {
					return
				}
				if !send("resync", cursor, map[string]string{"topic": "resync"}) {
					return
				}
			}
			if len(batch) > 0 && !deliver(batch) {
				return
			}
		}
	}
}

// publishMutation emits coarse hints only after successful writes; read-only POSTs are excluded.
func (s *Server) publishMutation(r *http.Request, route string) {
	if s.events == nil || r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
		return
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/watch-together") {
		return
	}
	if strings.HasSuffix(path, "/batch") || strings.HasSuffix(path, "/playback-options") || strings.HasSuffix(path, "/playback-preview") || strings.HasSuffix(path, "/compatibility") || strings.HasSuffix(path, "/ping") || strings.HasPrefix(path, "/api/auth/") {
		return
	}
	se := s.session(r)
	if se == nil || se.UserID == "" {
		return
	}
	personal := strings.HasPrefix(path, "/api/me/") || strings.HasPrefix(path, "/api/playlists") || strings.HasPrefix(path, "/api/play-queues") || strings.HasPrefix(path, "/api/home/continue") || strings.HasPrefix(path, "/api/news")
	for _, suffix := range []string{"/progress", "/played", "/watchlist", "/rating"} {
		personal = personal || strings.HasSuffix(path, suffix)
	}
	if personal {
		s.events.Publish(events.Event{Topic: "profile", UserID: se.UserID})
		return
	}
	if strings.HasSuffix(path, "/playback") || strings.HasPrefix(path, "/api/transcode/") {
		s.events.Publish(events.Event{Topic: "playback", UserID: se.UserID})
		s.events.Publish(events.Event{Topic: "sessions", Admin: true})
		return
	}
	u, err := s.store.GetUser(r.Context(), se.UserID)
	if err != nil || u == nil || !u.IsAdmin {
		return
	}
	var library int64
	if strings.HasPrefix(path, "/api/libraries/") {
		parts := strings.Split(path, "/")
		if len(parts) > 3 {
			library, _ = strconv.ParseInt(parts[3], 10, 64)
		}
	}
	if strings.HasPrefix(path, "/api/items/") {
		parts := strings.Split(path, "/")
		if len(parts) > 3 {
			if it, err := s.store.GetItem(r.Context(), parts[3]); err == nil && it != nil {
				library = it.LibraryID
			}
		}
	}
	s.events.Publish(events.Event{Topic: "catalog", LibraryID: library})
}
