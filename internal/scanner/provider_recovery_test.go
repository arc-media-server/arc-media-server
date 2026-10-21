package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"ams/internal/metadata"
	"ams/internal/store"
)

func TestProviderOutageRecovery(t *testing.T) {
	if os.Getenv("AMS_RECOVERY_CHILD") != "1" {
		var requests atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/movie/348" {
				http.Error(w, "unexpected request", 500)
				return
			}
			if requests.Add(1) == 1 {
				http.Error(w, "provider unavailable", 503)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":348,"title":"Alien","release_date":"1979-05-25","overview":"Recovered description","runtime":117}`)
		}))
		defer srv.Close()
		bin, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		// Provider base URLs are read at process startup. A child also isolates all
		// inherited owner settings and guarantees that no real provider is contacted.
		cmd := exec.Command(bin, "-test.run=^TestProviderOutageRecovery$", "-test.v", "-test.timeout=20s")
		for _, e := range os.Environ() {
			if !strings.HasPrefix(strings.ToUpper(e), "AMS_") {
				cmd.Env = append(cmd.Env, e)
			}
		}
		cmd.Env = append(cmd.Env, "AMS_RECOVERY_CHILD=1", "AMS_TMDB_URL="+srv.URL)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("provider recovery: %v\n%s", err, out)
		}
		if requests.Load() != 2 {
			t.Fatalf("provider requests=%d, want failure then retry", requests.Load())
		}
		return
	}
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Alien", TMDBID: 348}, "tmdb:348")
	if err != nil {
		t.Fatal(err)
	}
	title := "My Alien"
	if err := st.EditItem(ctx, id, store.ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, u.ID, id, 120000, 7200000); err != nil {
		t.Fatal(err)
	}
	s := New(ctx, st, nil, metadata.NewTMDB("test-only"), metadata.NewTVDB(""), slog.New(slog.DiscardHandler))
	s.status[lib] = &Status{}
	if err := s.matchMovies(ctx, lib); err != nil {
		t.Fatal(err)
	}
	it, err := st.GetItem(ctx, id)
	if err != nil || it == nil || it.MetadataState != "pending" || it.Title != title {
		t.Fatalf("outage changed item: %+v %v", it, err)
	}
	if err := s.matchMovies(ctx, lib); err != nil {
		t.Fatal(err)
	}
	it, err = st.GetItem(ctx, id)
	if err != nil || it == nil || it.MetadataState != "matched" || it.Title != title || it.Overview != "Recovered description" {
		t.Fatalf("retry failed: %+v %v", it, err)
	}
	data, err := st.UserDataFor(ctx, u.ID, []string{id})
	if err != nil || data[id].PositionMS != 120000 {
		t.Fatalf("resume lost: %+v %v", data, err)
	}
}
