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
