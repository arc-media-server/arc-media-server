package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"ams/internal/lifecycle"
)

func TestRootShutdownCancelsRequestsButListenerRebindDoesNot(t *testing.T) {
	root, cancel := context.WithCancel(t.Context())
	defer cancel()
	closing := make(chan struct{})
	ctx := ShutdownContextWithParent(root, closing)
	request := httptest.NewRequest("GET", "/api/health", nil).WithContext(ctx)
	close(closing)
	select {
	case <-shuttingDown(request):
	default:
		t.Fatal("streams did not observe listener replacement")
	}
	if ctx.Err() != nil {
		t.Fatal("listener replacement canceled ordinary requests/root jobs")
	}
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("process cancellation did not reach requests")
	}
}

func TestTrackedRequestsFinishBeforeAsyncDrain(t *testing.T) {
	owner := lifecycle.New(t.Context(), nil)
	var jobs sync.WaitGroup
	s := &Server{workContext: owner.Context(), workers: &jobs}
	owner.OnStop("API jobs", jobs.Wait)
	workStarted, workRelease := make(chan struct{}), make(chan struct{})
	var release sync.Once
	defer func() { release.Do(func() { close(workRelease) }); owner.Close() }()
	handler := TrackRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.launchWork(func(ctx context.Context) { close(workStarted); <-ctx.Done(); <-workRelease }) {
			t.Error("work not admitted")
		}
		w.WriteHeader(204)
	}), func() (func(), bool) { return owner.Begin("HTTP requests") })
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("POST", "/api/fixture", nil))
	<-workStarted
	done := make(chan struct{})
	go func() { owner.Close(); close(done) }()
	<-owner.Context().Done()
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, httptest.NewRequest("POST", "/api/fixture", nil))
	if denied.Code != 503 || denied.Header().Get("Retry-After") == "" {
		t.Fatalf("stopping admission: %d %s", denied.Code, denied.Body.String())
	}
	select {
	case <-done:
		t.Fatal("async drain returned before work finished")
	case <-time.After(20 * time.Millisecond):
	}
	release.Do(func() { close(workRelease) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("process did not drain")
	}
	if s.launchWork(func(context.Context) { t.Error("stopped API work ran") }) {
		t.Fatal("stopped root admitted API work")
	}
}
