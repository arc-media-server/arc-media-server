package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestFirstFileArrivalReplacesMetadataDateAndDrivesSeriesRecency(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "TV", "shows", "tmdb_aired", []string{"/tv"})
	if err != nil {
		t.Fatal(err)
	}
	add := func(title, kind, parent string) string {
		t.Helper()
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Title: title, Kind: kind, ParentID: parent}, title)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	olderShow := add("Alpha", "series", "")
	newerShow := add("Zulu", "series", "")
	old := add("Already present", "episode", olderShow)
	late := add("Metadata known months ago", "episode", newerShow)
	if _, err := st.db.ExecContext(ctx, `UPDATE items SET added_at=100`); err != nil {
		t.Fatal(err)
	}
	attach := func(id, path string) int64 {
		t.Helper()
		f, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Size: 1, ModTime: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = st.SetFileItems(ctx, f, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		return f
	}
	attach(old, "/tv/old.mkv")
	// Represent a genuinely earlier arrival, not the later metadata discovery.
	if _, err = st.db.ExecContext(ctx, `UPDATE items SET added_at=200 WHERE id IN (?,?)`, old, olderShow); err != nil {
		t.Fatal(err)
	}
	since := time.Now().Unix()
	file := attach(late, "/tv/new.mkv")
	got, err := st.GetItem(ctx, late)
	if err != nil || got.AddedAt < since {
		t.Fatalf("new file retained metadata timestamp: %+v %v", got, err)
	}
	before := got.AddedAt
	for _, reverse := range []bool{false, true} {
		items, _, err := st.ListItems(ctx, ItemQuery{LibraryID: lib, Kind: "series", Sort: "added", Reverse: reverse})
		if err != nil {
			t.Fatal(err)
		}
		want := newerShow
		if reverse {
			want = olderShow
		}
		if len(items) != 2 || items[0].ID != want {
			t.Fatalf("series chronology reverse=%v: %+v", reverse, items)
		}
	}
	episodes, err := st.RecentEpisodes(ctx, lib, Access{}, 24)
	if err != nil || len(episodes) != 2 || episodes[0].ID != late {
		t.Fatalf("recent episodes: %+v %v", episodes, err)
	}
	// Metadata edits, repeated discovery and an offline/online cycle preserve arrival.
	got.Title = "Refreshed metadata"
	got.AddedAt = 1
	if err = st.UpdateMetadata(ctx, got); err != nil {
		t.Fatal(err)
	}
	if _, err = st.MarkMissing(ctx, lib, 100, 200); err != nil {
		t.Fatal(err)
	}
	if err = st.TouchFiles(ctx, []int64{file}, 300); err != nil {
		t.Fatal(err)
	}
	if err = st.SetFileItems(ctx, file, []string{late}, 0); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetItem(ctx, late)
	if got.AddedAt != before {
		t.Fatalf("refresh/offline return bumped arrival: %d -> %d", before, got.AddedAt)
	}
}

func TestAddedSortUsesArrivalSequenceAndRelinksPreserveIt(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	ids := []string{}
	var firstFile int64
	for _, name := range []string{"Alpha", "Zulu"} {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: name}, name)
		if err != nil {
			t.Fatal(err)
		}
		fid, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + name, Size: 1, ModTime: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = st.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		if firstFile == 0 {
			firstFile = fid
		}
		ids = append(ids, id)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE items SET added_at=100`); err != nil {
		t.Fatal(err)
	}
	for _, reverse := range []bool{false, true} {
		got, _, err := st.ListItems(ctx, ItemQuery{LibraryID: lib, Sort: "added", Reverse: reverse})
		if err != nil {
			t.Fatal(err)
		}
		want := ids[1]
		if reverse {
			want = ids[0]
		}
		if len(got) != 2 || got[0].ID != want {
			t.Fatalf("tied chronology reverse=%v: %+v", reverse, got)
		}
	}
	replacement, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Corrected match"}, "corrected")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.SetFileItems(ctx, firstFile, []string{replacement}, 0); err != nil {
		t.Fatal(err)
	}
	var before, after int64
	if err = st.db.QueryRowContext(ctx, `SELECT sequence FROM item_arrivals WHERE item_id=?`, ids[0]).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err = st.db.QueryRowContext(ctx, `SELECT sequence FROM item_arrivals WHERE item_id=?`, replacement).Scan(&after); err != nil || before != after {
		t.Fatalf("relink changed sequence: %d %d %v", before, after, err)
	}
}

func TestArrivalMigrationPreservesMissingLinkedHistoryButNotMetadataOnlyDates(t *testing.T) {
	ctx := context.Background()
	db := filepath.Join(t.TempDir(), "old.db")
	st, err := Open(ctx, db, Options{})
	if err != nil {
		t.Fatal(err)
	}
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	linked, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Linked"}, "linked")
	metadata, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Metadata only"}, "metadata")
	file, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/old", Size: 1, ModTime: 1}, 1, 1)
	if err = st.SetFileItems(ctx, file, []string{linked}, 0); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`UPDATE items SET added_at=100`, `UPDATE media_files SET missing_since=200`, `DROP TABLE item_arrivals`, `DROP TABLE arrival_clock`, `DROP INDEX items_parent_added`, `DELETE FROM schema_migrations WHERE version LIKE '0058%'`} {
		if _, err = st.db.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = Open(ctx, db, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var n int
	if err = st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_arrivals WHERE item_id=? AND added_at=100`, linked).Scan(&n); err != nil || n != 1 {
		t.Fatalf("lost historical missing-file arrival: %d %v", n, err)
	}
	if err = st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM item_arrivals WHERE item_id=?`, metadata).Scan(&n); err != nil || n != 0 {
		t.Fatalf("metadata-only became available: %d %v", n, err)
	}
}

func TestMergeAndSplitKeepEarlierArrival(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	ids := []string{}
	for _, name := range []string{"Earlier", "Later"} {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: name}, name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + name, Size: 1, ModTime: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = st.SetFileItems(ctx, file, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	var before int64
	if err := st.db.QueryRowContext(ctx, `SELECT sequence FROM item_arrivals WHERE item_id=?`, ids[0]).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if err := st.MergeItems(ctx, ids[1], []string{ids[0]}); err != nil {
		t.Fatal(err)
	}
	it, err := st.GetItem(ctx, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	split, err := st.SplitItem(ctx, it, func(string) (string, int) { return "Split", 0 })
	if err != nil {
		t.Fatal(err)
	}
	if len(split) != 1 {
		t.Fatalf("split: %v", split)
	}
	for _, id := range append(split, ids[1]) {
		var seq int64
		if err = st.db.QueryRowContext(ctx, `SELECT sequence FROM item_arrivals WHERE item_id=?`, id).Scan(&seq); err != nil || seq != before {
			t.Fatalf("merge/split bumped earlier arrival %s: %d/%d %v", id, seq, before, err)
		}
	}
}
