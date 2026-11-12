package scanner

import (
	"context"
	"errors"
	"testing"
)

func TestScopedCompletionRetainsFinishedRunWhenDiscoveryIsQueued(t *testing.T) {
	_, s, lib, _ := libraryFixture(t, "movies", map[string]string{"One.mkv": "fixture"})
	ctx, cancel := context.WithCancel(t.Context())
	s.ctx = ctx
	t.Cleanup(func() { cancel(); s.Wait() })
	completed := make(chan Status, 4)
	s.OnFinish = func(_ int64, snapshot Status) { completed <- snapshot }
	_, finish, err := s.beginScopedWork(ctx, lib, "refresh_series", "Refreshing series")
	if err != nil {
		t.Fatal(err)
	}
	before := s.Status(lib)
	s.Request(lib)
	finish(errors.New("fixture refresh failed"))
	s.Wait()
	close(completed)
	found := false
	for got := range completed {
		// Queued discovery may finish quickly; completion order is not the contract.
		if got.JobID != before.JobID {
			continue
		}
		found = true
		if got.StartedAt != before.StartedAt || got.Operation != "refresh_series" || got.Error != "fixture refresh failed" || got.Running || got.FinishedAt.IsZero() {
			t.Fatalf("completion describes another or unfinished run: before%+v got%+v", before, got)
		}
	}
	if !found {
		t.Fatal("completed scoped run was replaced by queued discovery")
	}
	if got := s.Status(lib); got.JobID <= before.JobID || got.Running {
		t.Fatalf("queued discovery lost: %+v", got)
	}
}
