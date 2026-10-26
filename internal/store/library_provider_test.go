package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLibraryProviderUpgradePreservesStoredMetadata(t *testing.T) {
	ctx := t.Context()
	old := oldBackupThrough(t, 52)
	db, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE items SET details_provider='tvdb' WHERE id='movie'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	backup := filepath.Join(t.TempDir(), "before-provider.db")
	st, err := Open(ctx, old, Options{BeforeMigrate: func(st *Store) error { return st.BackupTo(ctx, backup) }})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	libs, err := st.ListLibraries(ctx)
	if err != nil || len(libs) == 0 {
		t.Fatalf("libraries=%v %v", libs, err)
	}
	lib := libs[0]
	if lib.DetailsProvider != "" {
		t.Fatal("old library did not retain TMDB default")
	}
	it, err := st.GetItem(ctx, "movie")
	if err != nil || it.Title != "My edited title" || it.DetailsProvider != "tvdb" {
		t.Fatalf("stored metadata/legacy record changed: %+v %v", it, err)
	}
	if err := st.SetLibraryDetailsProvider(ctx, lib.ID, "tvdb"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetLibrary(ctx, lib.ID)
	if err != nil || got.DetailsProvider != "tvdb" || got.SourceOrdering != lib.SourceOrdering {
		t.Fatalf("independent provider=%+v %v", got, err)
	}
	if err := st.SetLibraryDetailsProvider(ctx, lib.ID, "unknown"); err == nil {
		t.Fatal("accepted invalid provider")
	}

	if _, err := st.SyncLibrary(ctx, "Occupied", "shows", "tmdb_aired", []string{t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateLibrary(ctx, lib.ID, "Occupied", lib.SourceOrdering, lib.Paths, ""); err != ErrDuplicateName {
		t.Fatalf("duplicate=%v", err)
	}
	unchanged, err := st.GetLibrary(ctx, lib.ID)
	if err != nil || unchanged.DetailsProvider != "tvdb" {
		t.Fatal("failed library edit changed provider")
	}

	raw, err := sql.Open("sqlite", "file:"+backup+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	var title string
	if err := raw.QueryRow("SELECT title FROM items WHERE id='movie'").Scan(&title); err != nil || title != it.Title {
		t.Fatal("pre-upgrade backup lost metadata")
	}
	var added int
	if err := raw.QueryRow("SELECT COUNT(*) FROM pragma_table_info('libraries') WHERE name='details_provider'").Scan(&added); err != nil || added != 0 {
		t.Fatal("pre-upgrade backup schema was modified")
	}
}
