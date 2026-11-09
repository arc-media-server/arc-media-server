package upkeep

import (
	"ams/internal/store"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCleanupKeepsCurrentSourceAndRemovesObsoleteRevision(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "cache.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(t.Context(), "Offline", "movies", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := st.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: "/fixture/movie.mkv", Size: 1, ModTime: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	file, err := st.GetFileByPath(t.Context(), "/fixture/movie.mkv")
	if err != nil {
		t.Fatal(err)
	}
	current, stale := "v2-"+file.CacheKey, "v2-"+strings.Repeat("a", 32)
	if current == stale {
		stale = "v2-" + strings.Repeat("b", 32)
	}
	prefix := strconv.FormatInt(id, 10)
	keep := []string{filepath.Join("trickplay", prefix, current, "0.jpg"), filepath.Join("subtitles", "downloaded", prefix, current, "en.1.srt")}
	remove := []string{filepath.Join("trickplay", prefix, stale, "0.jpg"), filepath.Join("subtitles", "downloaded", prefix, stale, "en.1.srt")}
	old := time.Now().Add(-2 * time.Hour)
	for _, name := range append(append([]string{}, keep...), remove...) {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("asset"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	// Temporarily missing media still owns its current assets.
	if _, err := st.MarkMissing(t.Context(), lib, 2, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := CleanCaches(t.Context(), st, dir, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("current/offline source asset removed: %s", name)
		}
	}
	for _, name := range remove {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("obsolete revision retained: %s err=%v", name, err)
		}
	}
}
