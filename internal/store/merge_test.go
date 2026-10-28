package store

import (
	"context"
	"strconv"
	"testing"
)

func TestMergeAndSplit(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	user, _ := st.CreateUser(ctx, "Jo", true, "blue", "x", "")
	add := func(title, path string) (string, int64) {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, "title:"+title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		return id, fid
	}
	keep, _ := add("Blade Runner", "/m/Blade Runner (1982).mkv")
	cut, cutFile := add("Blade Runner Final Cut", "/m/Blade Runner Final Cut (1982).mkv")
	st.SetPlayed(ctx, user.ID, []string{cut}, true)

	if err := st.MergeItems(ctx, keep, []string{cut}); err != nil {
		t.Fatal(err)
	}
	if it, _ := st.GetItem(ctx, cut); it != nil {
		t.Error("the merged title is still there")
	}
	if files, _ := st.ItemFiles(ctx, keep); len(files) != 2 {
		t.Errorf("the kept title has %d files, want 2", len(files))
	}
	if over, _ := st.FileItemOverride(ctx, cutFile); over != keep {
		t.Errorf("override = %q, want the kept title", over)
	}
	ud, _ := st.UserDataFor(ctx, user.ID, []string{keep})
	if !ud[keep].Played {
		t.Error("watched state didn't move to the kept title")
	}

	it, _ := st.GetItem(ctx, keep)
	ids, err := st.SplitItem(ctx, it, func(string) (string, int) { return "Final Cut", 1982 })
	if err != nil || len(ids) != 1 {
		t.Fatalf("split = %v, %v", ids, err)
	}
	if files, _ := st.ItemFiles(ctx, keep); len(files) != 1 {
		t.Errorf("after splitting the original has %d files", len(files))
	}
	if over, _ := st.FileItemOverride(ctx, cutFile); over != ids[0] {
		t.Errorf("split file's override = %q, want %q", over, ids[0])
	}
	if n, _ := st.GetItem(ctx, ids[0]); n == nil || n.Title != "Final Cut" || n.MetadataState != "pending" {
		t.Errorf("new title = %+v", n)
	}
}

// Two folders of one show become one: the shared episode keeps the most progress and both files,
// an episode only the merged show had moves across with its id and history, show-level marks
// move, and the merged folder's key now finds the kept show.
func TestMergeSeries(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "TV", "shows", "tmdb_aired", []string{"/tv"})
	user, _ := st.CreateUser(ctx, "Jo", true, "blue", "x", "")
	show := func(key, title string) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: title}, key)
		return id
	}
	keep, other := show("series:title:doctor who:2005", "Doctor Who"), show("series:title:doctor who:0", "Doctor Who (copy)")
	episode := func(series string, n int64, path string) (string, int64) {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: series, Title: "Episode"},
			"ep:"+series+":tmdb:"+strconv.FormatInt(n, 10))
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		return id, fid
	}
	keepE1, _ := episode(keep, 1, "/tv/Doctor Who (2005)/S01E01.mkv")
	keepE2, _ := episode(keep, 2, "/tv/Doctor Who (2005)/S01E02.mkv")
	otherE2, otherE2File := episode(other, 2, "/tv/Doctor Who/S01E02.mkv")
	otherE3, _ := episode(other, 3, "/tv/Doctor Who/S01E03.mkv")
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := st.db.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	// Part-way through E2 in the kept show; watched it fully in the other folder.
	exec(`INSERT INTO user_item_data (user_id, item_id, position_ms, played, play_count, last_played_at) VALUES (?, ?, 600000, 0, 0, 100)`, user.ID, keepE2)
	exec(`INSERT INTO user_item_data (user_id, item_id, position_ms, played, play_count, last_played_at) VALUES (?, ?, 0, 1, 1, 50)`, user.ID, otherE2)
	exec(`INSERT INTO user_item_data (user_id, item_id, position_ms, played, play_count, last_played_at) VALUES (?, ?, 120000, 0, 0, 200)`, user.ID, otherE3)
	exec(`INSERT INTO user_item_data (user_id, item_id, watchlisted_at, user_rating) VALUES (?, ?, 300, 8)`, user.ID, other)
	for _, ep := range []string{otherE2, otherE3} {
		exec(`INSERT INTO play_history (user_id, item_id, kind, library_id, series_id, title, series_title, started_at, ended_at)
			VALUES (?, ?, 'episode', ?, ?, 'Episode', 'Doctor Who', 1, 2)`, user.ID, ep, lib, other)
	}

	if err := st.MergeSeries(ctx, keep, []string{other}); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{other, otherE2} {
		if it, _ := st.GetItem(ctx, gone); it != nil {
			t.Errorf("%s is still there", gone)
		}
	}
	moved, _ := st.GetItem(ctx, otherE3)
	if moved == nil || moved.ParentID != keep {
		t.Fatalf("the merged show's own episode = %+v, want it under the kept show", moved)
	}
	if e3, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: keep}, "ep:"+keep+":tmdb:3"); e3 != otherE3 {
		t.Errorf("the moved episode isn't found by the kept show's key (%s)", e3)
	}
	if files, _ := st.ItemFiles(ctx, keepE2); len(files) != 2 {
		t.Errorf("the shared episode has %d files, want both", len(files))
	}
	_ = otherE2File
	ud, _ := st.UserDataFor(ctx, user.ID, []string{keepE1, keepE2, otherE3, keep})
	if !ud[keepE2].Played || ud[keepE2].PositionMS != 0 {
		t.Errorf("shared episode = %+v, want watched (the most progress)", ud[keepE2])
	}
	if ud[otherE3].PositionMS != 120000 {
		t.Errorf("moved episode's resume point = %d", ud[otherE3].PositionMS)
	}
	if ud[keep].Rating != 8 || !ud[keep].Watchlisted {
		t.Errorf("show-level rating/watchlist didn't move: %+v", ud[keep])
	}
	var history int
	st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM play_history WHERE series_id = ? AND item_id IN (?, ?)`, keep, keepE2, otherE3).Scan(&history)
	if history != 2 {
		t.Errorf("%d history rows followed the episodes to the kept show, want 2", history)
	}
	// The merged folder's key now finds the kept show, so a rescan doesn't bring it back.
	if id, created, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Doctor Who"}, "series:title:doctor who:0"); id != keep || created {
		t.Errorf("merged folder's key -> %s (created %v), want the kept show", id, created)
	}
	if aliases, _ := st.SeriesAliases(ctx, lib); aliases["series:title:doctor who:0"] != "series:title:doctor who:2005" {
		t.Errorf("aliases = %v", aliases)
	}
	// Merging the kept show into a third show carries the alias along.
	third := show("series:title:the doctor:0", "The Doctor")
	if err := st.MergeSeries(ctx, third, []string{keep}); err != nil {
		t.Fatal(err)
	}
	if id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series"}, "series:title:doctor who:0"); id != third {
		t.Errorf("an older alias wasn't carried to the new kept show")
	}
}
