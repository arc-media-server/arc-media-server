package store

import (
	"context"
	"testing"
	"time"
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

	for _, u := range []struct {
		title, overview string
		runtime         int
		rating          float64
		addedDaysAgo    int
	}{
		{"Alien", "In space no one can hear you scream.", 117, 8.2, 400},
		{"Heat", "A Christmas-time heist in Los Angeles.", 170, 7.9, 3},
		{"Dune", "Paul Atreides travels to Arrakis.", 155, 7.8, 30},
	} {
		if _, err := st.db.ExecContext(ctx, `UPDATE items SET overview = ?, runtime_min = ?, rating = ?, added_at = ? WHERE title = ?`,
			u.overview, u.runtime, u.rating, time.Now().AddDate(0, 0, -u.addedDaysAgo).Unix(), u.title); err != nil {
			t.Fatal(err)
		}
	}
	check("added within", titles(ItemQuery{AddedWithinDays: 7}), "Heat")
	check("max runtime", titles(ItemQuery{MaxRuntime: 120}), "Alien")
	check("min runtime", titles(ItemQuery{MinRuntime: 150}), "Dune", "Heat")
	check("min rating", titles(ItemQuery{MinRating: 8}), "Alien")
	check("about", titles(ItemQuery{About: []string{"santa", "christmas"}}), "Heat")
	if got := titles(ItemQuery{Sort: "shuffle"}); len(got) != 3 {
		t.Errorf("shuffle: %v", got)
	}

	f, err := st.Facets(ctx, lib, "movie", Access{})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Genres) != 3 || len(f.Decades) != 3 || len(f.Resolutions) != 3 {
		t.Errorf("facets: %+v", f)
	}
}
