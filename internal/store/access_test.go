package store

import (
	"context"
	"testing"
)

func TestLibraryAccess(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	movies, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	kids, _ := st.SyncLibrary(ctx, "Kids", "movies", "tmdb_aired", []string{"/k"})
	add := func(lib int64, title string) *Item {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		it, _ := st.GetItem(ctx, id)
		return it
	}
	heat, frozen := add(movies, "Heat"), add(kids, "Frozen")

	kidsOnly := Access{Libraries: []int64{kids}}
	titles, err := st.SearchTitles(ctx, kidsOnly)
	if err != nil || len(titles) != 1 || titles[0].ID != frozen.ID {
		t.Fatalf("search for a kids-only profile: %+v %v", titles, err)
	}
	if all, _ := st.SearchTitles(ctx, Access{}); len(all) != 2 {
		t.Errorf("no restriction: %d titles", len(all))
	}
	if none, _ := st.SearchTitles(ctx, Access{Libraries: []int64{}}); len(none) != 0 {
		t.Errorf("no libraries: %d titles", len(none))
	}
	if ok, _ := st.CanSee(ctx, heat, kidsOnly); ok {
		t.Error("a kids-only profile can see a film in Movies")
	}
	if ok, _ := st.CanSee(ctx, frozen, kidsOnly); !ok {
		t.Error("a kids-only profile can't see a film in Kids")
	}

	// The profile keeps its list; an empty list means none, and clearing it means all again.
	u, _ := st.CreateUser(ctx, "Kid", false, "teal", "", "")
	ids := []int64{kids}
	st.UpdateUser(ctx, u.ID, UserUpdate{LibraryIDs: &ids})
	if got, _ := st.GetUser(ctx, u.ID); len(got.LibraryIDs) != 1 || got.LibraryIDs[0] != kids {
		t.Errorf("saved libraries: %v", got.LibraryIDs)
	}
	empty := []int64{}
	st.UpdateUser(ctx, u.ID, UserUpdate{LibraryIDs: &empty})
	if got, _ := st.GetUser(ctx, u.ID); got.LibraryIDs == nil || len(got.LibraryIDs) != 0 {
		t.Errorf("no libraries: %v", got.LibraryIDs)
	}
	st.UpdateUser(ctx, u.ID, UserUpdate{AllLibraries: true})
	if got, _ := st.GetUser(ctx, u.ID); got.LibraryIDs != nil {
		t.Errorf("all libraries: %v", got.LibraryIDs)
	}
}
