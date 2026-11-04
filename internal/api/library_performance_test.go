package api

import (
	"ams/internal/auth"
	"ams/internal/perftest"
	"ams/internal/scanner"
	"ams/internal/settings"
	"ams/internal/store"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func libraryPerformanceServer(t *testing.T, movies, shows, episodes int) (http.Handler, *store.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "api.db")
	st, err := store.Open(t.Context(), path, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	seed, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer seed.Close()
	tx, err := seed.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = perftest.Seed(t.Context(), tx, movies, shows, episodes); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = seed.ExecContext(t.Context(), "ANALYZE"); err != nil {
		t.Fatal(err)
	}
	if err = st.CreateSession(t.Context(), auth.HashToken("fixture-benchmark-token"), perftest.UserID, "password", ""); err != nil {
		t.Fatal(err)
	}
	if _, err = seed.ExecContext(t.Context(), "UPDATE sessions SET last_seen_at=1"); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	sc := scanner.New(t.Context(), st, nil, nil, nil, log)
	h := New(Deps{Store: st, Scanner: sc, Log: log, Settings: func() settings.Settings { return settings.Settings{} }})
	return h, st
}
func fixtureAPICall(ctx context.Context, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://arc/api/v1"+path, strings.NewReader(body)).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer fixture-benchmark-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestLargeLibraryAPIMeasurements(t *testing.T) {
	if os.Getenv("ARC_RUN_LIBRARY_BENCH") != "1" {
		t.Skip("opt-in synthetic library measurements")
	}
	h, _ := libraryPerformanceServer(t, 10000, 1000, 100)
	for _, path := range []string{"/libraries", "/items?library_id=1&kind=movie&sort=added&limit=50", "/items?library_id=1&kind=movie&genre=Drama&unwatched=1&limit=50", "/libraries/2/recent-shows?limit=24", "/home/continue?with_up_next=1", "/series/s-1/seasons"} {
		var times []float64
		for range 5 {
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			start := time.Now()
			w := fixtureAPICall(ctx, h, "GET", path, "")
			times = append(times, float64(time.Since(start).Microseconds())/1000)
			cancel()
			if w.Code != 200 {
				t.Fatalf("%s %d %s", path, w.Code, w.Body.String())
			}
		}
		sort.Float64s(times)
		b, _ := json.Marshal(map[string]any{"route": path, "median_ms": times[len(times)/2], "samples_ms": times})
		t.Log(string(b))
	}
}
func TestAuthenticatedLibraryRequestsDuringCatalogWrites(t *testing.T) {
	movies, shows, episodes := 30, 5, 10
	if os.Getenv("ARC_RUN_LIBRARY_BENCH") == "1" {
		movies, shows, episodes = 10000, 1000, 100
	}
	h, st := libraryPerformanceServer(t, movies, shows, episodes)
	ctx, stop := context.WithTimeout(t.Context(), 10*time.Second)
	defer stop()
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	latencies := make(chan float64, 20)
	for worker := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 20 {
				var err error
				switch worker {
				case 0:
					err = st.TouchFiles(ctx, []int64{1, 2, 3}, int64(i+2))
				case 1:
					err = st.SetItemDetails(ctx, "m-1", []string{"Drama"}, nil)
				case 2:
					start := time.Now()
					w := fixtureAPICall(ctx, h, "PUT", fmt.Sprintf("/items/m-%d/watchlist", i%10+1), `{"on":true}`)
					latencies <- float64(time.Since(start).Microseconds()) / 1000
					if w.Code != 204 {
						err = fmt.Errorf("watchlist status %d: %s", w.Code, w.Body.String())
					}
				}
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	for range 15 {
		w := fixtureAPICall(ctx, h, "GET", "/libraries", "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var libs []struct {
			ID    int64 `json:"id"`
			Count int   `json:"count"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &libs); err != nil || len(libs) != 2 {
			t.Fatal("library request lost catalog", err, w.Body.String())
		}
		for _, lib := range libs {
			want := movies
			if lib.ID == 2 {
				want = shows
			}
			if lib.Count != want {
				t.Fatal("incorrect live counts", libs)
			}
		}
		for _, route := range []string{"/home/continue", "/items?library_id=1&kind=movie"} {
			w = fixtureAPICall(ctx, h, "GET", route, "")
			if w.Code != 200 {
				t.Fatal(route, w.Code, w.Body.String())
			}
		}
	}
	wg.Wait()
	close(latencies)
	var times []float64
	for ms := range latencies {
		times = append(times, ms)
	}
	sort.Float64s(times)
	if len(times) != 20 {
		t.Fatalf("completed %d/20 mutations", len(times))
	}
	b, _ := json.Marshal(map[string]any{"movies": movies, "episodes": shows * episodes, "watchlist_p50_ms": times[10], "watchlist_p95_ms": times[18], "watchlist_max_ms": times[19], "pool": st.DatabaseStats()})
	t.Log(string(b))
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	data, err := st.UserDataFor(ctx, perftest.UserID, []string{"m-1", "m-10"})
	if err != nil || !data["m-1"].Watchlisted || !data["m-10"].Watchlisted {
		t.Fatal("user mutation lost", data, err)
	}
}
