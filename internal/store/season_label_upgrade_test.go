package store

import (
	"path/filepath"
	"testing"

	"ams/internal/perftest"
)

func TestPostgresSeasonLabelUpgrade(t *testing.T) {
	dsn, ok := perftest.PostgresDatabase(t)
	if !ok {
		t.Skip("requires a disposable PostgreSQL parity database")
	}
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	current, err := currentPostgresSchema()
	check(err)
	previous, err := newPostgresSchema(current.versions[:len(current.versions)-1], current.steps[:len(current.steps)-1])
	check(err)
	ctx := t.Context()
	old, err := openPostgres(ctx, dsn, "", previous, postgresBinding{})
	check(err)
	t.Cleanup(func() { old.Close() })
	lib, err := old.SyncLibrary(ctx, "Fixture shows", "shows", "tmdb_aired", nil)
	check(err)
	show, _, err := old.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Fixture show"}, "show")
	check(err)
	check(old.Close())

	backups := t.TempDir()
	upgraded, err := OpenPostgres(ctx, dsn, backups)
	check(err)
	defer upgraded.Close()
	library, err := upgraded.GetLibrary(ctx, lib)
	check(err)
	item, err := upgraded.GetItem(ctx, show)
	check(err)
	if library == nil || library.SeasonLabelMode != "number" || item == nil || item.SeasonLabelMode != "" {
		t.Fatalf("upgraded rows did not retain their identities with default labels: %+v %+v", library, item)
	}
	archives, err := filepath.Glob(filepath.Join(backups, "*-before-upgrade.pg.zip"))
	check(err)
	if len(archives) != 1 {
		t.Fatalf("expected one safety backup, found %d", len(archives))
	}
}
