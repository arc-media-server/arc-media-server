package trickplay

import (
	"ams/internal/store"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestReusedFileIDCannotInheritAnotherSourcesPreviews(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("prepared FFmpeg required")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "old.mkv")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=duration=1:size=320x180:rate=25", "-c:v", "libx264", path).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	st, err := store.Open(t.Context(), filepath.Join(dir, "store.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(t.Context(), "Old", "movies", "tmdb_aired", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	oldID, err := st.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix(), DurationMS: 1000}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	m := New(filepath.Join(dir, "previews"), st, func() string { return ffmpeg }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := m.make(context.Background(), store.VideoFile{ID: oldID, Path: path, DurationMS: 1000, Width: 320, Height: 180}); err != nil {
		t.Fatal(err)
	}
	if m.Info(oldID) == nil {
		t.Fatal("original previews unavailable")
	}
	if err := st.DeleteLibrary(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	lib, err = st.SyncLibrary(t.Context(), "New", "movies", "tmdb_aired", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(dir, "new.mkv")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	newID, err := st.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: newPath, Size: info.Size(), ModTime: info.ModTime().Unix(), DurationMS: 1000}, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if newID != oldID {
		t.Fatalf("reuse scenario not reproduced: old=%d new=%d", oldID, newID)
	}
	if m.Info(newID) != nil {
		t.Error("new source inherited deleted file's preview info")
	}
	if _, ok := m.Sheet(newID, 0); ok {
		t.Error("new source can read deleted file's thumbnail sheet")
	}
}
