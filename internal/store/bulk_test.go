package store

import (
	"context"
	"slices"
	"testing"
)

func TestEditItems(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	add := func(title string, genres ...string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		st.SetItemDetails(ctx, id, genres, nil)
		return id
	}
	a := add("Alien", "Horror", "Science Fiction")
	b := add("Heat", "Crime")
	rating := "18"
	if err := st.EditItems(ctx, []string{a, b}, BulkEdit{
		AddGenres: []string{"Thriller", "horror"}, RemoveGenres: []string{"science fiction"},
		AddLabels: []string{"Favourites"}, ContentRating: &rating,
	}); err != nil {
		t.Fatal(err)
	}
	ga, _ := st.ItemGenres(ctx, a)
	gb, _ := st.ItemGenres(ctx, b)
	if !slices.Equal(ga, []string{"Horror", "Thriller"}) || !slices.Equal(gb, []string{"Crime", "Thriller", "horror"}) {
		t.Errorf("genres = %v, %v", ga, gb)
	}
	it, _ := st.GetItem(ctx, a)
	if it.ContentRating != "18" || !slices.Equal(it.LockedFields, []string{"content_rating", "genres"}) {
		t.Errorf("rating %q, locked %v", it.ContentRating, it.LockedFields)
	}
	if l, _ := st.ItemLabels(ctx, b); !slices.Equal(l, []string{"Favourites"}) {
		t.Errorf("labels = %v", l)
	}

	st.EditItems(ctx, []string{a}, BulkEdit{Locks: "all"})
	if it, _ = st.GetItem(ctx, a); len(it.LockedFields) != len(LockableFields) {
		t.Errorf("lock all: %v", it.LockedFields)
	}
	st.EditItems(ctx, []string{a}, BulkEdit{Locks: "none", RemoveLabels: []string{"favourites"}})
	if it, _ = st.GetItem(ctx, a); len(it.LockedFields) != 0 {
		t.Errorf("unlock all: %v", it.LockedFields)
	}
	if l, _ := st.ItemLabels(ctx, a); len(l) != 0 {
		t.Errorf("labels after removing = %v", l)
	}
}
