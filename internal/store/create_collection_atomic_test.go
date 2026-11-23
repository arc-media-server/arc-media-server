package store

import (
	"errors"
	"testing"
)

func TestCollectionCreateRollsBackWhenInitialMemberWriteFails(t *testing.T) {
	s := openTest(t)
	lib, err := s.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.EnsureItem(t.Context(), &Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	rejectMember := `CREATE TRIGGER fail_fixture_member BEFORE INSERT ON collection_items BEGIN SELECT RAISE(ABORT,'fixture membership write failure'); END`
	if s.IsPostgres() {
		rejectMember = `ALTER TABLE collection_items ADD CONSTRAINT reject_fixture_member CHECK (position < 0)`
	}
	if _, err = s.db.ExecContext(t.Context(), rejectMember); err != nil {
		t.Fatal(err)
	}
	c := &Collection{LibraryID: lib, Kind: "manual", Title: "Must roll back"}
	if err = s.CreateCollection(t.Context(), c, item); err == nil {
		t.Fatal("injected write failure ignored")
	}
	if c.ID != "" {
		t.Fatal("failed create published an identity", c.ID)
	}
	var count int
	if err = s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM collections`).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial collection persisted", count, err)
	}
}

func TestCollectionCreateRejectsWrongLibraryAndEpisodeBeforeWrite(t *testing.T) {
	s := openTest(t)
	lib, err := s.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.SyncLibrary(t.Context(), "Other", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	wrongLibrary, _, err := s.EnsureItem(t.Context(), &Item{LibraryID: other, Kind: "movie", Title: "Wrong library"}, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	episode, _, err := s.EnsureItem(t.Context(), &Item{LibraryID: lib, Kind: "episode", Title: "Episode"}, "episode")
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{wrongLibrary, episode, "missing"} {
		c := &Collection{LibraryID: lib, Kind: "manual", Title: "Invalid"}
		if err = s.CreateCollection(t.Context(), c, member); !errors.Is(err, ErrCollectionItems) {
			t.Fatal("invalid initial member accepted", member, err)
		}
	}
	var count int
	if err = s.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM collections`).Scan(&count); err != nil || count != 0 {
		t.Fatal("invalid create left rows", count, err)
	}
}
