package scanner

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/store"
)

func TestStoppedScanKeepsUnvisitedFilesAndRejectsStaleStop(t *testing.T) {
	entered := make(chan struct{}, 32)
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/3/search/movie") {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-r.Context().Done()
			return
		}
		http.Error(w, "unexpected fixture request", 500)
	})
	files := map[string]string{}
	for i := 0; i < 80; i++ {
		files[fmt.Sprintf("Movie %03d (2020).mkv", i)] = "fixture"
	}
	st, scanner, lib, _ := libraryFixture(t, "movies", files)
	scanner.tmdb, scanner.Activity = metadata.NewTMDB("fixture"), activity.New()
	if !scanner.Start(lib) {
		t.Fatal("scan not started")
	}
	t.Cleanup(func() { scanner.Stop(lib, scanner.Status(lib).JobID); scanner.Wait() })
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("provider not reached")
	}
	old := scanner.Status(lib)
	scanner.retryLater(lib)
	scanner.Request(lib)
	scanner.RequestPaths(lib, []string{"queued-folder"})
	if !scanner.Stop(lib, old.JobID) || !scanner.Stop(lib, old.JobID) {
		t.Fatal("stop not idempotent")
	}
	done := make(chan struct{})
	go func() { scanner.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not cancel held provider")
	}
	status := scanner.Status(lib)
	if status.Running || !status.Stopped || status.Error != "" || status.FilesSeen >= len(files) {
		t.Fatalf("incomplete walk not stopped: %+v", status)
	}
	for name := range files {
		file, err := st.GetFileByPath(t.Context(), filepath.Join(scannerRoot(t, st, lib), name))
		if err != nil || file == nil || file.Missing {
			t.Fatalf("unvisited file marked missing: %+v %v", file, err)
		}
	}
	// No queued discovery or automatic retry survives the stop. A new explicit operation is independent.
	if len(scanner.retryWake) != 0 {
		t.Fatal("retry timer survived stop")
	}
	if scanner.DiscoveryPending() {
		t.Fatal("queued discovery survived stop")
	}
	if !scanner.StartOperation(lib, "analyze") {
		t.Fatal("new run not admitted")
	}
	if scanner.Stop(lib, old.JobID) {
		t.Fatal("stale stop accepted for new run")
	}
	scanner.Stop(lib, scanner.Status(lib).JobID)
	scanner.Wait()
}

func scannerRoot(t *testing.T, st *store.Store, lib int64) string {
	t.Helper()
	library, err := st.GetLibrary(t.Context(), lib)
	if err != nil {
		t.Fatal(err)
	}
	return library.Paths[0]
}

func TestManualRefreshDrainsPagesRegardlessOfAgeAndKeepsLocks(t *testing.T) {
	var calls atomic.Int32
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/3/movie/"))
		if err != nil {
			http.Error(w, "unexpected fixture path", 500)
			return
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":%d,"title":"Provider title","overview":"Fresh details","release_date":"2020-01-01"}`, id)
	})
	st, scanner, lib, _ := libraryFixture(t, "movies", nil)
	scanner.tmdb, scanner.Activity = metadata.NewTMDB("fixture"), activity.New()
	ids := []string{}
	for i := 0; i < 205; i++ {
		id, _, err := st.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: fmt.Sprint("Title ", i), TMDBID: int64(1000 + i)}, fmt.Sprint("manual-", i))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
		if err := st.MarkSynced(t.Context(), id); err != nil {
			t.Fatal(err)
		}
		title := fmt.Sprint("Locked ", i)
		if err := st.EditItem(t.Context(), id, store.ItemEdit{Title: &title}); err != nil {
			t.Fatal(err)
		}
	}
	if !scanner.StartOperation(lib, "refresh") {
		t.Fatal("refresh not admitted")
	}
	if scanner.Start(lib) || scanner.Maintain(lib) || scanner.StartOperation(lib, "analyze") {
		t.Fatal("overlapping library work admitted")
	}
	scanner.Wait()
	status := scanner.Status(lib)
	if status.Error != "" || status.Done != 205 || calls.Load() != 205 {
		t.Fatalf("refresh stopped at page/age limit: %+v calls %d", status, calls.Load())
	}
	for _, id := range ids {
		item, err := st.GetItem(t.Context(), id)
		if err != nil || !strings.HasPrefix(item.Title, "Locked ") || item.Overview != "Fresh details" {
			t.Fatalf("lock/details lost: %+v %v", item, err)
		}
	}
}

func TestManualReanalysisStopsFFprobeAndPreservesSavedDetails(t *testing.T) {
	st, scanner, lib, root := libraryFixture(t, "movies", map[string]string{"Example.mkv": "fixture"})
	file, err := st.GetFileByPath(t.Context(), filepath.Join(root, "Example.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	file.DurationMS = 12345
	if _, err := st.SaveFile(t.Context(), file, 1, 9); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "entered")
	t.Setenv("AMS_ACTIVITY_PROBE_HELPER", "1")
	t.Setenv("AMS_ACTIVITY_PROBE_ENTERED", marker)
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	scanner.prober, scanner.Activity = probe.New(executable), activity.New()
	if !scanner.StartOperation(lib, "analyze") {
		t.Fatal("analysis not started")
	}
	t.Cleanup(func() { scanner.Stop(lib, scanner.Status(lib).JobID); scanner.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("probe not reached")
		}
		time.Sleep(10 * time.Millisecond)
	}
	running, _ := scanner.Activity.Snapshot()
	if len(running) != 1 || !running[0].CanStop || !scanner.Activity.Stop(running[0].ID) {
		t.Fatal("analysis not cancellable")
	}
	done := make(chan struct{})
	go func() { scanner.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("probe was not killed")
	}
	saved, err := st.GetFileByPath(context.Background(), file.Path)
	if err != nil || saved.ID != file.ID || saved.DurationMS != 12345 || saved.ProbeError != "" || saved.Missing {
		t.Fatalf("cancelled analysis damaged state: %+v %v", saved, err)
	}
}
