package store

import (
	"context"
	"testing"
)

func TestMergeAndSplit(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	user, _ := st.CreateUser(ctx, "Jo", true, "blue", "x", "")
	add := func(title, path string) (string, int64) {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, "title:"+title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		return id, fid
	}
	keep, _ := add("Blade Runner", "/m/Blade Runner (1982).mkv")
	cut, cutFile := add("Blade Runner Final Cut", "/m/Blade Runner Final Cut (1982).mkv")
	st.SetPlayed(ctx, user.ID, []string{cut}, true)

	if err := st.MergeItems(ctx, keep, []string{cut}); err != nil {
		t.Fatal(err)
	}
	if it, _ := st.GetItem(ctx, cut); it != nil {
		t.Error("the merged title is still there")
	}
	if files, _ := st.ItemFiles(ctx, keep); len(files) != 2 {
		t.Errorf("the kept title has %d files, want 2", len(files))
	}
	if over, _ := st.FileItemOverride(ctx, cutFile); over != keep {
		t.Errorf("override = %q, want the kept title", over)
	}
	ud, _ := st.UserDataFor(ctx, user.ID, []string{keep})
	if !ud[keep].Played {
		t.Error("watched state didn't move to the kept title")
	}

	it, _ := st.GetItem(ctx, keep)
	ids, err := st.SplitItem(ctx, it, func(string) (string, int) { return "Final Cut", 1982 })
	if err != nil || len(ids) != 1 {
		t.Fatalf("split = %v, %v", ids, err)
	}
	if files, _ := st.ItemFiles(ctx, keep); len(files) != 1 {
		t.Errorf("after splitting the original has %d files", len(files))
	}
	if over, _ := st.FileItemOverride(ctx, cutFile); over != ids[0] {
		t.Errorf("split file's override = %q, want %q", over, ids[0])
	}
	if n, _ := st.GetItem(ctx, ids[0]); n == nil || n.Title != "Final Cut" || n.MetadataState != "pending" {
		t.Errorf("new title = %+v", n)
	}
}
