package store

import (
	"ams/internal/perftest"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func smallPerformanceStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(t.Context(), filepath.Join(t.TempDir(), "fixture.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	tx, err := st.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err = perftest.Seed(t.Context(), tx, 30, 5, 10); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestLibraryReadsAndWatchlistSurviveWriterContention(t *testing.T) {
	st := smallPerformanceStore(t)
	tx, err := st.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(t.Context(), "UPDATE media_files SET last_seen_at=2"); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- st.SetWatchlist(t.Context(), perftest.UserID, "m-1", true) }()
	cancelled, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err = st.SetWatchlist(cancelled, perftest.UserID, "m-2", true); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("queued mutation ignored cancellation", err)
	}
	readCtx, stop := context.WithTimeout(t.Context(), 2*time.Second)
	defer stop()
	for range 3 {
		libs, err := st.ListLibraries(readCtx)
		if err != nil || len(libs) != 2 {
			t.Fatal("libraries disappeared during write", libs, err)
		}
		counts, err := st.LibraryCountsFor(readCtx, libs)
		if err != nil || counts[1] != 30 || counts[2] != 5 {
			t.Fatal("committed counts unavailable", counts, err)
		}
		items, _, err := st.ListItems(readCtx, ItemQuery{LibraryID: 1, Kind: "movie", Limit: 10})
		if err != nil || len(items) != 10 {
			t.Fatal("browse unavailable", err)
		}
	}
	select {
	case err := <-done:
		t.Fatal("write bypassed held transaction", err)
	default:
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	state, err := st.UserDataFor(t.Context(), perftest.UserID, []string{"m-1", "m-2"})
	if err != nil || !state["m-1"].Watchlisted || state["m-2"].Watchlisted {
		t.Fatal("lost or late mutation", state, err)
	}
}

func TestConcurrentScanMetadataAndUserMutationsKeepLibraries(t *testing.T) {
	st := smallPerformanceStore(t)
	if _, err := st.BackgroundJob(t.Context(), "formats", "fixture", "fixture", 1); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithTimeout(t.Context(), 10*time.Second)
	defer stop()
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	jobs := []func(int) error{
		func(i int) error { return st.TouchFiles(ctx, []int64{1, 2, 3, 4, 5}, int64(i+3)) },
		func(i int) error {
			return st.SetItemDetails(ctx, "m-1", []string{"Drama"}, []Credit{{TMDBID: 1, Name: "Fixture Actor", Role: "actor"}})
		},
		func(i int) error { return st.SetBackgroundJob(ctx, "formats", "fixture", "fixture", "running", "") },
		func(i int) error { return st.SetWatchlist(ctx, perftest.UserID, fmt.Sprintf("m-%d", i%20+1), true) },
	}
	for _, job := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 30 {
				if err := job(i); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	for range 15 {
		libs, err := st.ListLibraries(ctx)
		if err != nil || len(libs) != 2 {
			t.Fatal("library read failed", err)
		}
		counts, err := st.LibraryCountsFor(ctx, libs)
		if err != nil || counts[1] != 30 || counts[2] != 5 {
			t.Fatal("library counts changed", counts, err)
		}
		if _, err = st.InProgress(ctx, perftest.UserID, 24, Access{}); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	data, err := st.UserDataFor(ctx, perftest.UserID, []string{"m-1", "m-20"})
	if err != nil || !data["m-1"].Watchlisted || !data["m-20"].Watchlisted {
		t.Fatal("watchlist writes were lost", data, err)
	}
}
