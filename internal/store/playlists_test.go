package store

import (
	"context"
	"testing"
)

func TestPlaylists(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/m"})
	sam, _ := st.CreateUser(ctx, "Sam", false, "blue", "", "")
	add := func(title string, runtime int) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		st.db.ExecContext(ctx, `UPDATE items SET runtime_min = ? WHERE id = ?`, runtime, id)
		return id
	}
	a, b, c := add("A", 90), add("B", 100), add("C", 110)

	p, err := st.CreatePlaylist(ctx, sam.ID, "Weekend")
	if err != nil {
		t.Fatal(err)
	}
	st.AddToPlaylist(ctx, p.ID, []string{a, b})
	st.AddToPlaylist(ctx, p.ID, []string{c, a}) // a title can be in twice
	entries, items, err := st.PlaylistEntries(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 || items[0].Title != "A" || items[3].Title != "A" || entries[0].ID == entries[3].ID {
		t.Fatalf("entries: %+v", items)
	}
	lists, _ := st.Playlists(ctx, sam.ID, 0)
	if len(lists) != 1 || lists[0].Count != 4 || lists[0].DurationMS != (90+100+110+90)*60000 {
		t.Errorf("list: %+v", lists)
	}
	// Reverse it, then take the first entry out.
	st.OrderPlaylist(ctx, p.ID, []int64{entries[3].ID, entries[2].ID, entries[1].ID, entries[0].ID})
	st.RemovePlaylistEntry(ctx, p.ID, entries[3].ID)
	_, items, _ = st.PlaylistEntries(ctx, p.ID, 0)
	if len(items) != 3 || items[0].Title != "C" || items[2].Title != "A" {
		t.Errorf("after reorder and remove: %+v", items)
	}
	if other, _ := st.Playlists(ctx, "someone-else", 0); len(other) != 0 {
		t.Error("another profile sees the playlist")
	}
	st.DeletePlaylist(ctx, p.ID)
	if lists, _ = st.Playlists(ctx, sam.ID, 0); len(lists) != 0 {
		t.Error("deleted playlist still listed")
	}
}
