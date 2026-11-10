package trickplay

import (
	"io"
	"log/slog"
	"sync"
	"testing"
)

func TestRepeatedUnavailablePreviewRequestsStayDeduplicated(t *testing.T) {
	m := New(t.TempDir(), nil, func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 100 {
				m.Request(42)
			}
		})
	}
	wg.Wait()
	if len(m.urgent) != 1 || m.urgent[0] != 42 {
		t.Fatalf("disabled/unavailable previews accumulated %d queued requests for one file", len(m.urgent))
	}
	m.Request(7)
	m.Request(42)
	if len(m.urgent) != 2 || m.urgent[0] != 42 || m.urgent[1] != 7 {
		t.Fatalf("watching a title again must move it to front once: %v", m.urgent)
	}
}

func TestDistinctUnavailablePreviewRequestsAreBounded(t *testing.T) {
	m := New(t.TempDir(), nil, func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for id := int64(1); id <= 1000; id++ {
		m.Request(id)
	}
	if len(m.urgent) > 256 {
		t.Fatalf("urgent queue grew to %d", len(m.urgent))
	}
	if len(m.urgent) == 0 || m.urgent[0] != 1000 {
		t.Fatal("newest viewer request did not take priority")
	}
	m.Request(0)
	m.Request(-1)
	if m.urgent[0] != 1000 {
		t.Fatal("invalid file id took queue priority")
	}
}
