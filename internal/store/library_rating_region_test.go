package store

import (
	"database/sql"
	"testing"
)

// Libraries from before per-library age ratings keep the server-wide region they used, and
// changing a library's country has only that library's titles fetched again.
func TestLibraryRatingRegionUpgradeKeepsServerRegion(t *testing.T) {
	ctx := t.Context()
	old := oldBackupThrough(t, 55)
	db, err := sql.Open("sqlite", old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO settings (key, value) VALUES ('region', 'GB') ON CONFLICT(key) DO UPDATE SET value = excluded.value`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE items SET synced_at = 123`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	st, err := Open(ctx, old, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	libs, err := st.ListLibraries(ctx)
	if err != nil || len(libs) == 0 {
		t.Fatalf("libraries=%v %v", libs, err)
	}
	for _, lib := range libs {
		if lib.RatingRegion != "GB" {
			t.Fatalf("library %q rating region %q, want the server's GB", lib.Name, lib.RatingRegion)
		}
	}
	_, err = st.SyncLibrary(ctx, "Other", "movies", "", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetLibraryRatingRegion(ctx, libs[0].ID, "US"); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetLibrary(ctx, libs[0].ID)
	if err != nil || got.RatingRegion != "US" {
		t.Fatalf("library=%+v %v", got, err)
	}
	var stale, kept int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE library_id = ? AND kind IN ('movie','series') AND synced_at = 0`, libs[0].ID).Scan(&stale); err != nil || stale == 0 {
		t.Fatalf("changed library's titles not marked for refresh: %d %v", stale, err)
	}
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE library_id <> ? AND synced_at = 0`, libs[0].ID).Scan(&kept); err != nil || kept != 0 {
		t.Fatalf("other libraries' titles marked for refresh: %d %v", kept, err)
	}
}
