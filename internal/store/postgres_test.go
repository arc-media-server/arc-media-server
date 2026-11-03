package store

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
)

func postgresTarget(t testing.TB) string {
	t.Helper()
	dsn := os.Getenv("ARC_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("ARC_TEST_POSTGRES not configured: real PostgreSQL integration required")
	}
	raw, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	schema := "arc_test_" + NewID()
	if _, err = raw.ExecContext(context.Background(), "CREATE SCHEMA "+quoted(schema)); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	target := dsn + separator + "search_path=" + schema
	t.Cleanup(func() {
		db, e := sql.Open("pgx", dsn)
		if e == nil {
			defer db.Close()
			db.ExecContext(context.Background(), "DROP SCHEMA "+quoted(schema)+" CASCADE")
		}
	})
	return target
}
func postgresFixture(t testing.TB) *Store {
	t.Helper()
	dsn := postgresTarget(t)
	st, err := OpenPostgres(context.Background(), dsn, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestPostgresOpen(t *testing.T) {
	st := postgresFixture(t)
	if st.Backend() != "postgres" {
		t.Fatal(st.Backend())
	}
	if _, err := st.CreateUser(t.Context(), "Fixture", true, "", "hash", ""); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresWithoutRowIDDoesNotAllocateForeignKey(t *testing.T) {
	st := postgresFixture(t)
	var seq sql.NullString
	if err := st.db.QueryRowContext(t.Context(), "SELECT pg_get_serial_sequence('library_recommended','library_id')").Scan(&seq); err != nil || seq.Valid {
		t.Fatalf("WITHOUT ROWID foreign key gained an allocator: %+v %v", seq, err)
	}
	if err := st.db.QueryRowContext(t.Context(), "INSERT INTO settings(key,value) VALUES('forbidden-reader','x') RETURNING key").Scan(new(string)); err == nil {
		t.Fatal("read pool admitted a write")
	}
	if err := st.SetSetting(t.Context(), "writer-still-works", "yes"); err != nil {
		t.Fatal(err)
	}
}
