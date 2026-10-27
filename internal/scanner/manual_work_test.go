package scanner

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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

// Analyze on a show rereads only its episodes' files; on an episode, only that episode's.
func TestItemAnalysisRereadsOnlyThatTitle(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	clip := filepath.Join(t.TempDir(), "clip.mkv")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=size=64x48:rate=10:duration=1",
		"-c:v", "libx264", "-preset", "ultrafast", clip).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg: %v %s", err, out)
	}
	video, err := os.ReadFile(clip)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	// The fixture saves the files as already read but without details, as an older scan might.
	st, s, lib, root := libraryFixture(t, "shows", map[string]string{
		"Alpha/Season 01/Alpha S01E01.mkv": string(video),
		"Alpha/Season 01/Alpha S01E02.mkv": string(video),
		"Beta/Season 01/Beta S01E01.mkv":   string(video),
	})
	s.prober, s.Activity = probe.New(ffprobe), activity.New()
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	duration := func(name string) int64 {
		f, err := st.GetFileByPath(ctx, filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || f == nil {
			t.Fatalf("%s: %+v %v", name, f, err)
		}
		return f.DurationMS
	}
	shows, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	if err != nil || len(shows) != 2 {
		t.Fatalf("shows=%v err=%v", shows, err)
	}
	byTitle := map[string]store.Item{}
	for _, sh := range shows {
		byTitle[sh.Title] = sh
	}
	run := func(itemID string) {
		t.Helper()
		if !s.StartItemAnalysis(lib, itemID) {
			t.Fatal("analysis not started")
		}
		s.Wait()
		if st := s.Status(lib); st.Error != "" || st.Running {
			t.Fatalf("analysis status: %+v", st)
		}
	}

	run(byTitle["Alpha"].ID)
	if duration("Alpha/Season 01/Alpha S01E01.mkv") < 900 || duration("Alpha/Season 01/Alpha S01E02.mkv") < 900 {
		t.Fatal("show's episodes not reread")
	}
	if duration("Beta/Season 01/Beta S01E01.mkv") != 0 {
		t.Fatal("another show was reread")
	}
	_, recent := s.Activity.Snapshot()
	if len(recent) == 0 || recent[0].Title != "Analyzing Alpha" || recent[0].Detail != "2 analysed · 0 failed" {
		t.Fatalf("activity: %+v", recent)
	}

	episodes, err := st.Children(ctx, byTitle["Beta"].ID)
	if err != nil || len(episodes) != 1 {
		t.Fatalf("episodes=%v err=%v", episodes, err)
	}
	run(episodes[0].ID)
	if duration("Beta/Season 01/Beta S01E01.mkv") < 900 {
		t.Fatal("episode not reread")
	}
	// Overlapping library work waits for the analysis, and analysis waits for a scan.
	if !s.Start(lib) {
		t.Fatal("scan not started")
	}
	if s.StartItemAnalysis(lib, byTitle["Alpha"].ID) {
		t.Fatal("analysis admitted during a scan")
	}
	s.Wait()
}
