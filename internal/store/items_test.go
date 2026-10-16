package store

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

func openTest(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "ams.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestLockedFieldsSurviveRefresh(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/m"})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "alien"}, "title:alien:1979")
	if err != nil {
		t.Fatal(err)
	}
	refresh := func(title, overview, poster string) *Item {
		t.Helper()
		if err := st.UpdateMetadata(ctx, &Item{ID: id, Title: title, Overview: overview, Year: 1979,
			PosterPath: poster, MetadataState: "matched"}); err != nil {
			t.Fatal(err)
		}
		it, err := st.GetItem(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return it
	}

	refresh("Alien", "In space no one can hear you scream.", "/a.jpg")

	// Edit the title and lock the poster.
	title := "Alien (Director's Cut)"
	if err := st.EditItem(ctx, id, ItemEdit{Title: &title, Locked: []string{"poster"}}); err != nil {
		t.Fatal(err)
	}
	it := refresh("Alien", "New overview.", "/b.jpg")
	if it.Title != title || it.SortTitle != "alien (director's cut)" {
		t.Errorf("edited title not kept: %q / %q", it.Title, it.SortTitle)
	}
	if it.PosterPath != "/a.jpg" {
		t.Errorf("locked poster replaced with %q", it.PosterPath)
	}
	if it.Overview != "New overview." {
		t.Errorf("unlocked overview not refreshed: %q", it.Overview)
	}
	if !slices.Equal(it.LockedFields, []string{"poster", "title"}) {
		t.Errorf("locked fields = %v", it.LockedFields)
	}

	// Unlocking lets the next refresh replace the values again.
	if err := st.EditItem(ctx, id, ItemEdit{Locked: []string{}}); err != nil {
		t.Fatal(err)
	}
	it = refresh("Alien", "New overview.", "/b.jpg")
	if it.Title != "Alien" || it.SortTitle != "alien" || it.PosterPath != "/b.jpg" || len(it.LockedFields) != 0 {
		t.Errorf("after unlocking: %+v", it)
	}

	// A custom sort title is kept; clearing it makes it follow the title again.
	sortAs := "Alien 1"
	if err := st.EditItem(ctx, id, ItemEdit{SortTitle: &sortAs}); err != nil {
		t.Fatal(err)
	}
	if it = refresh("Alien", "", ""); it.SortTitle != "alien 1" {
		t.Errorf("sort title = %q, want the custom one", it.SortTitle)
	}
	empty := ""
	if err := st.EditItem(ctx, id, ItemEdit{SortTitle: &empty}); err != nil {
		t.Fatal(err)
	}
	if it, _ = st.GetItem(ctx, id); it.SortTitle != "alien" || slices.Contains(it.LockedFields, "sort_title") {
		t.Errorf("cleared sort title = %q (locked %v)", it.SortTitle, it.LockedFields)
	}

	if err := st.EditItem(ctx, id, ItemEdit{Locked: []string{"nope"}}); err == nil {
		t.Error("unknown locked field accepted")
	}
}
