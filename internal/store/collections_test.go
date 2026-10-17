package store

import (
	"context"
	"testing"
)

func TestCollections(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/m"})
	add := func(title string, year int, release string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title, Year: year}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		st.db.ExecContext(ctx, `UPDATE items SET release_date = ? WHERE id = ?`, release, id)
		return id
	}
	reloaded := add("The Matrix Reloaded", 2003, "2003-05-15")
	matrix := add("The Matrix", 1999, "1999-03-31")
	alien := add("Alien", 1979, "1979-05-25")
	ref := &CollectionRef{TMDBID: 2344, Name: "The Matrix Collection", PosterPath: "/m.jpg"}
	for _, id := range []string{matrix, reloaded} {
		if err := st.SetMovieCollection(ctx, id, lib, ref); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetMovieCollection(ctx, alien, lib, &CollectionRef{TMDBID: 8091, Name: "Alien Collection"}); err != nil {
		t.Fatal(err)
	}

	// The Alien collection has only one film here, below the default minimum of 2.
	list, err := st.Collections(ctx, lib, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "The Matrix Collection" || list[0].Count != 2 || list[0].Kind != "auto" {
		t.Fatalf("collections: %+v", list)
	}
	items, _ := st.CollectionItems(ctx, &list[0], 0)
	if len(items) != 2 || items[0].Title != "The Matrix" {
		t.Errorf("by release date: %+v", items)
	}
	st.SetLibraryCollectionMin(ctx, lib, 1)
	if list, _ = st.Collections(ctx, lib, 0); len(list) != 2 {
		t.Errorf("minimum of 1: %d collections", len(list))
	}
	st.SetLibraryCollectionMin(ctx, lib, 0)
	if list, _ = st.Collections(ctx, lib, 0); len(list) != 0 {
		t.Errorf("automatic off: %d collections", len(list))
	}
	st.SetLibraryCollectionMin(ctx, lib, 2)

	// A manual collection keeps its order.
	fav := &Collection{LibraryID: lib, Kind: "manual", Title: "Favourites"}
	if err := st.CreateCollection(ctx, fav); err != nil {
		t.Fatal(err)
	}
	st.AddToCollection(ctx, fav.ID, []string{alien, matrix})
	st.AddToCollection(ctx, fav.ID, []string{matrix}) // already in it
	items, _ = st.CollectionItems(ctx, fav, 0)
	if len(items) != 2 || items[0].Title != "Alien" {
		t.Errorf("manual order: %+v", items)
	}
	st.OrderCollection(ctx, fav.ID, []string{matrix, alien})
	if items, _ = st.CollectionItems(ctx, fav, 0); items[0].Title != "The Matrix" {
		t.Errorf("reordered: %+v", items)
	}
	it, _ := st.GetItem(ctx, matrix)
	in, _ := st.ItemCollections(ctx, it, 0)
	if len(in) != 2 {
		t.Errorf("The Matrix is in %d collections, want 2", len(in))
	}

	// Removing an automatic collection hides it; it doesn't come back with a refresh.
	auto, _ := st.Collections(ctx, lib, 0)
	for _, c := range auto {
		if c.Kind == "auto" {
			st.DeleteCollection(ctx, &c)
		}
	}
	st.SetMovieCollection(ctx, matrix, lib, ref)
	if list, _ = st.Collections(ctx, lib, 0); len(list) != 1 || list[0].Title != "Favourites" {
		t.Errorf("after hiding: %+v", list)
	}
}
