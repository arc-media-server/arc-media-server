package subtitles

import (
	"ams/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestReusedFileIDCannotInheritDownloadedSubtitles(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(t.Context(), filepath.Join(dir, "store.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(t.Context(), "Old", "movies", "tmdb_aired", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(dir, "old.mkv")
	if err := os.WriteFile(oldPath, []byte("scratch media"), 0644); err != nil {
		t.Fatal(err)
	}
	oldID, err := st.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: oldPath, Size: 13, ModTime: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	old, err := st.GetFileByPath(t.Context(), oldPath)
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache(filepath.Join(dir, "cache"), func() string { return "" })
	if _, err := cache.SaveDownload(old, "en", false, false, []byte("1\n00:00:00,000 --> 00:00:01,000\nOld dialogue\n")); err != nil {
		t.Fatal(err)
	}
	if len(cache.Tracks(old)) != 1 {
		t.Fatal("original downloaded subtitles unavailable")
	}
	if err := st.DeleteLibrary(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	lib, err = st.SyncLibrary(t.Context(), "New", "movies", "tmdb_aired", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dir, "new.mkv")
	if err := os.WriteFile(newPath, []byte("scratch media"), 0644); err != nil {
		t.Fatal(err)
	}
	newID, err := st.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: newPath, Size: 13, ModTime: 1}, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if newID != oldID {
		t.Fatalf("reuse scenario not reproduced: old=%d new=%d", oldID, newID)
	}
	next, err := st.GetFileByPath(t.Context(), newPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, track := range cache.Tracks(next) {
		if track.Downloaded {
			t.Error("new source inherited another file's downloaded subtitles")
		}
	}
}
