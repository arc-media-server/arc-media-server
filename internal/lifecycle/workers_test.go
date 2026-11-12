package lifecycle

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCloseCancelsAndWaitsBeforeDependenciesClose(t *testing.T) {
	var logs bytes.Buffer
	w := New(t.Context(), slog.New(slog.NewTextHandler(&logs, nil)))
	w.drainWarning = 10 * time.Millisecond
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer func() { once.Do(func() { close(release) }); w.Close() }()
	if !w.Go("metadata", func() { close(entered); <-w.Context().Done(); close(canceled); <-release }) {
		t.Fatal("work rejected")
	}
	<-entered
	done := make(chan struct{})
	go func() { w.Close(); close(done) }()
	<-canceled
	if w.Go("late", func() { t.Error("late worker ran") }) {
		t.Fatal("late admission accepted")
	}
	select {
	case <-done:
		t.Fatal("dependencies would close before worker exit")
	case <-time.After(25 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close did not join")
	}
	if strings.Count(logs.String(), "shutdown waiting for workers") != 1 || !strings.Contains(logs.String(), "metadata") {
		t.Fatalf("missing/bounded diagnostics: %s", logs.String())
	}
	w.Close()
}

func TestRequestReservationPreventsAsyncWaitRace(t *testing.T) {
	w := New(t.Context(), slog.New(slog.DiscardHandler))
	requestDone, ok := w.Begin("HTTP requests")
	if !ok {
		t.Fatal("request rejected")
	}
	var async sync.WaitGroup
	asyncGate := make(chan struct{})
	drained := make(chan struct{})
	go func() { w.Close(); async.Wait(); close(drained) }()
	<-w.Context().Done()
	// An already-admitted request may register accepted work while cancellation unwinds.
	// The process owner must join requests before waiting for that async group.
	async.Add(1)
	go func() { defer async.Done(); <-asyncGate }()
	requestDone()
	requestDone()
	select {
	case <-drained:
		t.Fatal("async work was overlooked")
	case <-time.After(10 * time.Millisecond):
	}
	close(asyncGate)
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("drain blocked")
	}
}

func TestConcurrentAdmissionAndClose(t *testing.T) {
	w := New(context.Background(), slog.New(slog.DiscardHandler))
	var started, finished atomic.Int32
	var callers sync.WaitGroup
	for range 100 {
		callers.Add(1)
		go func() {
			defer callers.Done()
			w.Go("fixture", func() { started.Add(1); <-w.Context().Done(); finished.Add(1) })
		}()
	}
	w.Close()
	callers.Wait()
	if started.Load() != finished.Load() {
		t.Fatalf("unjoined work: started=%d finished=%d", started.Load(), finished.Load())
	}
}
