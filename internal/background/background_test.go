package background

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/schedule"
	"ams/internal/settings"
	"ams/internal/store"
)

func fixture(t *testing.T) (context.Context, *store.Store, *Manager, int64) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "jobs.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := st.SyncLibrary(ctx, "Jobs", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(ctx, st, schedule.New(2, 5), activity.New())
	if err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return time.Date(2026, 10, 23, 3, 0, 0, 0, time.UTC) }
	m.Configure(settings.Settings{AnalysisEnabled: true, MetadataRefreshEnabled: true, AutoBackupsEnabled: true, PreviewMode: "scheduled", DetectionMode: "scheduled", BackgroundEpoch: 100})
	return ctx, st, m, lib
}
func savedFile(t *testing.T, ctx context.Context, st *store.Store, lib int64, name string, probed int64) store.MediaFile {
	t.Helper()
	f := store.MediaFile{LibraryID: lib, Path: name, Size: 100, ModTime: 1, Streams: []store.Stream{{Index: 0, Kind: "video", Codec: "hevc"}}}
	id, err := st.SaveFile(ctx, &f, probed, 1)
	if err != nil {
		t.Fatal(err)
	}
	f.ID = id
	return f
}
func TestAnalysisDrainsMoreThanOnePageWithoutScanning(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	for i := 0; i < 70; i++ {
		savedFile(t, ctx, st, lib, fmt.Sprintf("%03d.mkv", i), 99)
	}
	count := 0
	m.analysisPass(ctx, func(ctx context.Context, f store.MediaFile) error {
		count++
		if f.ID == 1 {
			return fmt.Errorf("offline fixture")
		}
		return st.SaveFormats(ctx, f)
	})
	if count != 70 {
		t.Fatalf("only processed %d of 70", count)
	}
	pending, done, failed, err := st.BackgroundCounts(ctx, "analysis")
	if err != nil || pending != 0 || done != 69 || failed != 1 {
		t.Fatalf("states=%d/%d/%d %v", pending, done, failed, err)
	}
	count = 0
	m.analysisPass(ctx, func(context.Context, store.MediaFile) error { count++; return nil })
	if count != 0 {
		t.Fatalf("completed/failed files retried immediately: %d", count)
	}
}
func TestRunNowBoundedToExistingBacklogAndDisabledTasks(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	m.now = func() time.Time { return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC) }
	f := savedFile(t, ctx, st, lib, "old.mkv", 99)
	if _, _, ok := m.Begin(ctx, "analysis", "old", f.ID); ok {
		t.Fatal("old analysis ran outside window")
	}
	if err := m.RunNow(ctx, "analysis"); err != nil {
		t.Fatal(err)
	}
	late := savedFile(t, ctx, st, lib, "later.mkv", 99)
	if _, _, ok := m.Begin(ctx, "analysis", "late", late.ID); ok {
		t.Fatal("Run now included a future addition")
	}
	_, finish, ok := m.Begin(ctx, "analysis", "old", f.ID)
	if !ok {
		t.Fatal("manual backlog did not run")
	}
	finish(nil)
	m.CompletePass("analysis")
	if m.Ready("analysis") {
		t.Fatal("Run now became a permanent override")
	}
	m.Configure(settings.Settings{PreviewMode: "off", DetectionMode: "off"})
	if err := m.RunNow(ctx, "analysis"); err == nil {
		t.Fatal("disabled task ran")
	}
	if err := m.RunNow(ctx, "unknown"); err == nil {
		t.Fatal("unknown task accepted")
	}
}
func TestAddedPolicyExistingWorkAndOvernightWindow(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	old := savedFile(t, ctx, st, lib, "old.mkv", 99)
	newFile := savedFile(t, ctx, st, lib, "new.mkv", 101)
	m.Configure(settings.Settings{PreviewMode: "added", DetectionMode: "off", BackgroundEpoch: 100})
	m.Window.Set(23, 4)
	m.now = func() time.Time { return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC) }
	if _, _, ok := m.Begin(ctx, "previews", "old", old.ID); ok {
		t.Fatal("old work used added policy")
	}
	_, finish, ok := m.Begin(ctx, "previews", "new", newFile.ID)
	if !ok {
		t.Fatal("new addition did not run")
	}
	finish(nil)
	m.now = func() time.Time { return time.Date(2026, 10, 24, 2, 0, 0, 0, time.UTC) }
	_, finish, ok = m.Begin(ctx, "previews", "old", old.ID)
	if !ok {
		t.Fatal("overnight maintenance denied")
	}
	finish(nil)
}
func TestForegroundAndSettingsCancelLeaseAndResume(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	f := savedFile(t, ctx, st, lib, "file.mkv", 99)
	var busy atomic.Bool
	m.SetPlayback(busy.Load)
	work, finish, ok := m.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("not admitted")
	}
	if _, _, ok := m.Begin(ctx, "markers", "other", f.ID); ok {
		t.Fatal("two heavy jobs admitted")
	}
	busy.Store(true)
	select {
	case <-work.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("foreground did not cancel optional job")
	}
	finish(work.Err())
	busy.Store(false)
	m.Configure(settings.Settings{AnalysisEnabled: true, PreviewMode: "scheduled", DetectionMode: "off"})
	work, finish, ok = m.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("paused job did not resume")
	}
	m.Configure(settings.Settings{PreviewMode: "off", DetectionMode: "off"})
	select {
	case <-work.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Off did not cancel job")
	}
	finish(work.Err())
}
func TestCompletionRestartAndRevisionChanges(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	f := savedFile(t, ctx, st, lib, "file.mkv", 99)
	_, finish, ok := m.Begin(ctx, "previews", "file", f.ID)
	if !ok {
		t.Fatal("not admitted")
	}
	finish(nil)
	restarted, err := New(ctx, st, m.Window, activity.New())
	if err != nil {
		t.Fatal(err)
	}
	restarted.now = m.now
	restarted.Configure(settings.Settings{PreviewMode: "scheduled"})
	if _, _, ok := restarted.Begin(ctx, "previews", "file", f.ID); ok {
		t.Fatal("completed job repeated after restart")
	}
	f.ModTime = 2
	if _, err := st.SaveFile(ctx, &f, 99, 1); err != nil {
		t.Fatal(err)
	}
	_, finish, ok = restarted.Begin(ctx, "previews", "file", f.ID)
	if !ok {
		t.Fatal("changed revision did not run")
	}
	finish(nil)
}
func TestWindowClosingAndRestoreCancelInFlight(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	f := savedFile(t, ctx, st, lib, "file.mkv", 99)
	work, finish, ok := m.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("not admitted")
	}
	m.Window.Set(4, 5)
	select {
	case <-work.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("closed window did not cancel")
	}
	finish(work.Err())
	m.Window.Set(2, 5)
	work, finish, ok = m.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("job did not resume")
	}
	m.Reset(ctx)
	select {
	case <-work.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("restore did not cancel")
	}
	finish(work.Err())
	_, finish, ok = m.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("restored pending job did not resume")
	}
	finish(nil)
}

func TestFailedRetryAndInterruptedRestart(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	m.Window.Set(0, 0)
	m.now = time.Now
	f := savedFile(t, ctx, st, lib, "file.mkv", 99)
	_, finish, ok := m.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("not admitted")
	}
	finish(fmt.Errorf("temporary failure"))
	if _, _, ok := m.Begin(ctx, "analysis", "file", f.ID); ok {
		t.Fatal("failed job retried without delay")
	}
	m.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	_, finish, ok = m.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("failed job did not retry")
	}
	finish(nil)
	parent, cancel := context.WithCancel(ctx)
	work, finish, ok := m.Begin(parent, "markers", "interrupted", f.ID)
	if !ok {
		t.Fatal("not admitted")
	}
	cancel()
	<-work.Done()
	finish(work.Err())
	restarted, err := New(ctx, st, m.Window, activity.New())
	if err != nil {
		t.Fatal(err)
	}
	restarted.Configure(settings.Settings{DetectionMode: "scheduled"})
	_, finish, ok = restarted.Begin(ctx, "markers", "interrupted", f.ID)
	if !ok {
		t.Fatal("interrupted revision lost after restart")
	}
	finish(nil)
}
func TestMarkerResetAndMissingPreviewArtifactsArePending(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	f := savedFile(t, ctx, st, lib, "file.mkv", 99)
	_, finish, ok := m.Begin(ctx, "markers", "season", f.ID)
	if !ok {
		t.Fatal("not admitted")
	}
	finish(nil)
	if _, _, ok := m.Begin(ctx, "markers", "season", f.ID); ok {
		t.Fatal("negative local detection repeated")
	}
	if err := st.ResetMarkers(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	_, finish, ok = m.Begin(ctx, "markers", "season", f.ID)
	if !ok {
		t.Fatal("explicit marker reset was ignored")
	}
	finish(nil)
	_, finish, ok = m.BeginPreview(ctx, f.ID)
	if !ok {
		t.Fatal("preview not admitted")
	}
	finish(nil)
	// Only the worker's missing-artifact path uses BeginPreview to reopen completion.
	if _, _, ok := m.Begin(ctx, "previews", fmt.Sprint(f.ID), f.ID); ok {
		t.Fatal("completed preview repeated")
	}
	_, finish, ok = m.BeginPreview(ctx, f.ID)
	if !ok {
		t.Fatal("missing artifact could not be rebuilt")
	}
	finish(nil)
}

func TestWaitingWorkerGetsTurnBeforeBacklogContinues(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	f := savedFile(t, ctx, st, lib, "file.mkv", 99)
	_, first, ok := m.Begin(ctx, "analysis", "first", f.ID)
	if !ok {
		t.Fatal("first work denied")
	}
	if _, _, ok := m.Begin(ctx, "markers", "second", f.ID); ok {
		t.Fatal("concurrent work admitted")
	}
	first(nil)
	if _, _, ok := m.Begin(ctx, "analysis", "third", f.ID); ok {
		t.Fatal("analysis bypassed a waiting worker")
	}
	_, second, ok := m.Begin(ctx, "markers", "second", f.ID)
	if !ok {
		t.Fatal("waiting worker not admitted")
	}
	second(nil)
	_, third, ok := m.Begin(ctx, "analysis", "third", f.ID)
	if !ok {
		t.Fatal("backlog did not resume")
	}
	third(nil)
}

func TestCompletedPassRemovesObsoleteWaiter(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	f := savedFile(t, ctx, st, lib, "file.mkv", 99)
	_, first, ok := m.Begin(ctx, "analysis", "first", f.ID)
	if !ok {
		t.Fatal("first work denied")
	}
	if _, _, ok := m.Begin(ctx, "markers", "old-candidate", f.ID); ok {
		t.Fatal("concurrent work admitted")
	}
	first(nil)
	// An online/manual marker update can make the waiting local candidate unnecessary.
	m.CompletePass("markers")
	_, finish, ok := m.Begin(ctx, "analysis", "next", f.ID)
	if !ok {
		t.Fatal("obsolete marker waiter blocked all work")
	}
	finish(nil)
}

func TestMonitorWakesWhenClockWindowOpensWhileServerStaysIdle(t *testing.T) {
	ctx, _, m, _ := fixture(t)
	m.now = time.Now
	hour := time.Now().Hour()
	m.Window.Set((hour+1)%24, (hour+2)%24)
	woke := make(chan struct{}, 4)
	m.Register("markers", func() {
		select {
		case woke <- struct{}{}:
		default:
		}
	})
	run, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); m.Monitor(run) }()
	defer func() { cancel(); <-done }()
	select {
	case <-woke:
	case <-time.After(2 * time.Second):
		t.Fatal("initial idle wake missing")
	}
	m.Window.Set(0, 0)
	select {
	case <-woke:
	case <-time.After(2 * time.Second):
		t.Fatal("opening maintenance window did not wake an idle worker")
	}
}

func TestAnalysisActivityIdentifiesLibraryAndFile(t *testing.T) {
	ctx, st, m, lib := fixture(t)
	f := savedFile(t, ctx, st, lib, "Example.Movie.mkv", 99)
	calls := 0
	m.analysisPass(ctx, func(context.Context, store.MediaFile) error {
		calls++
		running, _ := m.Activity.Snapshot()
		if len(running) != 1 || running[0].Kind != "analysis" || running[0].Title != "Analyzing formats in Jobs" || len(running[0].Current) != 1 {
			t.Fatalf("analysis activity: %+v", running)
		}
		op := running[0].Current[0]
		if op.Subject != "Example.Movie.mkv" || op.Detail != "Example.Movie.mkv" || op.Stage != "Inspecting source formats" {
			t.Fatalf("operation: %+v", op)
		}
		return st.SaveFormats(ctx, f)
	})
	running, recent := m.Activity.Snapshot()
	if calls != 1 || len(running) != 0 || len(recent) != 1 || len(recent[0].Current) != 0 {
		t.Fatalf("analysis cleanup: %d %+v %+v", calls, running, recent)
	}
}
