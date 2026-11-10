package store

import (
	"errors"
	"testing"
)

func TestImportProfileCreationRollsBackOnLaterCollision(t *testing.T) {
	s := openTest(t)
	for _, name := range []string{"Taken", "Taken (Plex)"} {
		if _, err := s.CreateUser(t.Context(), name, false, "slate", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	before, err := s.CountUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = s.PrepareImportProfiles(t.Context(), []ImportProfileTarget{{AccountID: "one", Name: "Would be orphaned", Target: "new"}, {AccountID: "two", Name: "Taken", Target: "new"}}, "Plex")
	if !errors.Is(err, ErrNameTaken) {
		t.Fatal(err)
	}
	after, err := s.CountUsers(t.Context())
	if err != nil || after != before {
		t.Fatal("partial profiles persisted", before, after, err)
	}
}

func TestImportProfilesValidateAllAndKeepExistingFallback(t *testing.T) {
	s := openTest(t)
	existing, err := s.CreateUser(t.Context(), "Existing", false, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	mapping, names, err := s.PrepareImportProfiles(t.Context(), []ImportProfileTarget{{AccountID: "existing", Target: existing.ID}, {AccountID: "new", Name: "Existing", Target: "new"}, {AccountID: "skip"}}, "Plex")
	if err != nil || len(mapping) != 2 || mapping["existing"] != existing.ID || names[mapping["new"]] != "Existing (Plex)" {
		t.Fatal(mapping, names, err)
	}
	count, err := s.CountUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, targets := range [][]ImportProfileTarget{
		{{AccountID: "one", Name: "New", Target: "new"}, {AccountID: "two", Target: "missing"}},
		{{AccountID: "one", Target: existing.ID}, {AccountID: "one", Name: "New", Target: "new"}},
		{{AccountID: "one", Target: ""}},
	} {
		if _, _, err = s.PrepareImportProfiles(t.Context(), targets, "Plex"); !errors.Is(err, ErrInvalidImportProfiles) {
			t.Fatal("invalid mapping accepted", err)
		}
	}
	after, err := s.CountUsers(t.Context())
	if err != nil || count != after {
		t.Fatal("invalid mappings mutated profiles", count, after, err)
	}
}
