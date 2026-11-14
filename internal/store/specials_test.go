package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
)

func TestSpecialTraversalInheritanceAndIdentity(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tvdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	series, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, key := range []string{"a", "b", "c", "special1", "special2", "unknown", "missingTarget"} {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: series, Title: key}, key)
		if err != nil {
			t.Fatal(err)
		}
		ids[key] = id
	}
	order, err := st.UpsertOrdering(ctx, series, "tvdb_aired", "", "Aired", "")
	if err != nil {
		t.Fatal(err)
	}
	entries := []OrderingEntry{{EpisodeID: ids["special1"], Season: 0, Episode: 1}, {EpisodeID: ids["special2"], Season: 0, Episode: 2}, {EpisodeID: ids["unknown"], Season: 0, Episode: 3}, {EpisodeID: ids["missingTarget"], Season: 0, Episode: 4}, {EpisodeID: ids["a"], Season: 1, Episode: 1}, {EpisodeID: ids["b"], Season: 1, Episode: 2}, {EpisodeID: ids["c"], Season: 2, Episode: 1}}
	if err := st.ReplaceOrdering(ctx, order, map[int]string{0: "Specials", 1: "One", 2: "Two"}, nil, entries); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceSpecialPlacements(ctx, series, map[string]string{ids["special1"]: ids["b"], ids["special2"]: ids["b"], ids["missingTarget"]: "absent", ids["unknown"]: ids["unknown"]}); err != nil {
		t.Fatal(err)
	}
	check := func(label string, want []string) []OrderingEntry {
		t.Helper()
		got, err := st.DisplayEntries(ctx, order)
		if err != nil {
			t.Fatal(err)
		}
		actual := []string{}
		for _, e := range got {
			actual = append(actual, e.EpisodeID)
		}
		expected := []string{}
		for _, key := range want {
			expected = append(expected, ids[key])
		}
		if !slices.Equal(actual, expected) {
			t.Fatalf("%s traversal=%v want %v", label, actual, expected)
		}
		return got
	}
	off := []string{"a", "b", "c", "special1", "special2", "unknown", "missingTarget"}
	on := []string{"a", "special1", "special2", "b", "c", "unknown", "missingTarget"}
	check("default", off)
	yes, no := true, false
	if err := st.SetInlineSpecials(ctx, series, &yes); err != nil {
		t.Fatal(err)
	}
	got := check("on", on)
	if !got[1].InlineSpecial || got[1].Season != 0 || got[1].Episode != 1 || got[1].GroupSeason != 1 {
		t.Fatalf("changed source identity: %+v", got[1])
	}
	// A profile's old row (from when profiles could choose) is kept but no longer read.
	if _, err := st.db.ExecContext(ctx, `INSERT INTO series_specials(series_id,user_id,inline) VALUES(?, 'viewer', 0)`, series); err != nil {
		t.Fatal(err)
	}
	check("old profile row", on)
	if err := st.SetInlineSpecials(ctx, series, &no); err != nil {
		t.Fatal(err)
	}
	check("off", off)
	if err := st.SetInlineSpecials(ctx, series, nil); err != nil {
		t.Fatal(err)
	}
	check("cleared", off)
	if err := st.SetInlineSpecials(ctx, series, &yes); err != nil {
		t.Fatal(err)
	}
	check("on again", on)
	// An alternate order resolves the mapped target identity into its own season,
	// retaining repeated positions used by multi-part playback's last-position rule.
	alt, err := st.UpsertOrdering(ctx, series, "tvdb_dvd", "", "DVD", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceOrdering(ctx, alt, nil, nil, []OrderingEntry{
		{EpisodeID: ids["a"], Season: 3, Episode: 5},
		{EpisodeID: ids["b"], Season: 3, Episode: 6},
		{EpisodeID: ids["a"], Season: 3, Episode: 7},
		{EpisodeID: ids["special1"], Season: 0, Episode: 1},
	}); err != nil {
		t.Fatal(err)
	}
	altEntries, err := st.DisplayEntries(ctx, alt)
	if err != nil || len(altEntries) != 4 || altEntries[1].EpisodeID != ids["special1"] || altEntries[1].GroupSeason != 3 || altEntries[3].EpisodeID != ids["a"] {
		t.Fatalf("alternate traversal=%+v %v", altEntries, err)
	}
	if err := st.SetEntry(ctx, order, ids["special1"], 0, 1); err != nil {
		t.Fatal(err)
	}
	check("placed", []string{"a", "special2", "b", "c", "special1", "unknown", "missingTarget"})
	custom, err := st.CopyOrdering(ctx, &Ordering{ID: order, SeriesID: series}, "Custom")
	if err != nil {
		t.Fatal(err)
	}
	customEntries, err := st.DisplayEntries(ctx, custom)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range customEntries {
		if e.InlineSpecial {
			t.Fatal("changed custom placement")
		}
	}
	// Replacing hints removes stale positions and leaves stable IDs/watch data alone.
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.ReportProgress(ctx, u.ID, ids["special2"], 70000, 120000); err != nil {
		t.Fatal(err)
	}
	if err := st.ReplaceSpecialPlacements(ctx, series, nil); err != nil {
		t.Fatal(err)
	}
	check("placed", off)
	data, err := st.UserDataFor(ctx, u.ID, []string{ids["special2"]})
	if err != nil || data[ids["special2"]].PositionMS != 70000 {
		t.Fatalf("lost resume: %+v %v", data, err)
	}
}

func TestEpisodeDetailsUpgradePreservesLegacyMetadataAndDefaults(t *testing.T) {
	ctx := t.Context()
	old := oldBackupThrough(t, 51)
	db, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO people(id,tmdb_id,name,profile_path) VALUES(71,7,'Legacy actor',''); INSERT INTO item_people(item_id,person_id,role,character,ord) VALUES('movie',71,'actor','Visitor',0)`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "before-episode-upgrade.db")
	st, err := Open(ctx, old, Options{BeforeMigrate: func(s *Store) error { return s.BackupTo(ctx, backup) }})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cs, err := st.ItemCredits(ctx, "movie")
	if err != nil || len(cs) != 1 || cs[0].PersonID != 71 || cs[0].EpisodeCount != 0 {
		t.Fatalf("legacy credits=%+v %v", cs, err)
	}
	it, err := st.GetItem(ctx, "movie")
	if err != nil || it.Title != "My edited title" || !slices.Contains(it.LockedFields, "title") {
		t.Fatal("legacy metadata changed")
	}
	enabled, err := st.InlineSpecials(ctx, "movie")
	if err != nil || enabled {
		t.Fatalf("migration enabled specials: %v %v", enabled, err)
	}
	if err := st.RestoreFrom(ctx, backup); err != nil {
		t.Fatal(err)
	}
	cs, err = st.ItemCredits(ctx, "movie")
	if err != nil || len(cs) != 1 || cs[0].PersonID != 71 {
		t.Fatalf("legacy restore lost credits: %+v %v", cs, err)
	}
}

func TestCreditCountsReplaceWithoutChangingPeopleOrLocks(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", Title: "Pilot"}, "pilot")
	if err != nil {
		t.Fatal(err)
	}
	title := "Locked"
	if err := st.EditItem(ctx, id, ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{14, 0, 3} {
		if err := st.SetItemDetails(ctx, id, nil, []Credit{{TMDBID: 7, Name: "Actor", Role: "actor", EpisodeCount: count}, {TMDBID: 7, Name: "Actor", Role: "director"}, {TMDBID: 0, Name: "Invalid", Role: "actor"}}); err != nil {
			t.Fatal(err)
		}
		cs, err := st.ItemCredits(ctx, id)
		if err != nil || len(cs) != 2 || cs[0].EpisodeCount != count || cs[0].PersonID != cs[1].PersonID {
			t.Fatalf("credits=%+v %v", cs, err)
		}
	}
	it, err := st.GetItem(ctx, id)
	if err != nil || it.Title != title {
		t.Fatal("credits changed title lock")
	}
	if err := st.SetItemDetails(ctx, id, nil, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := st.ItemCredits(ctx, id)
	if err != nil || len(cs) != 0 {
		t.Fatalf("stale credits=%+v %v", cs, err)
	}
}

func TestDisplayEntriesWithoutPlacementsKeepsRepeatedIdentities(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	sid, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
	if err != nil {
		t.Fatal(err)
	}
	ep, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: sid, Title: "Episode"}, "episode")
	if err != nil {
		t.Fatal(err)
	}
	special, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: sid, Title: "Special"}, "special")
	if err != nil {
		t.Fatal(err)
	}
	oid, err := st.UpsertOrdering(ctx, sid, "tmdb_aired", "", "Aired", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.ReplaceOrdering(ctx, oid, nil, nil, []OrderingEntry{{EpisodeID: special, Season: 0, Episode: 1}, {EpisodeID: ep, Season: 1, Episode: 1}, {EpisodeID: ep, Season: 2, Episode: 1}}); err != nil {
		t.Fatal(err)
	}
	for _, inline := range []bool{false, true} {
		if err = st.SetInlineSpecials(ctx, sid, &inline); err != nil {
			t.Fatal(err)
		}
		got, err := st.DisplayEntries(ctx, oid)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 || got[0].EpisodeID != ep || got[1].EpisodeID != ep || got[2].EpisodeID != special || got[1].GroupSeason != 2 || got[2].InlineSpecial {
			t.Fatalf("inline%v: %+v", inline, got)
		}
	}
}
