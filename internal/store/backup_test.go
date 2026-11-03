package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// Construct an actual earlier schema, rather than deleting migration records
// from a current database (which does not simulate an upgrade).
func oldBackup(t *testing.T) string {
	return oldBackupThrough(t, 3)
}

func oldBackupThrough(t *testing.T, count int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	names, err := migrationNames()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names[:count] {
		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO schema_migrations VALUES (?, 1)`, migrationVersion(name)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO libraries (id,name,kind,created_at) VALUES (1,'Preserved','movies',1);
	INSERT INTO items (id,library_id,kind,match_key,title,sort_title,locked_fields,added_at,updated_at)
	VALUES ('movie',1,'movie','test','My edited title','my edited title','title',1,1)`); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRestoreEarlierSchemaRebuildsRatingAges(t *testing.T) {
	ctx := context.Background()
	p := oldBackupThrough(t, 8) // Rating labels existed before derived parental ages.
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE items SET content_rating='15' WHERE id='movie'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s := openSQLiteTest(t)
	if err := s.RestoreFrom(ctx, p); err != nil {
		t.Fatal(err)
	}
	var age int
	if err := s.db.QueryRow(`SELECT min_age FROM items WHERE id='movie'`).Scan(&age); err != nil {
		t.Fatal(err)
	}
	if age != 15 {
		t.Fatalf("restored rating has age %d, want 15 without restarting Arc", age)
	}
}

func TestRestoreEarlierSchema(t *testing.T) {
	ctx := context.Background()
	p := oldBackup(t)
	if err := CheckBackup(ctx, p); err != nil {
		t.Fatal(err)
	}
	s, err := Open(ctx, filepath.Join(t.TempDir(), "live.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.RestoreFrom(ctx, p); err != nil {
		t.Fatal(err)
	}
	var title, locks string
	if err := s.db.QueryRow(`SELECT title, locked_fields FROM items WHERE id='movie'`).Scan(&title, &locks); err != nil {
		t.Fatal(err)
	}
	if title != "My edited title" || locks != "title" {
		t.Fatalf("edits lost: %q %q", title, locks)
	}
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	names, _ := migrationNames()
	if count != len(names) {
		t.Fatalf("applied %d migrations, want %d", count, len(names))
	}
	// Confirm a column introduced much later is usable after the online restore.
	if _, err := s.db.Exec(`SELECT device FROM sessions LIMIT 1`); err != nil {
		t.Fatal(err)
	}
}

func TestRejectIncompleteBackupHistory(t *testing.T) {
	for _, query := range []string{
		`DELETE FROM schema_migrations`,
		`DELETE FROM schema_migrations WHERE version='0002_tv'`,
		`INSERT INTO schema_migrations VALUES ('9999_future',1)`,
	} {
		t.Run(query, func(t *testing.T) {
			p := oldBackup(t)
			db, err := sql.Open("sqlite", p)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
			db.Close()
			if err := CheckBackup(context.Background(), p); err == nil {
				t.Fatal("invalid backup accepted")
			}
		})
	}
}
