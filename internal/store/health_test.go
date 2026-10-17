package store

import (
	"context"
	"testing"
)

func TestEmptyTrash(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/m"})
	file := func(path string) int64 {
		id, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Size: 1, ModTime: 1}, 1, 100)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	movie := func(key, path string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: key}, key)
		if err := st.SetFileItems(ctx, file(path), []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		return id
	}
	gone := movie("gone", "/m/gone.mkv")
	here := movie("here", "/m/here.mkv")
	both := movie("both", "/m/both-1.mkv")
	st.SetFileItems(ctx, file("/m/both-2.mkv"), []string{both}, 0)

	// A later scan sees only here.mkv and both-2.mkv.
	for _, p := range []string{"/m/here.mkv", "/m/both-2.mkv"} {
		f, _ := st.GetFileByPath(ctx, p)
		st.TouchFile(ctx, f.ID, 200)
	}
	if _, err := st.MarkMissing(ctx, lib, 200, 300); err != nil {
		t.Fatal(err)
	}

	// A single title with a file still present can't be removed.
	h, _ := st.GetItem(ctx, here)
	if removed, err := st.RemoveGoneItem(ctx, h); err != nil || removed {
		t.Fatalf("removing a title whose file is present: removed=%v err=%v", removed, err)
	}

	n, err := st.EmptyTrash(ctx, lib)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("removed %d titles, want 1", n)
	}
	for id, want := range map[string]bool{gone: false, here: true, both: true} {
		if it, _ := st.GetItem(ctx, id); (it != nil) != want {
			t.Errorf("item %s kept = %v, want %v", id, it != nil, want)
		}
	}
	if f, _ := st.GetFileByPath(ctx, "/m/both-1.mkv"); f != nil {
		t.Error("the missing copy of 'both' should be forgotten")
	}
}
