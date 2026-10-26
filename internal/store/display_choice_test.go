package store

import "testing"

func TestDisplayChoiceKeepsInheritanceSeparateFromEffectiveOrder(t *testing.T) {
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
	check := func(user string, admin bool, choice, effective int64) {
		t.Helper()
		got, err := st.DisplayChoice(ctx, id, user, admin)
		if err != nil || got != choice {
			t.Fatalf("saved choice=%d want %d (%v)", got, choice, err)
		}
		got, err = st.DisplayOrdering(ctx, id, user, "tmdb_aired")
		if err != nil || got != effective {
			t.Fatalf("effective=%d want %d (%v)", got, effective, err)
		}
	}
	check("owner", true, 0, aired)
	if err := st.SetDisplayOrdering(ctx, id, "", story); err != nil {
		t.Fatal(err)
	}
	check("viewer", false, 0, story)
	check("owner", true, story, story)
	if err := st.SetDisplayOrdering(ctx, id, "viewer", aired); err != nil {
		t.Fatal(err)
	}
	check("viewer", false, aired, aired)
	if err := st.ClearDisplayOrdering(ctx, id, "viewer"); err != nil {
		t.Fatal(err)
	}
	check("viewer", false, 0, story)
}
