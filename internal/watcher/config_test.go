package watcher

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestLivePeriodicTimer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var count atomic.Int32
		w := New(func(int64) { count.Add(1) }, slog.New(slog.DiscardHandler))
		w.Configure(Config{Interval: time.Hour}) // no native watcher in a fake-clock bubble
		w.SetLibraries(map[int64][]string{1: {"unused"}})
		go w.Run(ctx)
		synctest.Wait()
		time.Sleep(30 * time.Minute)
		synctest.Wait()
		w.Configure(Config{Partial: true, Interval: time.Hour}) // unchanged interval must not reset
		synctest.Wait()
		time.Sleep(30 * time.Minute)
		synctest.Wait()
		if count.Load() != 1 {
			t.Fatalf("hourly scans: %d", count.Load())
		}
		w.Configure(Config{Interval: 6 * time.Hour})
		synctest.Wait()
		time.Sleep(5 * time.Hour)
		synctest.Wait()
		if count.Load() != 1 {
			t.Fatalf("old timer survived: %d", count.Load())
		}
		time.Sleep(time.Hour)
		synctest.Wait()
		if count.Load() != 2 {
			t.Fatalf("new timer: %d", count.Load())
		}
		w.Configure(Config{})
		synctest.Wait()
		time.Sleep(25 * time.Hour)
		synctest.Wait()
		if count.Load() != 2 {
			t.Fatalf("Off still scans: %d", count.Load())
		}
		w.Configure(Config{Interval: time.Hour})
		synctest.Wait()
		if count.Load() != 2 {
			t.Fatal("saving interval started a scan")
		}
		time.Sleep(time.Hour)
		synctest.Wait()
		if count.Load() != 3 {
			t.Fatalf("re-enabled timer: %d", count.Load())
		}
		cancel()
		synctest.Wait()
	})
}

func TestReenableRequestsOneCatchup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var count atomic.Int32
		w := New(func(int64) { count.Add(1) }, slog.New(slog.DiscardHandler))
		w.Configure(Config{})
		// No roots means no native filesystem goroutine in the fake-clock bubble.
		w.SetLibraries(map[int64][]string{1: nil})
		go w.Run(ctx)
		synctest.Wait()
		w.Configure(Config{Automatic: true, Partial: true})
		synctest.Wait()
		if count.Load() != 1 {
			t.Fatalf("catch-up count: %d", count.Load())
		}
		w.Configure(Config{Automatic: true, Partial: true})
		synctest.Wait()
		time.Sleep(20 * time.Second)
		synctest.Wait()
		if count.Load() != 1 {
			t.Fatal("unchanged settings duplicated catch-up")
		}
		cancel()
		synctest.Wait()
	})
}

func TestChangeControlsAndFailureGuidance(t *testing.T) {
	root := t.TempDir()
	full, partial := 0, 0
	w := New(func(int64) { full++ }, slog.New(slog.DiscardHandler))
	w.libs = map[int64][]string{1: {root}}
	w.roots = map[string]int64{root: 1}
	w.ScanPaths = func(id int64, paths []string) {
		partial++
		if id != 1 || len(paths) != 1 {
			t.Fatal(paths)
		}
	}
	path := filepath.Join(root, "movie.mkv")
	now := time.Now()
	queue := func() { w.handle(fsnotify.Event{Name: path, Op: fsnotify.Write}) }
	queue()
	w.dispatchSettled(now.Add(settle + time.Second))
	if partial != 1 || full != 0 {
		t.Fatalf("default: %d/%d", full, partial)
	}
	w.Configure(Config{Automatic: true}) // partial off
	queue()
	w.dispatchSettled(now.Add(settle + time.Second))
	if full != 1 {
		t.Fatal("partial Off did not request full scan")
	}
	queue()
	w.Configure(Config{}) // discard pending changes
	w.dispatchSettled(now.Add(time.Hour))
	w.watchError(errors.New("lost notifications"))
	if full != 1 || len(w.dirty) != 0 || w.Problem(1) != "" {
		t.Fatal("disabled watcher dispatched work")
	}
	w.Configure(Config{Automatic: true, Partial: true})
	if !w.catchup {
		t.Fatal("re-enable did not retain catch-up intent")
	}
	w.watchError(errors.New("lost notifications"))
	if !strings.Contains(w.Problem(1), "enable scheduled") || full != 2 {
		t.Fatal(w.Problem(1))
	}
	w.Configure(Config{Automatic: true, Interval: time.Hour})
	if !strings.Contains(w.Problem(1), "Scheduled full scans") {
		t.Fatal(w.Problem(1))
	}
}

func TestReturningRootWithoutPeriodicScan(t *testing.T) {
	root := filepath.Join(t.TempDir(), "returning")
	count := 0
	w := New(func(int64) { count++ }, slog.New(slog.DiscardHandler))
	w.Configure(Config{Automatic: true, Partial: true})
	w.watch(map[int64][]string{1: {root}})
	defer w.fs.Close()
	if w.Problem(1) == "" {
		t.Fatal("missing root not reported")
	}
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	w.checkRoots()
	if count != 1 || w.Problem(1) != "" {
		t.Fatalf("returning root: %d %s", count, w.Problem(1))
	}
	w.checkRoots()
	if count != 1 {
		t.Fatal("availability check became a recurring full scan")
	}
}
