package store

import (
	"ams/internal/perftest"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// Opt-in evidence harness, not a wall-clock CI gate. Run with ARC_RUN_LIBRARY_BENCH=1.
func TestLargeLibraryMeasurements(t *testing.T) {
	if os.Getenv("ARC_RUN_LIBRARY_BENCH") != "1" {
		t.Skip("opt-in synthetic library measurements")
	}
	for _, shows := range []int{100, 1000} {
		t.Run(fmt.Sprint(shows*100), func(t *testing.T) {
			st, err := Open(t.Context(), filepath.Join(t.TempDir(), "perf.db"), Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			tx, err := st.db.BeginTx(t.Context(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if err = perftest.Seed(t.Context(), tx, 10000, shows, 100); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			if os.Getenv("ARC_BENCH_BURST") == "1" {
				if _, err = st.db.ExecContext(t.Context(), "UPDATE items SET added_at=1700000000+CAST(substr(parent_id,3) AS INTEGER) WHERE kind='episode'"); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = st.db.ExecContext(t.Context(), "ANALYZE"); err != nil {
				t.Fatal(err)
			}
			checks := []struct {
				name string
				call func(context.Context) error
			}{
				{"libraries", func(ctx context.Context) error {
					_, e := st.ListLibraries(ctx)
					if e != nil {
						return e
					}
					_, e = st.LibraryCounts(ctx)
					return e
				}},
				{"movies_title", func(ctx context.Context) error {
					_, _, e := st.ListItems(ctx, ItemQuery{LibraryID: 1, Kind: "movie", Limit: 50})
					return e
				}},
				{"movies_added", func(ctx context.Context) error {
					_, _, e := st.ListItems(ctx, ItemQuery{LibraryID: 1, Kind: "movie", Sort: "added", Limit: 50})
					return e
				}},
				{"series_added", func(ctx context.Context) error {
					_, _, e := st.ListItems(ctx, ItemQuery{LibraryID: 2, Kind: "series", Sort: "added", Limit: 50})
					return e
				}},
				{"movies_filters", func(ctx context.Context) error {
					_, _, e := st.ListItems(ctx, ItemQuery{LibraryID: 1, Kind: "movie", Genre: "Drama", Unwatched: true, UserID: perftest.UserID, Limit: 50})
					return e
				}},
				{"series_unwatched", func(ctx context.Context) error {
					_, _, e := st.ListItems(ctx, ItemQuery{LibraryID: 2, Kind: "series", Unwatched: true, UserID: perftest.UserID, Limit: 50})
					return e
				}},
				{"facets", func(ctx context.Context) error { _, e := st.Facets(ctx, 1, "movie", Access{}); return e }},
				{"recent_episodes", func(ctx context.Context) error { _, e := st.RecentEpisodes(ctx, 2, Access{}, 24); return e }},
				{"continue_watching", func(ctx context.Context) error { _, e := st.InProgress(ctx, perftest.UserID, 24, Access{}); return e }},
				{"watched_series", func(ctx context.Context) error {
					_, e := st.RecentlyWatchedSeries(ctx, perftest.UserID, 24, Access{})
					return e
				}},
				{"series_ordering", func(ctx context.Context) error {
					_, e := st.ListOrderings(ctx, "s-1")
					if e != nil {
						return e
					}
					_, e = st.OrderingEntries(ctx, 1)
					return e
				}},
				{"watchlist", func(ctx context.Context) error { return st.SetWatchlist(ctx, perftest.UserID, "m-1", true) }},
			}
			for _, check := range checks {
				if selected := os.Getenv("ARC_BENCH_QUERY"); selected != "" && selected != check.name {
					continue
				}
				if os.Getenv("ARC_BENCH_EXPLAIN") == "1" && check.name == "recent_episodes" {
					q, args := recentEpisodesQuery(2, Access{}, 24)
					rows, e := st.db.QueryContext(t.Context(), "EXPLAIN QUERY PLAN "+q, args...)
					if e != nil {
						t.Fatal(e)
					}
					for rows.Next() {
						var id, parent, unused int
						var detail string
						if e = rows.Scan(&id, &parent, &unused, &detail); e != nil {
							t.Fatal(e)
						}
						t.Logf("PLAN %d %d %s", id, parent, detail)
					}
					rows.Close()
				}
				times := []float64{}
				failure := ""
				for range 5 {
					ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
					start := time.Now()
					err := check.call(ctx)
					times = append(times, float64(time.Since(start).Microseconds())/1000)
					cancel()
					if err != nil {
						failure = err.Error()
						break
					}
				}
				sort.Float64s(times)
				data, _ := json.Marshal(map[string]any{"episodes": shows * 100, "movies": 10000, "query": check.name, "samples_ms": times, "median_ms": times[len(times)/2], "error": failure})
				t.Log(string(data))
				if failure != "" {
					t.Errorf("%s failed: %s", check.name, failure)
				}
			}
		})
	}
}
