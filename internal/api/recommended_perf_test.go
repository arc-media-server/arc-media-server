package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ams/internal/store"
)

// recommendedFixture builds a library about the size of a large real one (a few hundred shows
// with their episodes, or a couple of thousand films), with genres, cast and a profile's watch
// history, for timing Recommended. Every title has a present file.
func recommendedFixture(t testing.TB, shows bool) (*Server, *store.Library, *store.User) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	if keep := os.Getenv("AMS_PERF_DIR"); keep != "" { // keep the database to look at afterwards
		dir = filepath.Join(keep, map[bool]string{true: "shows", false: "movies"}[shows])
		os.RemoveAll(dir)
		os.MkdirAll(dir, 0o755)
	}
	st, err := store.Open(ctx, filepath.Join(dir, "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	kind, lkind := "movie", "movies"
	if shows {
		kind, lkind = "series", "shows"
	}
	libID, err := st.SyncLibrary(ctx, "Big", lkind, "tmdb_aired", []string{"/big"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	genres := []string{"Drama", "Comedy", "Crime", "Sci-Fi & Fantasy", "Animation", "Action & Adventure", "Mystery", "Documentary", "Family", "Reality"}
	titles, perTitle := 2000, 0
	if shows {
		titles, perTitle = 276, 30
	}
	file := 0
	present := func(id string) {
		file++
		f := &store.MediaFile{LibraryID: libID, Path: fmt.Sprintf("/big/%d.mkv", file), Size: 1, DurationMS: 2_400_000}
		fid, err := st.SaveFile(ctx, f, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
	}
	var watchedTitles, watchedEpisodes []string
	for i := range titles {
		title := fmt.Sprintf("Title %04d", i)
		id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: libID, Kind: kind, Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		rating := 5 + float64(i%50)/10
		if err := st.UpdateMetadata(ctx, &store.Item{ID: id, Title: title, MetadataState: "matched", Rating: rating, Year: 1980 + i%45, ReleaseDate: fmt.Sprintf("%d-%02d-01", 1980+i%45, 1+i%12)}); err != nil {
			t.Fatal(err)
		}
		// Two or three genres and a cast drawn from a shared pool, so people and genres overlap.
		credits := []store.Credit{}
		for c := range 8 {
			p := (i*7 + c*13) % 400
			credits = append(credits, store.Credit{TMDBID: int64(1000 + p), Name: fmt.Sprintf("Person %d", p), Role: "actor"})
		}
		credits = append(credits, store.Credit{TMDBID: int64(9000 + i%60), Name: fmt.Sprintf("Director %d", i%60), Role: "director"})
		if err := st.SetItemDetails(ctx, id, []string{genres[i%len(genres)], genres[(i*3+1)%len(genres)], genres[(i*7+2)%len(genres)]}, credits); err != nil {
			t.Fatal(err)
		}
		if !shows {
			present(id)
			if i%5 == 0 {
				watchedTitles = append(watchedTitles, id)
			}
			continue
		}
		for e := 1; e <= perTitle; e++ {
			key := fmt.Sprintf("%d:%d", i, e)
			eid, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: libID, Kind: "episode", ParentID: id, Title: key}, key)
			if err != nil {
				t.Fatal(err)
			}
			present(eid)
			// The profile has watched part of one show in four.
			if i%4 == 0 && e <= 12 {
				watchedEpisodes = append(watchedEpisodes, eid)
			}
		}
	}
	if err := st.SetPlayed(ctx, u.ID, append(watchedTitles, watchedEpisodes...), true); err != nil {
		t.Fatal(err)
	}
	lib, err := st.GetLibrary(ctx, libID)
	if err != nil {
		t.Fatal(err)
	}
	return &Server{store: st}, lib, u
}

// TestRecommendedTimingOnALargeLibrary times Recommended for a large TV library and a large film
// library. Building the libraries takes a while, so it runs only with AMS_PERF=1:
//
//	AMS_PERF=1 go test ./internal/api -run RecommendedTiming -v
func TestRecommendedTimingOnALargeLibrary(t *testing.T) {
	if os.Getenv("AMS_PERF") == "" {
		t.Skip("set AMS_PERF=1 to time Recommended on a large library")
	}
	for _, shows := range []bool{true, false} {
		built := time.Now()
		s, lib, u := recommendedFixture(t, shows)
		t.Logf("%s library built in %s", lib.Kind, time.Since(built).Round(time.Millisecond))
		for run := range 3 {
			r := httptest.NewRequest("GET", "/api/libraries/1/recommended", nil)
			r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			started := time.Now()
			var log strings.Builder
			s.log = slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))
			rows, err := s.recommendedRows(r, lib)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("%s run %d: %d rows in %s", lib.Kind, run+1, len(rows), time.Since(started).Round(time.Millisecond))
			if run == 2 {
				t.Log(strings.TrimSpace(log.String()))
			}
		}
	}
}
