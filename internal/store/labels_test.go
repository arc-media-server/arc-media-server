package store

import (
	"context"
	"testing"
)

func TestLabels(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	add := func(title string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		return id
	}
	saw := add("Saw")
	add("Paddington")
	labels := []string{"Not for kids", "Halloween"}
	if err := st.EditItem(ctx, saw, ItemEdit{Labels: &labels}); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.ItemLabels(ctx, saw); len(got) != 2 || got[0] != "Not for kids" {
		t.Errorf("labels = %v", got)
	}

	titles := func(q ItemQuery) []string {
		q.LibraryID = lib
		items, _, err := st.ListItems(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, it := range items {
			out = append(out, it.Title)
		}
		return out
	}
	if got := titles(ItemQuery{Label: "halloween"}); len(got) != 1 || got[0] != "Saw" {
		t.Errorf("label filter (any case) = %v", got)
	}
	kids := Access{ExcludeLabels: []string{"not for KIDS"}}
	if got := titles(ItemQuery{Access: kids}); len(got) != 1 || got[0] != "Paddington" {
		t.Errorf("hidden label = %v", got)
	}
	it, _ := st.GetItem(ctx, saw)
	if ok, err := st.CanSee(ctx, it, kids); err != nil || ok {
		t.Errorf("CanSee a hidden title = %v, %v", ok, err)
	}
	if ok, _ := st.CanSee(ctx, it, Access{}); !ok {
		t.Error("CanSee without restrictions = false")
	}
}
