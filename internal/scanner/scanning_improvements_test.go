package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ams/internal/metadata"
	"ams/internal/store"
)

func localProviders(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	old := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = old; transport.CloseIdleConnections() })
}

func TestTargetedMovieScanDoesNotSweepOtherFolders(t *testing.T) {
	ctx := context.Background()
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{
		"Écho (2000)/Écho (2000).mkv":         "a",
		"Écho (2000) extras/Other (2000).mkv": "b",
	})
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(root, "Écho (2000)", "Écho (2000).mkv")
	b := filepath.Join(root, "Écho (2000) extras", "Other (2000).mkv")
	fa, _ := st.GetFileByPath(ctx, a)
	fb, _ := st.GetFileByPath(ctx, b)
	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(b); err != nil {
		t.Fatal(err)
	}
	// Restore source bytes for the fixture's final media-integrity check.
	t.Cleanup(func() { os.WriteFile(a, []byte("a"), 0644); os.WriteFile(b, []byte("b"), 0644) })
	if err := s.scanPaths(ctx, lib, []string{a}); err != nil {
		t.Fatal(err)
	}
	afterA, _ := st.GetFileByPath(ctx, a)
	afterB, _ := st.GetFileByPath(ctx, b)
	if !afterA.Missing || afterB.Missing || afterA.ID != fa.ID || afterB.ID != fb.ID {
		t.Fatalf("scoped sweep a=%+v b=%+v", afterA, afterB)
	}
	if err := s.scanPaths(ctx, lib, []string{filepath.Join(t.TempDir(), "outside.mkv")}); err == nil {
		t.Fatal("out-of-library scope accepted")
	}
}

func TestTargetedTVScanPreservesShowIdentity(t *testing.T) {
	ctx := context.Background()
	st, s, lib, root := libraryFixture(t, "shows", map[string]string{
		"Show A/Season 01/Show A S01E01.mkv": "one",
		"Show B/Season 01/Show B S01E01.mkv": "two",
	})
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	shows, _, _ := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	var id string
	for _, show := range shows {
		if show.Title == "Show A" {
			id = show.ID
		}
	}
	kids, _ := st.Children(ctx, id)
	before := kids[0].ID
	user, _ := st.CreateUser(ctx, "Fixture", false, "blue", "", "")
	st.ReportProgress(ctx, user.ID, before, 70000, 120000)
	path := filepath.Join(root, "Show A", "Season 02", "Show A S02E01.mkv")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	st.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}, 1, 1)
	s.status[lib] = &Status{}
	if err := s.scanPaths(ctx, lib, []string{path}); err != nil {
		t.Fatal(err)
	}
	if s.Status(lib).FilesSeen != 2 {
		t.Fatalf("scanned unrelated show: %+v", s.Status(lib))
	}
	kids, _ = st.Children(ctx, id)
	if len(kids) != 2 {
		t.Fatalf("show identity changed: %v", kids)
	}
	data, _ := st.UserDataFor(ctx, user.ID, []string{before})
	if data[before].PositionMS != 70000 {
		t.Fatal("resume changed")
	}
}

func TestProgressiveTVCombinesSplitShowFolders(t *testing.T) {
	ctx := context.Background()
	st, s, lib, _ := libraryFixture(t, "shows", map[string]string{
		"Example (2020)/Season 01/Example S01E01.mkv": "one",
		"Example.2020/Season 02/Example S02E01.mkv":   "two",
	})
	for range 2 {
		if err := s.scan(ctx, lib); err != nil {
			t.Fatal(err)
		}
		shows, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
		if err != nil || len(shows) != 1 {
			t.Fatalf("split folders duplicated show: %v %v", shows, err)
		}
		children, err := st.Children(ctx, shows[0].ID)
		if err != nil || len(children) != 2 {
			t.Fatalf("split folders lost episodes: %v %v", children, err)
		}
		for _, episode := range children {
			files, err := st.ItemFiles(ctx, episode.ID)
			if err != nil || len(files) != 1 {
				t.Fatalf("split folders lost file links: %v %v", files, err)
			}
		}
	}
}

func TestTVThumbAdditionAndRemovalAreDetectedOnScopedRescan(t *testing.T) {
	ctx := context.Background()
	st, s, lib, root := libraryFixture(t, "shows", map[string]string{
		"Example/Season 01/Example S01E01.mkv": "one",
		"Example/Season 01/Example S01E02.mkv": "two",
	})
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	shows, _, _ := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	children, _ := st.Children(ctx, shows[0].ID)
	thumb := filepath.Join(root, "Example", "Season 01", "Example S01E01-thumb.jpg")
	if err := os.WriteFile(thumb, []byte("fixture artwork"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, present := range []bool{true, false} {
		if !present {
			if err := os.Remove(thumb); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.scanPaths(ctx, lib, []string{thumb}); err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, child := range children {
			item, err := st.GetItem(ctx, child.ID)
			if err != nil || item == nil {
				t.Fatalf("episode identity lost: %v", err)
			}
			if item.LocalPoster == thumb {
				found++
			} else if item.LocalPoster != "" {
				t.Fatalf("unexpected sibling artwork: %s", item.LocalPoster)
			}
		}
		want := 0
		if present {
			want = 1
		}
		if found != want {
			t.Fatalf("present=%t: %d episodes with thumb, want %d", present, found, want)
		}
	}
}

func TestLargeChangedFolderBatchFallsBackToFullScan(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for i := range 150 {
		dir := filepath.Join(root, fmt.Sprint(i))
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, filepath.Join(dir, "movie.mkv"))
	}
	folders, err := scanFolders(&store.Library{Paths: []string{root}}, paths)
	if err != nil || len(folders) != 1 || folders[0].path != root {
		t.Fatalf("large batch did not reconcile safely: %v %v", folders, err)
	}
}

func TestChangedFolderQueuedDuringScanCompletionIsRetained(t *testing.T) {
	_, s, lib, root := libraryFixture(t, "movies", map[string]string{
		"One/One.mkv": "one", "Two/Two.mkv": "two",
	})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	t.Cleanup(func() { cancel(); s.Wait() })
	done := make(chan int, 2)
	s.OnFinish = func(id int64) {
		seen := s.Status(id).FilesSeen
		if seen == 2 {
			s.RequestPaths(id, []string{filepath.Join(root, "One", "One.mkv")})
		}
		done <- seen
	}
	s.Start(lib)
	for _, want := range []int{2, 1} {
		select {
		case got := <-done:
			if got != want {
				t.Fatalf("queued scan saw %d files, want %d", got, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("changed folder lost during scan completion")
		}
	}
	s.Wait()
}

func TestMoviesMatchBeforeDiscoveryCompletes(t *testing.T) {
	ctx := context.Background()
	var scanner *Scanner
	var libID int64
	var observed atomic.Int32
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		if scanner.Status(libID).FilesSeen < 80 {
			observed.Store(1)
		}
		var id int
		fmt.Sscanf(r.URL.Path, "/3/movie/%d", &id)
		fmt.Fprintf(w, `{"id":%d,"title":"Movie %d","release_date":"2000-01-01"}`, id, id)
	})
	files := map[string]string{}
	for i := range 80 {
		files[fmt.Sprintf("Movie %03d (2000) {tmdb-%d}/Movie %03d (2000) {tmdb-%d}.mkv", i, i+1, i, i+1)] = "seed"
	}
	_, scanner, libID, _ = libraryFixture(t, "movies", files)
	scanner.tmdb = metadata.NewTMDB("local-fixture")
	if err := scanner.scan(ctx, libID); err != nil {
		t.Fatal(err)
	}
	if observed.Load() != 1 {
		t.Fatal("matching waited for complete discovery")
	}
}

func TestEssentialTVPlayableBeforeOrdersAndAutomaticRecovery(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var groups, details, seasons atomic.Int32
	localProviders(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/3/search/tv":
			fmt.Fprint(w, `{"results":[{"id":303,"name":"Example","first_air_date":"2020-01-01"}]}`)
		case "/3/tv/303":
			if r.URL.Query().Get("append_to_response") == "season/1" {
				seasons.Add(1)
				fmt.Fprint(w, `{"season/1":{"episodes":[{"id":100,"name":"Pilot","season_number":1,"episode_number":1,"air_date":"2020-01-01"}]}}`)
			} else {
				details.Add(1)
				fmt.Fprint(w, `{"id":303,"name":"Example","first_air_date":"2020-01-01","seasons":[{"season_number":1}],"episode_groups":{"results":[{"id":"story","name":"Story"}]}}`)
			}
		case "/3/tv/episode_group/story":
			n := groups.Add(1)
			if n == 1 {
				close(entered)
				select {
				case <-release:
				case <-r.Context().Done():
					return
				}
			}
			if n <= 4 {
				http.Error(w, "temporary alternate-order failure", 503)
				return
			}
			fmt.Fprint(w, `{"id":"story","groups":[{"order":0,"episodes":[{"id":100,"name":"Pilot","season_number":1,"episode_number":1,"order":0,"air_date":"2020-01-01"}]}]}`)
		default:
			t.Errorf("unexpected fixture request %s", r.URL.Path)
			http.Error(w, "unexpected", 404)
		}
	})
	st, s, lib, _ := libraryFixture(t, "shows", map[string]string{"Example (2020)/Example S01E01.mkv": "seed"})
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.retryBase = 50 * time.Millisecond
	s.tmdb = metadata.NewTMDB("fixture")
	s.log = slog.New(slog.DiscardHandler)
	t.Cleanup(func() { cancel(); s.Wait() })
	s.Start(lib)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("alternate-order pass not reached")
	}
	shows, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	if err != nil || len(shows) != 1 || shows[0].MetadataState != "matched" {
		close(release)
		t.Fatalf("essential show unavailable: %v %v", shows, err)
	}
	show := shows[0].ID
	kids, _ := st.Children(ctx, show)
	if len(kids) != 1 {
		close(release)
		t.Fatalf("essential episode unavailable: %v", kids)
	}
	id := kids[0].ID
	files, _ := st.ItemFiles(ctx, id)
	if len(files) != 1 {
		close(release)
		t.Fatal("essential episode is not playable")
	}
	fid := files[0].ID
	user, _ := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	st.ReportProgress(ctx, user.ID, id, 70000, 120000)
	orders, _ := st.ListOrderings(ctx, show)
	custom, err := st.CopyOrdering(ctx, &orders[0], "My order")
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	st.SetEntry(ctx, custom, id, 3, 7)
	st.SetDisplayOrdering(ctx, show, user.ID, custom)
	close(release)
	deadline := time.Now().Add(10 * time.Second)
	for {
		synced, _ := st.SyncedAt(ctx, show)
		if synced > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("automatic background retry did not finish alternate orders")
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.Wait()
	kids, _ = st.Children(ctx, show)
	files, _ = st.ItemFiles(ctx, id)
	if len(kids) != 1 || kids[0].ID != id || len(files) != 1 || files[0].ID != fid {
		t.Fatal("episode/file identity changed")
	}
	data, _ := st.UserDataFor(ctx, user.ID, []string{id})
	if data[id].PositionMS != 70000 {
		t.Fatal("resume lost")
	}
	entries, _ := st.OrderingEntries(ctx, custom)
	if len(entries) != 1 || entries[0].Season != 3 || entries[0].Episode != 7 {
		t.Fatalf("custom placement lost: %v", entries)
	}
	if details.Load() != 1 || seasons.Load() != 1 || groups.Load() != 5 {
		t.Fatalf("cached essential data/refetch counts: details=%d seasons=%d groups=%d", details.Load(), seasons.Load(), groups.Load())
	}
	orders, _ = st.ListOrderings(ctx, show)
	found := false
	for _, o := range orders {
		if strings.HasPrefix(o.Source, "tmdb_group") {
			found = true
		}
	}
	if !found {
		t.Fatal("alternate ordering missing")
	}
}
