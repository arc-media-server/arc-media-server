package scanner

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/store"
)

func TestDriveRecoveryPreservesIdentityAndResume(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	root := filepath.Join(dir, "media")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Alien (1979).mkv")
	if err := os.WriteFile(path, []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(ctx, filepath.Join(dir, "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	// Seed a previously read file, so these filesystem tests need no real media or FFmpeg.
	fid, err := st.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	s := New(ctx, st, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), slog.New(slog.DiscardHandler))
	s.status[lib] = &Status{}
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	items, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%v err=%v", items, err)
	}
	id := items[0].ID
	title := "My edited Alien"
	if err := st.EditItem(ctx, id, store.ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, u.ID, id, 120000, 7200000); err != nil {
		t.Fatal(err)
	}
	check := func(missing bool) {
		t.Helper()
		f, err := st.GetFileByPath(ctx, path)
		if err != nil || f == nil || f.ID != fid || f.Missing != missing {
			t.Fatalf("file=%+v err=%v", f, err)
		}
		it, err := st.GetItem(ctx, id)
		if err != nil || it == nil || it.Title != title {
			t.Fatalf("item=%+v err=%v", it, err)
		}
		data, err := st.UserDataFor(ctx, u.ID, []string{id})
		if err != nil || data[id].PositionMS != 120000 {
			t.Fatalf("history=%+v err=%v", data, err)
		}
	}
	// Detach and reattach only our synthetic directory, never an owner's media folder.
	offline := filepath.Join(dir, "offline")
	if err := os.Rename(root, offline); err != nil {
		t.Fatal(err)
	}
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	check(false) // An inaccessible root must not hide existing titles.
	// A disconnected mount replaced with a regular file is also unavailable,
	// rather than a successfully scanned empty library.
	if err := os.WriteFile(root, []byte("not a mounted directory"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(offline, root); err != nil {
		t.Fatal(err)
	}
	// An accessible but empty folder represents a genuinely removed file.
	if err := os.Rename(path, filepath.Join(dir, "held.mkv")); err != nil {
		t.Fatal(err)
	}
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	check(true)
	if err := os.Rename(filepath.Join(dir, "held.mkv"), path); err != nil {
		t.Fatal(err)
	}
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	check(false)
	// Cancellation before the sweep cannot mark existing files missing; a fresh scan recovers.
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := s.scan(cancelled, lib); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled scan: %v", err)
	}
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	check(false)
}

func TestWalkInterrupted(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a.mkv", "b.mkv"} {
		if err := os.WriteFile(filepath.Join(root, name), nil, 0644); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	visited := 0
	_, err := walkVideos(ctx, root, slog.New(slog.DiscardHandler), func(string, fs.DirEntry) { visited++; cancel() }, nil)
	if !errors.Is(err, context.Canceled) || visited != 1 {
		t.Fatalf("visited=%d err=%v", visited, err)
	}
}
