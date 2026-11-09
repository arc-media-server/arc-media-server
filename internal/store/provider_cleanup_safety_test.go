package store

import "testing"

func TestProviderCleanupKeepsEditedLabeledAndSpecialEpisodes(t *testing.T) {
	s := openTest(t)
	lib, err := s.SyncLibrary(t.Context(), "Shows", "shows", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	show, _, err := s.EnsureItem(t.Context(), &Item{LibraryID: lib, Kind: "series", Title: "Fixture"}, "series:fixture")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]string{}
	for _, name := range []string{"edited", "labelled", "special", "target", "untouched"} {
		id, _, err := s.EnsureItem(t.Context(), &Item{LibraryID: lib, Kind: "episode", ParentID: show, Title: name}, "ep:"+show+":"+name)
		if err != nil {
			t.Fatal(err)
		}
		ids[name] = id
	}
	title := "Owner edit"
	if err := s.EditItem(t.Context(), ids["edited"], ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(t.Context(), `INSERT INTO item_labels(item_id,label) VALUES (?,?)`, ids["labelled"], "keep"); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceSpecialPlacements(t.Context(), show, map[string]string{ids["special"]: ids["target"]}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteOrphanEpisodes(t.Context(), show); err != nil {
		t.Fatal(err)
	}
	for name, id := range ids {
		item, err := s.GetItem(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		if name == "untouched" {
			if item != nil {
				t.Error("unused orphan retained")
			}
		} else if item == nil {
			t.Errorf("user-owned %s episode deleted", name)
		}
	}
}
