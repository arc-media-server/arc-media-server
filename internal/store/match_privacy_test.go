package store

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchTraceErrorPrivacyOnWriteReadAndUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(t.Context(), path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	lib, err := s.SyncLibrary(t.Context(), "Movies", "movies", "tvdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.EnsureItem(t.Context(), &Item{LibraryID: lib, Kind: "movie", Title: "Example"}, "movie:fixture")
	if err != nil {
		t.Fatal(err)
	}
	unsafe := `{"steps":[{"provider":"tmdb","error":"Get https://provider.example/search?api_key=fixture-private-key: connection refused"}]}`
	if err := s.SetMatchInfo(t.Context(), id, unsafe); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := s.db.QueryRowContext(t.Context(), `SELECT match_info FROM items WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "fixture-private-key") {
		t.Error("new persisted match explanation contains credential")
	}
	// Legacy rows must also be safe to read, before the next refresh.
	if _, err := s.db.ExecContext(t.Context(), `UPDATE items SET match_info=? WHERE id=?`, unsafe, id); err != nil {
		t.Fatal(err)
	}
	raw, err = s.MatchInfo(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "fixture-private-key") {
		t.Error("legacy match explanation exposes credential")
	}
	// Simulate upgrading a v63 database: the new cleanup migration has not run.
	if _, err := s.db.ExecContext(t.Context(), `DELETE FROM schema_migrations WHERE version LIKE '0064%'`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(t.Context(), path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(t.Context(), `SELECT match_info FROM items WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "fixture-private-key") {
		t.Error("upgraded database retains credential in trace for future backups")
	}
}
