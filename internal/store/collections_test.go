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
	list, err := st.Collections(ctx, lib, Access{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "The Matrix Collection" || list[0].Count != 2 || list[0].Kind != "auto" {
		t.Fatalf("collections: %+v", list)
	}
	items, _ := st.CollectionItems(ctx, &list[0], Access{})
	if len(items) != 2 || items[0].Title != "The Matrix" {
		t.Errorf("by release date: %+v", items)
	}
	st.SetLibraryCollectionMin(ctx, lib, 1)
	if list, _ = st.Collections(ctx, lib, Access{}); len(list) != 2 {
		t.Errorf("minimum of 1: %d collections", len(list))
	}
	st.SetLibraryCollectionMin(ctx, lib, 0)
	if list, _ = st.Collections(ctx, lib, Access{}); len(list) != 0 {
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
	items, _ = st.CollectionItems(ctx, fav, Access{})
	if len(items) != 2 || items[0].Title != "Alien" {
		t.Errorf("manual order: %+v", items)
	}
	st.OrderCollection(ctx, fav.ID, []string{matrix, alien})
	if items, _ = st.CollectionItems(ctx, fav, Access{}); items[0].Title != "The Matrix" {
		t.Errorf("reordered: %+v", items)
	}
	it, _ := st.GetItem(ctx, matrix)
	in, _ := st.ItemCollections(ctx, it, Access{})
	if len(in) != 2 {
		t.Errorf("The Matrix is in %d collections, want 2", len(in))
	}

	// Removing an automatic collection hides it; it doesn't come back with a refresh.
	auto, _ := st.Collections(ctx, lib, Access{})
	for _, c := range auto {
		if c.Kind == "auto" {
			st.DeleteCollection(ctx, &c)
		}
	}
	st.SetMovieCollection(ctx, matrix, lib, ref)
	if list, _ = st.Collections(ctx, lib, Access{}); len(list) != 1 || list[0].Title != "Favourites" {
		t.Errorf("after hiding: %+v", list)
	}
}

func TestEditedCollectionSurvivesRefresh(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Blade Runner"}, "br")
	ref := &CollectionRef{TMDBID: 422837, Name: "Blade Runner Collection", PosterPath: "/tmdb.jpg"}
	if err := st.SetMovieCollection(ctx, id, lib, ref); err != nil {
		t.Fatal(err)
	}
	var cid string
	if err := st.db.QueryRowContext(ctx, `SELECT id FROM collections WHERE tmdb_id = 422837`).Scan(&cid); err != nil {
		t.Fatal("no automatic collection:", err)
	}
	c, _ := st.GetCollection(ctx, cid)
	c.Title, c.LockedFields = "The Blade Runners", []string{"title"}
	if err := st.UpdateCollection(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := st.SetCollectionArt(ctx, c.ID, "poster", "upload:mine.jpg"); err != nil {
		t.Fatal(err)
	}
	// A rescan and a TMDB refresh bring the provider's name and artwork again.
	st.SetMovieCollection(ctx, id, lib, ref)
	st.SetCollectionInfo(ctx, c.ID, "Replicants.", "/tmdb2.jpg", "/back.jpg")
	got, _ := st.GetCollection(ctx, c.ID)
	if got.Title != "The Blade Runners" || got.PosterPath != "upload:mine.jpg" || got.BackdropPath != "/back.jpg" || got.Overview != "Replicants." {
		t.Errorf("after refresh: %+v", got)
	}
	// Automatic again: the next refresh sets it.
	st.SetCollectionArt(ctx, c.ID, "poster", "")
	st.SetCollectionInfo(ctx, c.ID, "Replicants.", "/tmdb2.jpg", "")
	if got, _ = st.GetCollection(ctx, c.ID); got.PosterPath != "/tmdb2.jpg" {
		t.Errorf("after going back to automatic: %q", got.PosterPath)
	}
}

// A Plex import brings over Plex's copy of a TMDB collection Arc already makes by itself. The two
// show as one (the imported one) while it holds all the automatic one's titles; once the
// automatic one has a title the copy lacks, both show in the library so that title isn't lost,
// but a title in both still gets one row on its page.
func TestImportedCopyOfAutomaticCollectionShowsOnce(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/m"})
	add := func(title string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		return id
	}
	one, two := add("Toy Story"), add("Toy Story 2")
	ref := &CollectionRef{TMDBID: 10194, Name: "Toy Story Collection"}
	for _, id := range []string{one, two} {
		if err := st.SetMovieCollection(ctx, id, lib, ref); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.ImportCollection(ctx, lib, "toy story collection ", []string{two, one}); err != nil {
		t.Fatal(err)
	}
	list, err := st.Collections(ctx, lib, Access{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Kind != "manual" || list[0].Count != 2 {
		t.Fatalf("library collections: %+v", list)
	}
	it, _ := st.GetItem(ctx, one)
	if in, _ := st.ItemCollections(ctx, it, Access{}); len(in) != 1 || in[0].Kind != "manual" {
		t.Fatalf("title's collections: %+v", in)
	}

	// A third film arrives that only TMDB's collection has: both show again.
	three := add("Toy Story 3")
	if err := st.SetMovieCollection(ctx, three, lib, ref); err != nil {
		t.Fatal(err)
	}
	if list, _ = st.Collections(ctx, lib, Access{}); len(list) != 2 {
		t.Fatalf("with a title only TMDB's has: %+v", list)
	}
	it3, _ := st.GetItem(ctx, three)
	if in, _ := st.ItemCollections(ctx, it3, Access{}); len(in) != 1 || in[0].Kind != "auto" {
		t.Fatalf("the new film's collections: %+v", in)
	}
	// The first film is in both, and still gets one row on its page.
	if in, _ := st.ItemCollections(ctx, it, Access{}); len(in) != 1 || in[0].Kind != "manual" {
		t.Fatalf("title in both: %+v", in)
	}
}
