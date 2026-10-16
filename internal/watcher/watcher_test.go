package watcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLibraryForPath(t *testing.T) {
	sep := string(filepath.Separator)
	w := &Watcher{roots: map[string]int64{
		filepath.Clean("/media/TV"):       1,
		filepath.Clean("/media/TV/Anime"): 2,
		filepath.Clean("/media/Movies"):   3,
	}}
	tests := []struct {
		path string
		want int64
		ok   bool
	}{
		{"/media/TV/Firefly/S01E01.mkv", 1, true},
		{"/media/TV/Anime/Show/E01.mkv", 2, true},
		{"/media/Movies", 3, true},
		{"/media/TV Extras/x.mkv", 0, false}, // shares a prefix with /media/TV but isn't in it
		{"/other/x.mkv", 0, false},
	}
	for _, tt := range tests {
		got, ok := w.library(filepath.Clean(tt.path))
		if got != tt.want || ok != tt.ok {
			t.Errorf("library(%q) = %d, %v; want %d, %v (sep %q)", tt.path, got, ok, tt.want, tt.ok, sep)
		}
	}
}

func TestScansAfterChangesSettle(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var scans []int64
	w := New(func(id int64) {
		mu.Lock()
		scans = append(scans, id)
		mu.Unlock()
	}, slog.New(slog.DiscardHandler))
	w.SetLibraries(map[int64][]string{7: {root}})
	go w.Run(ctx)
	time.Sleep(300 * time.Millisecond) // let the watch start

	// A new folder with a file in it, as when a season is copied in.
	season := filepath.Join(root, "Show", "Season 01")
	if err := os.MkdirAll(season, 0o755); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(season, "Show - S01E01.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(settle + 5*time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(scans)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(scans) != 1 || scans[0] != 7 {
		t.Fatalf("scans = %v, want one scan of library 7", scans)
	}
}
