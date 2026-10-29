package store

import "testing"

func TestDisplayChoiceIsTheServerDefault(t *testing.T) {
	ctx := t.Context()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
	if err != nil {
		t.Fatal(err)
	}
	aired, err := st.UpsertOrdering(ctx, id, "tmdb_aired", "", "Aired", "")
	if err != nil {
		t.Fatal(err)
	}
	story, err := st.UpsertOrdering(ctx, id, "custom", "", "Story", "")
	if err != nil {
		t.Fatal(err)
	}
	check := func(choice, effective int64) {
		t.Helper()
		got, err := st.DisplayChoice(ctx, id)
		if err != nil || got != choice {
			t.Fatalf("saved choice=%d want %d (%v)", got, choice, err)
		}
		got, err = st.DisplayOrdering(ctx, id, "tmdb_aired")
		if err != nil || got != effective {
			t.Fatalf("effective=%d want %d (%v)", got, effective, err)
		}
	}
	check(0, aired)
	if err := st.SetDisplayOrdering(ctx, id, "", story); err != nil {
		t.Fatal(err)
	}
	check(story, story)
	// A profile's old row (from when profiles could choose) is kept but no longer read.
	if err := st.SetDisplayOrdering(ctx, id, "viewer", aired); err != nil {
		t.Fatal(err)
	}
	check(story, story)
	if err := st.ClearDisplayOrdering(ctx, id, ""); err != nil {
		t.Fatal(err)
	}
	check(0, aired)
}
