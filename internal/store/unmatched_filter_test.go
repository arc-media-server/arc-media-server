package store

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
)

// The Unmatched filter finds titles Arc couldn't identify, and matched shows with an episode on
// disk it couldn't place; not an unplaced episode without a file, and not identified titles.
func TestUnmatchedFilter(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Unmatched fixture", "shows", "tvdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	add := func(kind, title, parent, state string, file bool) string {
		t.Helper()
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: kind, Title: title, ParentID: parent}, title+parent)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.db.ExecContext(ctx, `UPDATE items SET metadata_state = ? WHERE id = ?`, state, id); err != nil {
			t.Fatal(err)
		}
		if file {
			fid, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), title+".mkv")}, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	known := add("series", "Known Show", "", "matched", false)
	add("episode", "Pilot", known, "matched", true)
	stray := add("series", "Show With A Stray", "", "matched", false)
	add("episode", "Pilot", stray, "matched", true)
	add("episode", "Episode 20", stray, "unmatched", true)
	gone := add("series", "Show With A Gone Stray", "", "matched", false)
	add("episode", "Pilot", gone, "matched", true)
	add("episode", "Episode 30", gone, "unmatched", false)
	mystery := add("series", "Mystery Show", "", "unmatched", false)
	add("episode", "Episode 1", mystery, "unmatched", true)

	items, total, err := st.ListItems(ctx, ItemQuery{LibraryID: lib, Kind: "series", Unmatched: true, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, it := range items {
		titles = append(titles, it.Title)
	}
	slices.Sort(titles)
	if total != 2 || !slices.Equal(titles, []string{"Mystery Show", "Show With A Stray"}) {
		t.Fatalf("unmatched filter = %v (total %d), want Mystery Show and Show With A Stray", titles, total)
	}
	f, err := st.Facets(ctx, lib, "series", Access{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Unmatched != 2 {
		t.Fatalf("facets unmatched = %d, want 2", f.Unmatched)
	}
}
