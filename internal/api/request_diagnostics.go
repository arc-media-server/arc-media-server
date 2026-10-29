package api

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"
)

const requestWarningAfter = 10 * time.Second
const maxTrackedRequests = 64

type requestTraceKey struct{}

type requestTrace struct {
	mu           sync.Mutex
	RequestID    uint64    `json:"request_id"`
	Method       string    `json:"method"`
	Route        string    `json:"route"`
	Started      time.Time `json:"started_at"`
	Stage        string    `json:"stage"`
	StageStarted time.Time `json:"stage_started_at"`
	LibraryID    int64     `json:"library_id,omitempty"`
	finished     bool
}

type requestSnapshot struct {
	RequestID uint64 `json:"request_id"`
	Method    string `json:"method"`
	Route     string `json:"route"`
	Stage     string `json:"stage"`
	LibraryID int64  `json:"library_id,omitempty"`
	ElapsedMS int64  `json:"elapsed_ms"`
	StageMS   int64  `json:"stage_ms"`
}

type requestDiagnostics struct {
	mu         sync.Mutex
	active     map[*requestTrace]struct{}
	nextID     uint64
	omitted    uint64
	warnWindow time.Time
	warnings   int
}

// Track only short control requests implicated in the reported stall. Never time
// media streams, downloads, event streams or every successful polling request.
func trackRequest(route string) bool {
	switch route {
	case "GET /api/libraries", "GET /api/libraries/{id}/scan", "GET /api/home/continue",
		"PUT /api/items/{id}/watchlist", "PUT /api/items/{id}/played", "PUT /api/items/{id}/progress":
		return true
	}
	return false
}

func (d *requestDiagnostics) begin(method, route string) *requestTrace {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.active) >= maxTrackedRequests {
		d.omitted++
		return nil
	}
	if d.active == nil {
		d.active = make(map[*requestTrace]struct{})
	}
	d.nextID++
	now := time.Now()
	t := &requestTrace{RequestID: d.nextID, Method: method, Route: route, Started: now, Stage: "authentication", StageStarted: now}
	d.active[t] = struct{}{}
	return t
}

func (t *requestTrace) snapshot() requestSnapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	return requestSnapshot{t.RequestID, t.Method, t.Route, t.Stage, t.LibraryID, time.Since(t.Started).Milliseconds(), time.Since(t.StageStarted).Milliseconds()}
}

func (d *requestDiagnostics) finish(t *requestTrace) {
	t.mu.Lock()
	t.finished = true
	t.mu.Unlock()
	d.mu.Lock()
	delete(d.active, t)
	d.mu.Unlock()
}

func (d *requestDiagnostics) snapshot() any {
	d.mu.Lock()
	defer d.mu.Unlock()
	active := make([]requestSnapshot, 0, len(d.active))
	for t := range d.active {
		active = append(active, t.snapshot())
	}
	sort.Slice(active, func(i, j int) bool { return active[i].RequestID < active[j].RequestID })
	return struct {
		Active  []requestSnapshot `json:"active"`
		Limit   int               `json:"limit"`
		Omitted uint64            `json:"omitted_since_start"`
	}{active, maxTrackedRequests, d.omitted}
}

func (d *requestDiagnostics) allowWarning() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	if d.warnWindow.IsZero() || now.Sub(d.warnWindow) >= time.Minute {
		d.warnWindow = now
		d.warnings = 0
	}
	if d.warnings >= 5 {
		return false
	}
	d.warnings++
	return true
}

// Stage names must be fixed developer labels, never URLs, SQL, bodies or credentials.
func requestStage(r *http.Request, stage string, libraryID int64) {
	if t, ok := r.Context().Value(requestTraceKey{}).(*requestTrace); ok {
		t.mu.Lock()
		t.Stage = stage
		t.StageStarted = time.Now()
		t.LibraryID = libraryID
		t.mu.Unlock()
	}
}

func (s *Server) watchRequest(r *http.Request, route string) (*http.Request, func()) {
	if !trackRequest(route) {
		return r, func() {}
	}
	t := s.requests.begin(r.Method, route)
	if t == nil {
		return r, func() {}
	}
	timer := time.AfterFunc(requestWarningAfter, func() {
		t.mu.Lock()
		finished := t.finished
		t.mu.Unlock()
		if finished || !s.requests.allowWarning() {
			return
		}
		p := t.snapshot()
		// Pool stats do not query SQL or wait for the writer. They show pressure at
		// this instant, not a particular operation's SQL execution or queue duration.
		var database any
		if s.store != nil {
			database = s.store.DatabaseStats()
		}
		s.log.Warn("request still running", "request_id", p.RequestID, "method", p.Method, "route", p.Route,
			"stage", p.Stage, "library_id", p.LibraryID, "elapsed_ms", p.ElapsedMS, "stage_ms", p.StageMS, "database", database)
	})
	return r.WithContext(context.WithValue(r.Context(), requestTraceKey{}, t)), func() { s.requests.finish(t); timer.Stop() }
}
