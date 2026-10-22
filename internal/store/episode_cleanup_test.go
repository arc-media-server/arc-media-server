package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func assertEpisodeReverseLookups(t *testing.T, st *Store) {
	t.Helper()
	for _, q := range []string{
		`SELECT 1 FROM ordering_entries WHERE episode_id = 'fixture'`,
		`SELECT 1 FROM user_item_data WHERE item_id = 'fixture'`,
	} {
		rows, err := st.db.Query("EXPLAIN QUERY PLAN " + q)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(detail, "SEARCH") {
				found = true
			}
			if strings.Contains(detail, "SCAN") {
				t.Fatalf("full scan: %s", detail)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !found {
			t.Fatal("lookup has no indexed search")
		}
	}
}

func TestEpisodeReverseLookupsUpgradeAndRestore(t *testing.T) {
	ctx := context.Background()
	old := oldBackupThrough(t, 45)
	backup := filepath.Join(t.TempDir(), "safety.db")
	st, err := Open(ctx, old, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(ctx, backup) }})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	assertEpisodeReverseLookups(t, st)
	if err := CheckBackup(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if err := st.RestoreFrom(ctx, backup); err != nil {
		t.Fatal(err)
	}
	assertEpisodeReverseLookups(t, st)
	it, err := st.GetItem(ctx, "movie")
	if err != nil || it == nil || it.Title != "My edited title" {
		t.Fatalf("restored edit: %+v %v", it, err)
	}
}

func TestEpisodeCleanupPreservesReferences(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Series", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	series, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Fixture"}, "series")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, key := range []string{"file", "order", "history", "orphan"} {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: series, Title: key}, key)
		if err != nil {
			t.Fatal(err)
		}
		ids[key] = id
	}
	fid, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/synthetic/episode.mkv", Size: 1, ModTime: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetFileItems(ctx, fid, []string{ids["file"]}, 0); err != nil {
		t.Fatal(err)
	}
	order, err := st.UpsertOrdering(ctx, series, "custom", "", "Keep", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceOrdering(ctx, order, nil, nil, []OrderingEntry{{EpisodeID: ids["order"], Season: 1, Episode: 1, Locked: true}}); err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, u.ID, ids["history"], 90000, 300000); err != nil {
		t.Fatal(err)
	}
	removed, err := st.DeleteOrphanEpisodes(ctx, series)
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
	for key, id := range ids {
		it, err := st.GetItem(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if (it == nil) != (key == "orphan") {
			t.Fatalf("%s retained=%v", key, it != nil)
		}
	}
	assertEpisodeReverseLookups(t, st)
}
