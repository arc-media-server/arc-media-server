package api

import (
	"ams/internal/auth"
	"ams/internal/events"
	"ams/internal/store"
	"context"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type streamStartedWriter struct {
	*httptest.ResponseRecorder
	started chan struct{}
	once    sync.Once
}

func (w *streamStartedWriter) Flush() {
	w.ResponseRecorder.Flush()
	w.once.Do(func() { close(w.started) })
}

func TestOneProfileCannotConsumeAllEventStreams(t *testing.T) {
	s, u := securityServer(t)
	s.events = events.New()
	other, err := s.store.CreateUser(t.Context(), "Other viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []*store.User{u, other} {
		if err := s.store.CreateSession(t.Context(), auth.HashToken(profile.ID), profile.ID, "password", ""); err != nil {
			t.Fatal(err)
		}
	}
	start := func(profile *store.User) context.CancelFunc {
		t.Helper()
		ctx, cancel := context.WithCancel(t.Context())
		r := httptest.NewRequest("GET", "/api/events", nil).WithContext(context.WithValue(ctx, userKey, profile))
		r.Header.Set("Authorization", "Bearer "+profile.ID)
		w := &streamStartedWriter{ResponseRecorder: httptest.NewRecorder(), started: make(chan struct{})}
		done := make(chan struct{})
		go func() { defer close(done); s.eventStream(w, r) }()
		select {
		case <-w.started:
		case <-done:
			t.Fatalf("stream denied early: %d", w.Code)
		case <-time.After(5 * time.Second):
			cancel()
			t.Fatal("stream did not start")
		}
		stop := func() { cancel(); <-done }
		t.Cleanup(stop)
		return stop
	}
	stops := []context.CancelFunc{}
	for range 8 {
		stops = append(stops, start(u))
	}
	ctx, cancel := context.WithCancel(context.WithValue(t.Context(), userKey, u))
	r := httptest.NewRequest("GET", "/api/events", nil).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+u.ID)
	w := &streamStartedWriter{ResponseRecorder: httptest.NewRecorder(), started: make(chan struct{})}
	done := make(chan struct{})
	go func() { defer close(done); s.eventStream(w, r) }()
	select {
	case <-done:
	case <-w.started:
		cancel()
		<-done
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("limit response stalled")
	}
	cancel()
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Errorf("ninth stream: %d; wanted 429 with Retry-After", w.Code)
	}
	stopOther := start(other)
	stopOther()
	stops[0]()
	start(u)() // Disconnection returns the slot, rather than permanently throttling the profile.
}
