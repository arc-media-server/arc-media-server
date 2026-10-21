package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
)

func TestOpenRejectsFutureSchemaBeforeUpgrade(t *testing.T) {
	ctx := context.Background()
	p := oldBackup(t)
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations VALUES ('9999_future',1)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	called := false
	st, err := Open(ctx, p, Options{BeforeMigrate: func(*Store) error { called = true; return nil }})
	if st != nil {
		st.Close()
		t.Fatal("opened a newer database")
	}
	if err == nil || !strings.Contains(err.Error(), "compatible backup") || called {
		t.Fatalf("err=%v migration hook called=%v", err, called)
	}
	db, err = sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("rejected startup changed migrations: %d", count)
	}
	var title string
	if err := db.QueryRow(`SELECT title FROM items WHERE id='movie'`).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "My edited title" {
		t.Fatalf("rejected startup changed title: %q", title)
	}
}

func TestUpgradeStopsWhenSafetyBackupFails(t *testing.T) {
	ctx := context.Background()
	p := oldBackup(t)
	blocked := errors.New("backup storage unavailable")
	st, err := Open(ctx, p, Options{BeforeMigrate: func(*Store) error { return blocked }})
	if st != nil {
		st.Close()
		t.Fatal("upgraded without safety backup")
	}
	if !errors.Is(err, blocked) {
		t.Fatalf("err=%v", err)
	}
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if count != 3 {
		t.Fatalf("failed backup allowed upgrade: %d migrations", count)
	}
	called := false
	st, err = Open(ctx, p, Options{BeforeMigrate: func(s *Store) error {
		called = true
		return s.BackupTo(ctx, p+".safety")
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if !called {
		t.Fatal("upgrade skipped safety backup")
	}
	if err := CheckBackup(ctx, p+".safety"); err != nil {
		t.Fatal(err)
	}
}
