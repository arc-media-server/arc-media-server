package scanner

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/store"
)

// Run this test binary as a portable held ffprobe process, before testing parses
// ffprobe's flags. No shell, real media or installed helper executable is needed.
func init() {
	if os.Getenv("AMS_ACTIVITY_PROBE_HELPER") != "1" {
		return
	}
	if err := os.WriteFile(os.Getenv("AMS_ACTIVITY_PROBE_ENTERED"), []byte("entered"), 0600); err != nil {
		os.Exit(2)
	}
	for {
		time.Sleep(10 * time.Millisecond)
	} // The scanner's cancelled context kills this child.
}

func TestActivityShowsActualInspectionUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Example (2020).mkv": "fixture"})
	f, err := st.GetFileByPath(ctx, filepath.Join(root, "Example (2020).mkv"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveFile(ctx, f, 0, 1); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "entered")
	t.Setenv("AMS_ACTIVITY_PROBE_HELPER", "1")
	t.Setenv("AMS_ACTIVITY_PROBE_ENTERED", marker)
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	s.prober, s.ctx, s.Activity = probe.New(bin), ctx, activity.New()
	s.Start(lib)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			s.Wait()
			t.Fatal("probe not reached")
		}
		time.Sleep(10 * time.Millisecond)
	}
	running, _ := s.Activity.Snapshot()
	if len(running) != 1 || len(running[0].Current) != 1 {
		cancel()
		s.Wait()
		t.Fatalf("inspection: %+v", running)
	}
	op := running[0].Current[0]
	if op.Subject != "Example" || op.Stage != "Inspecting file" || !strings.Contains(op.Detail, "Example (2020).mkv") || strings.Contains(op.Detail, root) {
		cancel()
		s.Wait()
		t.Fatalf("inspection identity: %+v", op)
	}
	cancel()
	s.Wait()
	running, recent := s.Activity.Snapshot()
	if len(running) != 0 || len(recent) != 1 || len(recent[0].Current) != 0 || recent[0].Error != "Stopped before it finished" {
		t.Fatalf("cancelled activity: %+v %+v", running, recent)
	}
}

func TestActivityReportsConcurrentMatchingAndCachedRescan(t *testing.T) {
	entered, release := make(chan struct{}, 2), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	var requests atomic.Int32
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		entered <- struct{}{}
		<-release
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%s,"title":"Matched movie","release_date":"2020-01-01"}`, strings.TrimPrefix(r.URL.Path, "/3/movie/"))
	})
	ctx := context.Background()
	st, s, lib, _ := libraryFixture(t, "movies", map[string]string{"First (2020).mkv": "first", "Second (2020).mkv": "second"})
	s.Activity = activity.New()
	s.Start(lib)
	s.Wait()
	items, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(items) != 2 {
		t.Fatalf("items: %v %v", items, err)
	}
	for index, it := range items {
		if err := st.AdoptIDs(ctx, it.ID, int64(101+index), 0, ""); err != nil {
			t.Fatal(err)
		}
	}
	s.tmdb = metadata.NewTMDB("local-fixture")
	if !s.Start(lib) {
		t.Fatal("scan not started")
	}
	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			unblock()
			s.Wait()
			t.Fatal("provider not reached")
		}
	}
	running, _ := s.Activity.Snapshot()
	if len(running) != 1 || len(running[0].Current) != 2 || running[0].Total != 0 {
		unblock()
		s.Wait()
		t.Fatalf("live scan: %+v", running)
	}
	names := map[string]bool{}
	for _, op := range running[0].Current {
		if op.Stage != "Matching movie" || op.Detail != "Looking up metadata on TMDB" {
			unblock()
			s.Wait()
			t.Fatalf("operation: %+v", op)
		}
		names[op.Subject] = true
	}
	if !names["First (2020)"] || !names["Second (2020)"] {
		unblock()
		s.Wait()
		t.Fatalf("titles: %v", names)
	}
	unblock()
	s.Wait()
	s.Start(lib)
	s.Wait()
	status := s.Status(lib)
	running, recent := s.Activity.Snapshot()
	if len(running) != 0 || len(recent[0].Current) != 0 || status.FilesReused != 2 || status.FilesProbed != 0 || requests.Load() != 2 {
		t.Fatalf("rescan repeated work/status: %+v requests=%d recent=%+v", status, requests.Load(), recent)
	}
	if !strings.Contains(recent[0].Detail, "2 reused saved details") {
		t.Fatalf("summary: %s", recent[0].Detail)
	}
}

func TestActivityInspectionErrorsAndUnrecognisedSummary(t *testing.T) {
	ctx := context.Background()
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Example (2020).mkv": "fixture"})
	s.Activity = activity.New()
	file, err := st.GetFileByPath(ctx, filepath.Join(root, "Example (2020).mkv"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveFile(ctx, file, 0, 1); err != nil {
		t.Fatal(err)
	}
	s.prober = probe.New("nonexistent-activity-ffprobe")
	s.Start(lib)
	s.Wait()
	status := s.Status(lib)
	if status.Error != "" || status.FilesProbed != 1 || status.ProbeErrors != 1 || status.FilesReused != 0 {
		t.Fatalf("status: %+v", status)
	}
	running, recent := s.Activity.Snapshot()
	if len(running) != 0 || len(recent[0].Current) != 0 || !strings.Contains(recent[0].Detail, "1 inspection error") {
		t.Fatalf("summary: %+v", recent)
	}
	title, err := st.FileActivityTitle(ctx, file.ID)
	if err != nil || title != "Example" {
		t.Fatalf("file title: %q %v", title, err)
	}
	summary := (Status{Unrecognised: 2, MetadataErrors: 1}).summary()
	if !strings.Contains(summary, "2 unrecognised files") || !strings.Contains(summary, "1 metadata update deferred") {
		t.Fatal(summary)
	}
}

func TestMaintenanceListsOnlyRunsThatDidSomething(t *testing.T) {
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/3/search/") {
			fmt.Fprint(w, `{"results":[{"id":101,"title":"Known","release_date":"2020-01-01"}]}`)
			return
		}
		fmt.Fprint(w, `{"id":101,"title":"Known","release_date":"2020-01-01"}`)
	})
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Known (2020).mkv": "known"})
	s.tmdb = metadata.NewTMDB("local-fixture")
	s.Activity = activity.New()
	s.Start(lib)
	s.Wait()
	items, _, err := st.ListItems(context.Background(), store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(items) != 1 || items[0].MetadataState != "matched" {
		t.Fatalf("fixture not matched: %+v %v", items, err)
	}
	refreshes := func() int {
		_, recent := s.Activity.Snapshot()
		n := 0
		for _, task := range recent {
			if task.Kind == "refresh" && strings.HasPrefix(task.Title, "Updating details in ") {
				n++
			}
		}
		return n
	}
	// Nothing is due straight after matching: the routine check leaves no finished entry.
	s.Maintain(lib)
	s.Wait()
	if n := refreshes(); n != 0 {
		t.Fatalf("idle maintenance listed %d finished refreshes", n)
	}
	// A stale title is refreshed and that run is listed.
	fixtureSQL(t, root, `UPDATE items SET synced_at = 0 WHERE kind = 'movie'`)
	s.Maintain(lib)
	s.Wait()
	if n := refreshes(); n != 1 {
		t.Fatalf("maintenance that refreshed a title listed %d entries", n)
	}
}
