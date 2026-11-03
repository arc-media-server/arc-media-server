package store

import (
	"database/sql/driver"
	"strings"
	"testing"
	"time"
)

func TestPostgresBindingsPreserveLiteralsAndRepeatedSlots(t *testing.T) {
	q, a := postgresSQL("SELECT '?1', ?2, ?2, ?1 -- ?3\n", []driver.NamedValue{{Ordinal: 1, Value: "first"}, {Ordinal: 2, Value: "second"}})
	if !strings.Contains(q, "'?1'") || strings.Count(q, "$1") != 2 || len(a) != 2 || a[0].Value != "second" || a[1].Value != "first" {
		t.Fatal(q, a)
	}
	q, a = postgresSQL("DELETE FROM items WHERE id=?2", []driver.NamedValue{{Ordinal: 1, Value: "unused"}, {Ordinal: 2, Value: "used"}})
	if strings.Contains(q, "$2") || len(a) != 1 || a[0].Value != "used" {
		t.Fatal(q, a)
	}
}
func TestPostgresLikeConcatenationAndLocalYearBoundaries(t *testing.T) {
	st := postgresFixture(t)
	var found bool
	if err := st.db.QueryRowContext(t.Context(), "SELECT 'TMDB_group' LIKE ? || '%'", "tmdb").Scan(&found); err != nil || !found {
		t.Fatal("case/concatenation differs", found, err)
	}
	if err := st.db.QueryRowContext(t.Context(), `SELECT 'a\b' LIKE ?`, `a\b`).Scan(&found); err != nil || !found {
		t.Fatal("backslash literal differs", err)
	}
	lib, err := st.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(t.Context(), "Owner", true, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	yearBoundary := time.Date(2025, 12, 31, 12, 30, 0, 0, time.UTC)
	if _, err = st.AddPlay(t.Context(), Play{UserID: u.ID, Kind: "movie", LibraryID: lib, Title: "Fixture", StartedAt: yearBoundary.Unix()}); err != nil {
		t.Fatal(err)
	}
	years, err := st.playYearsIn(t.Context(), u.ID, time.FixedZone("fixture+14", 14*3600))
	if err != nil || len(years) != 1 || years[0] != 2026 {
		t.Fatal("server-local year was replaced by database timezone", years, err)
	}
}
