package background

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/schedule"
	"ams/internal/settings"
	"ams/internal/store"
)

func TestUpkeepWeeklyManualAndPersistentResults(t *testing.T) {
	ctx, st, m, _ := fixture(t)
	m.Configure(settings.Settings{DatabaseUpkeepEnabled: true, CacheCleanupEnabled: true})
	calls := 0
	work := func(context.Context, func() bool) (string, error) { calls++; return "Reduced database bytes", nil }
	m.upkeepPass(ctx, "database", work)
	m.upkeepPass(ctx, "database", work)
	if calls != 1 {
		t.Fatalf("weekly task repeated: %d", calls)
	}
	last, err := st.MaintenanceRun(ctx, "database")
	if err != nil || last.LastSuccessAt == 0 || last.Detail != "Reduced database bytes" {
		t.Fatalf("last=%+v %v", last, err)
	}
	next, err := New(ctx, st, schedule.New(2, 5), activity.New())
	if err != nil {
		t.Fatal(err)
	}
	next.Configure(settings.Settings{DatabaseUpkeepEnabled: true})
	next.now = m.now
	next.upkeepPass(ctx, "database", work)
	if calls != 1 {
		t.Fatal("restart forgot last successful pass")
	}
	now := m.now().Add(8 * 24 * time.Hour)
	m.now = func() time.Time { return now }
	m.upkeepPass(ctx, "database", func(context.Context, func() bool) (string, error) {
		calls++
		return "Integrity check did not complete", store.ErrIntegrity
	})
	last, _ = st.MaintenanceRun(ctx, "database")
	if last.Error == "" || last.LastSuccessAt != last.StartedAt-int64(8*24*time.Hour/time.Second) {
		t.Fatalf("failure replaced success: %+v", last)
	}
	for _, s := range m.Snapshot(ctx) {
		if s.Kind == "database" && (s.State != "failed" || s.LastRun == nil || !strings.Contains(s.LastRun.Error, "integrity")) {
			t.Fatalf("failure hidden: %+v", s)
		}
	}
	m.upkeepPass(ctx, "database", work)
	if calls != 2 {
		t.Fatal("failed work retried without backoff")
	}
	now = now.Add(8 * time.Hour)
	if m.Ready("database") {
		t.Fatal("scheduled upkeep outside window")
	}
	wakes := 0
	m.Register("database", func() { wakes++ })
	if err := m.RunNow(ctx, "database"); err != nil {
		t.Fatal(err)
	}
	if err := m.RunNow(ctx, "database"); err != nil {
		t.Fatal(err)
	}
	if wakes != 1 {
		t.Fatalf("Run now not coalesced: %d", wakes)
	}
	m.upkeepPass(ctx, "database", work)
	if calls != 3 || m.Forced("database") {
		t.Fatal("manual run failed or request replayable")
	}
	m.Configure(settings.Settings{})
	if err := m.RunNow(ctx, "database"); err == nil {
		t.Fatal("disabled upkeep admitted")
	}
}

func TestUpkeepYieldsAndReportsInterruptedWork(t *testing.T) {
	ctx, st, m, _ := fixture(t)
	m.Configure(settings.Settings{CacheCleanupEnabled: true})
	m.upkeepPass(ctx, "cache", func(ctx context.Context, _ func() bool) (string, error) {
		m.SetPlayback(func() bool { return true })
		select {
		case <-ctx.Done():
			return "Partial cleanup retained", ctx.Err()
		case <-time.After(2 * time.Second):
			return "", errors.New("did not yield to playback")
		}
	})
	last, _ := st.MaintenanceRun(ctx, "cache")
	if !strings.Contains(last.Error, "Paused") || last.LastSuccessAt != 0 {
		t.Fatalf("paused incorrectly completed: %+v", last)
	}
	m.SetPlayback(nil)
	if err := st.SaveMaintenanceRun(ctx, "cache", store.MaintenanceRun{StartedAt: 100}); err != nil {
		t.Fatal(err)
	}
	called := false
	m.upkeepPass(ctx, "cache", func(context.Context, func() bool) (string, error) { called = true; return "", nil })
	last, _ = st.MaintenanceRun(ctx, "cache")
	if called || !strings.Contains(last.Error, "restart") {
		t.Fatalf("interruption hidden: %+v", last)
	}
}

func TestUpkeepStateErrorsAreReported(t *testing.T) {
	ctx, st, m, _ := fixture(t)
	m.Configure(settings.Settings{DatabaseUpkeepEnabled: true})
	if err := st.SetSetting(ctx, "maintenance_run_database", "{invalid"); err != nil {
		t.Fatal(err)
	}
	if err := m.RunNow(ctx, "database"); err != nil {
		t.Fatal(err)
	}
	m.upkeepPass(ctx, "database", func(context.Context, func() bool) (string, error) {
		t.Error("maintenance started without readable state")
		return "", nil
	})
	_, recent := m.Activity.Snapshot()
	if len(recent) != 1 || recent[0].Error == "" || m.Forced("database") {
		t.Fatalf("failure hidden or request stuck: %+v", recent)
	}
	for _, task := range m.Snapshot(ctx) {
		if task.Kind == "database" && task.State != "failed" {
			t.Fatalf("unavailable status reported as healthy: %+v", task)
		}
	}
}
