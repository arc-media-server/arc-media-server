package store

import (
	"database/sql"
	"errors"
	"testing"
)

func TestPreferenceMutationRollsBackAndReleasesWriter(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	u, err := s.CreateUser(ctx, "Fixture", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetUserPrefs(ctx, u.ID, `{"theme":"light"}`); err != nil {
		t.Fatal(err)
	}
	rejected := errors.New("fixture validation failure")
	if err = s.UpdateUserPrefs(ctx, u.ID, func(raw string) (string, error) { return `{"theme":"dark"}`, rejected }); !errors.Is(err, rejected) {
		t.Fatal(err)
	}
	raw, err := s.UserPrefs(ctx, u.ID)
	if err != nil || raw != `{"theme":"light"}` {
		t.Fatal("failed mutation changed stored preferences", raw, err)
	}
	if err = s.SetUserPrefs(ctx, u.ID, `{}`); err != nil {
		t.Fatal("failed mutation kept the writer", err)
	}
	called := false
	if err = s.UpdateUserPrefs(ctx, "missing-profile", func(raw string) (string, error) { called = true; return raw, nil }); !errors.Is(err, sql.ErrNoRows) || called {
		t.Fatal("missing profile invoked mutation", err, called)
	}
}
