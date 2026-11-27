package store

import (
	"context"
	"fmt"
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

func TestDuplicatesSortsPresentCopiesByTitle(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	lib, err := s.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Zulu", "Alpha", "Missing Copy"} {
		item, _, err := s.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		for copy := range 2 {
			seenAt := int64(2)
			if title == "Missing Copy" && copy == 1 {
				seenAt = 1
			}
			file, err := s.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: fmt.Sprintf("/fixture/%s/%d.mkv", title, copy), Size: 1}, 1, seenAt)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.SetFileItems(ctx, file, []string{item}, 1); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := s.MarkMissing(ctx, lib, 2, 2); err != nil {
		t.Fatal(err)
	}

	got, err := s.Duplicates(ctx, lib)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("duplicate groups = %d, want 2", len(got))
	}
	for i, title := range []string{"Alpha", "Zulu"} {
		group := got[i]
		if group.Kind != "copies" || len(group.Items) != 1 || group.Items[0].Title != title || len(group.Files) != 2 {
			t.Fatalf("group %d = %+v, want two copies of %s", i, group, title)
		}
	}
}
