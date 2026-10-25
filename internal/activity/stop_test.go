package activity

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

func TestStopIsOwnedIdempotentAndDistinctFromFailure(t *testing.T) {
	tracker := New()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	task := tracker.Start("scan", "Library")
	var calls atomic.Int32
	task.Cancellable(func() { calls.Add(1); cancel() })
	running, _ := tracker.Snapshot()
	id := running[0].ID
	var group sync.WaitGroup
	for range 20 {
		group.Go(func() {
			if !tracker.Stop(id) {
				t.Error("stop rejected")
			}
		})
	}
	group.Wait()
	if calls.Load() != 1 || ctx.Err() == nil {
		t.Fatal("cancellation was not exactly once")
	}
	task.Finish(ctx.Err())
	_, recent := tracker.Snapshot()
	if len(recent) != 1 || !recent[0].Stopped || recent[0].Stopping || recent[0].CanStop || recent[0].Error != "" {
		t.Fatalf("stopped state: %+v", recent)
	}
	other := tracker.Start("refresh", "Other")
	other.Cancellable(func() { t.Error("old stop reached new task") })
	if !tracker.Stop(id) || tracker.Stop(id+100) {
		t.Fatal("old/unknown stop semantics")
	}
	other.Finish(nil)
}
