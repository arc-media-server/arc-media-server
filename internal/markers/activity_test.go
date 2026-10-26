package markers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ams/internal/activity"
	"ams/internal/store"
)

// A pass with nothing to look up or detect is routine and isn't listed as finished work.
func TestIdlePassLeavesNoActivity(t *testing.T) {
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	m := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.Activity = activity.New()
	if err := m.pass(context.Background()); err != nil {
		t.Fatal(err)
	}
	running, recent := m.Activity.Snapshot()
	if len(running) != 0 || len(recent) != 0 {
		t.Fatalf("idle pass listed running=%+v recent=%+v", running, recent)
	}
}

// find says where an episode's intro came from, or why there isn't one, for the log.
func TestFindReportsSourceAndReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("imdb_id") == "tt0303461" {
			w.Write([]byte(`{"intro":{"start_ms":60000,"end_ms":110000},"recap":null,"outro":null}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	t.Setenv("AMS_INTRODB_URL", srv.URL)
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "TV", "shows", "tvdb_aired", []string{"/tv"})
	if err != nil {
		t.Fatal(err)
	}
	m := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.http = srv.Client()
	file := func(path string) int64 {
		id, err := st.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: path, Size: 1, DurationMS: 2_640_000}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	online := true
	cases := []struct {
		job          store.MarkerJob
		source, want string
	}{
		{store.MarkerJob{FileID: file("/tv/a.mkv"), DurationMS: 2_640_000, SeriesIMDB: "tt0303461", Season: 1, Episode: 1}, "IntroDB", ""},
		{store.MarkerJob{FileID: file("/tv/b.mkv"), DurationMS: 2_640_000, Season: 1, Episode: 2}, "", "show has no IMDb ID, which IntroDB needs"},
		{store.MarkerJob{FileID: file("/tv/c.mkv"), DurationMS: 2_640_000, SeriesIMDB: "tt0303461", Season: -1, Episode: -1}, "", "no aired season and episode number"},
		{store.MarkerJob{FileID: file("/tv/d.mkv"), DurationMS: 2_640_000, Movie: true}, "", "movie credits online need a TheIntroDB key"},
	}
	for _, c := range cases {
		got, err := m.find(ctx, c.job, "", &online)
		if err != nil || got.source != c.source || got.reason != c.want {
			t.Errorf("find(%+v) = %+v, %v; want source %q reason %q", c.job, got, err, c.source, c.want)
		}
	}
	// Found once, the marker is reported as already there on a later check.
	if got, _ := m.find(ctx, cases[0].job, "", &online); got.source != "existing" {
		t.Errorf("recheck = %+v", got)
	}
}
