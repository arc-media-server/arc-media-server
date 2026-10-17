package store

import (
	"context"
	"testing"
)

func TestWatchState(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/m"})
	movie, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Alien"}, "m1")
	u, err := st.CreateUser(ctx, "Sam", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	const hour = int64(3_600_000)
	report := func(pos int64) UserData {
		t.Helper()
		d, err := st.ReportProgress(ctx, u.ID, movie, pos, 2*hour)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}

	if d := report(30_000); d.PositionMS != 0 || d.Played {
		t.Errorf("first 30s shouldn't leave a resume point: %+v", d)
	}
	if d := report(hour); d.PositionMS != hour || d.Played {
		t.Errorf("half way: %+v", d)
	}
	if d := report(2*hour - 60_000); !d.Played || d.PlayCount != 1 || d.PositionMS != 0 {
		t.Errorf("near the end should count as watched once: %+v", d)
	}
	if d := report(2 * hour); d.PlayCount != 1 {
		t.Errorf("more reports from the same viewing must not count again: %+v", d)
	}
	// Watching again from the start counts a second play when it's finished.
	report(10 * 60_000)
	if d := report(2 * hour); d.PlayCount != 2 {
		t.Errorf("rewatch: %+v", d)
	}

	if err := st.SetPlayed(ctx, u.ID, []string{movie}, false); err != nil {
		t.Fatal(err)
	}
	data, _ := st.UserDataFor(ctx, u.ID, []string{movie})
	if data[movie].Played || data[movie].PositionMS != 0 {
		t.Errorf("after marking unwatched: %+v", data[movie])
	}
	if err := st.SetPlayed(ctx, u.ID, []string{movie}, true); err != nil {
		t.Fatal(err)
	}
	data, _ = st.UserDataFor(ctx, u.ID, []string{movie})
	if !data[movie].Played || data[movie].PlayCount != 3 {
		t.Errorf("after marking watched: %+v", data[movie])
	}

	// Part-way through: listed in Continue Watching (once it has a file on disk).
	report(hour)
	if _, err := st.InProgress(ctx, u.ID, 10); err != nil {
		t.Fatalf("InProgress: %v", err)
	}

	// Other profiles have their own state.
	other, _ := st.CreateUser(ctx, "Alex", false, "teal", "", "")
	data, _ = st.UserDataFor(ctx, other.ID, []string{movie})
	if _, ok := data[movie]; ok {
		t.Error("another profile shouldn't see Sam's watch state")
	}
}

func TestRatingsAndWatchlist(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	sam, _ := st.CreateUser(ctx, "Sam", false, "blue", "", "")
	add := func(title string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		return id
	}
	heat, alien := add("Heat"), add("Alien")

	st.SetUserRating(ctx, sam.ID, heat, 9)
	st.SetWatchlist(ctx, sam.ID, alien, true)
	st.SetWatchlist(ctx, sam.ID, heat, true)
	d, _ := st.UserDataFor(ctx, sam.ID, []string{heat, alien})
	if d[heat].Rating != 9 || !d[heat].Watchlisted || d[alien].Rating != 0 || !d[alien].Watchlisted {
		t.Fatalf("user data: %+v", d)
	}
	list, err := st.Watchlist(ctx, sam.ID, Access{})
	if err != nil || len(list) != 2 {
		t.Fatalf("watchlist: %+v %v", list, err)
	}
	// Taking one off leaves the other; its rating stays.
	st.SetWatchlist(ctx, sam.ID, heat, false)
	if list, _ = st.Watchlist(ctx, sam.ID, Access{}); len(list) != 1 || list[0].ID != alien {
		t.Errorf("after removing Heat: %+v", list)
	}
	if d, _ = st.UserDataFor(ctx, sam.ID, []string{heat}); d[heat].Rating != 9 || d[heat].Watchlisted {
		t.Errorf("Heat after removing it: %+v", d[heat])
	}
	if other, _ := st.Watchlist(ctx, "someone-else", Access{}); len(other) != 0 {
		t.Error("another profile sees the watchlist")
	}
}
