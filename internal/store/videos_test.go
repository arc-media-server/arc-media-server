package store

import (
	"context"
	"testing"
)

func TestFolderCollections(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Home Videos", "videos", "tmdb_aired", []string{"/v"})
	add := func(name string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: name}, "file:/v/"+name)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/v/" + name, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		if err := st.SetLocalVideo(ctx, id, name, "2020-05-06"); err != nil {
			t.Fatal(err)
		}
		return id
	}
	a, b := add("a.mp4"), add("b.mp4")
	if it, _ := st.GetItem(ctx, a); it.MetadataState != "locked" || it.Year != 2020 || it.ReleaseDate != "2020-05-06" {
		t.Errorf("local video = %+v", it)
	}
	if err := st.SyncFolderCollections(ctx, lib, map[string][]string{"Holidays": {b, a}}, true); err != nil {
		t.Fatal(err)
	}
	cs, _ := st.Collections(ctx, lib, Access{})
	if len(cs) != 1 || cs[0].Title != "Holidays" || cs[0].Kind != "folder" || cs[0].Count != 2 {
		t.Fatalf("collections = %+v", cs)
	}
	items, _ := st.CollectionItems(ctx, &cs[0], Access{})
	if len(items) != 2 || items[0].ID != b {
		t.Errorf("folder order = %v", items)
	}
	// The same folder keeps its collection; a folder that's gone is removed.
	st.SyncFolderCollections(ctx, lib, map[string][]string{"Holidays": {a}}, true)
	if again, _ := st.Collections(ctx, lib, Access{}); len(again) != 1 || again[0].ID != cs[0].ID || again[0].Count != 1 {
		t.Errorf("after rescan = %+v", again)
	}
	st.SyncFolderCollections(ctx, lib, map[string][]string{}, true)
	if gone, _ := st.Collections(ctx, lib, Access{}); len(gone) != 0 {
		t.Errorf("removed folder still has a collection: %+v", gone)
	}
}
