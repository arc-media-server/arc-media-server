package store

import (
	"context"
	"testing"
)

func TestListItemFilters(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/m"})
	user, _ := st.CreateUser(ctx, "Sam", false, "blue", "", "")
	add := func(title string, year, height int, genres ...string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title, Year: year}, title)
		fid, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: 1, ModTime: 1,
			Streams: []Stream{{Index: 0, Kind: "video", Width: height * 16 / 9, Height: height}}}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		st.SetFileItems(ctx, fid, []string{id}, 0)
		st.SetItemDetails(ctx, id, genres, nil)
		return id
	}
	alien := add("Alien", 1979, 1080, "Horror", "Science Fiction")
	add("Heat", 1995, 720, "Crime")
	add("Dune", 2021, 2160, "Science Fiction")
	st.SetPlayed(ctx, user.ID, []string{alien}, true)

	titles := func(q ItemQuery) []string {
		t.Helper()
		q.LibraryID, q.Kind = lib, "movie"
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
	check := func(name string, got []string, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: got %v, want %v", name, got, want)
				return
			}
		}
	}
	check("genre", titles(ItemQuery{Genre: "Science Fiction"}), "Alien", "Dune")
	check("decade", titles(ItemQuery{Decade: 1990}), "Heat")
	check("4k", titles(ItemQuery{Resolution: "4k"}), "Dune")
	check("1080", titles(ItemQuery{Resolution: "1080"}), "Alien")
	check("unwatched", titles(ItemQuery{UserID: user.ID, Unwatched: true}), "Dune", "Heat")
	check("watched sort", titles(ItemQuery{UserID: user.ID, Sort: "watched"}), "Alien", "Dune", "Heat")

	f, err := st.Facets(ctx, lib, "movie", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Genres) != 3 || len(f.Decades) != 3 || len(f.Resolutions) != 3 {
		t.Errorf("facets: %+v", f)
	}
}
