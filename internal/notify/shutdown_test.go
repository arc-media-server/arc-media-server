package notify

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ams/internal/store"
)

type blockingDelivery struct {
	entered, canceled, release chan struct{}
	calls                      atomic.Int32
	once                       sync.Once
}

func (b *blockingDelivery) RoundTrip(r *http.Request) (*http.Response, error) {
	b.calls.Add(1)
	b.once.Do(func() { close(b.entered) })
	<-r.Context().Done()
	close(b.canceled)
	<-b.release
	return nil, r.Context().Err()
}

func TestCloseCancelsAndJoinsDeliveriesBeforeStoreClose(t *testing.T) {
	st, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h := store.Webhook{Name: "Fixture", URL: "https://fixture.invalid/hook", Events: []string{ServerProblem}}
	if err := st.SaveWebhook(t.Context(), &h); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	n := NewWithContext(t.Context(), st, slog.New(slog.NewTextHandler(&logs, nil)))
	transport := &blockingDelivery{entered: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	defer func() { release.Do(func() { close(transport.release) }); n.Close() }()
	n.http = &http.Client{Transport: transport}
	n.Problem("Fixture")
	select {
	case <-transport.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("delivery did not start")
	}
	done := make(chan struct{})
	go func() { n.Close(); close(done) }()
	select {
	case <-transport.canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("delivery did not inherit shutdown")
	}
	select {
	case <-done:
		t.Fatal("Close returned before delivery unwound")
	case <-time.After(20 * time.Millisecond):
	}
	release.Do(func() { close(transport.release) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not join delivery")
	}
	n.Problem("Not admitted")
	if transport.calls.Load() != 1 {
		t.Fatal("closed notifier admitted another delivery")
	}
	if bytes.Contains(logs.Bytes(), []byte("WARN")) {
		t.Fatalf("normal shutdown logged as failure: %s", logs.String())
	}
	root, cancel := context.WithCancel(t.Context())
	cancel()
	stopped := NewWithContext(root, st, slog.Default())
	defer stopped.Close()
	stopped.Problem("Not admitted")
	if !errors.Is(stopped.ctx.Err(), context.Canceled) {
		t.Fatal("root was not inherited")
	}
}
