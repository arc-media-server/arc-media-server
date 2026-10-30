package store

import (
	"testing"
)

func TestUnmatchRollbackAndSavedReferences(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	show, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Provider", Overview: "Keep on failure"}, "show")
	if err != nil {
		t.Fatal(err)
	}
	child, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: show, Title: "Provider episode"}, "episode")
	if err != nil {
		t.Fatal(err)
	}
	// Fail after the first item reset to establish transaction-wide rollback.
	if _, err = st.db.Exec(`CREATE TRIGGER refuse_reset BEFORE DELETE ON item_genres BEGIN SELECT RAISE(ABORT,'fixture failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = st.db.Exec("INSERT INTO item_genres(item_id,name) VALUES(?, 'Drama')", show); err != nil {
		t.Fatal(err)
	}
	if err = st.Unmatch(ctx, show, "Filename", 0, nil); err == nil {
		t.Fatal("expected reset failure")
	}
	got, _ := st.GetItem(ctx, show)
	if got.MatchSuppressed || got.Title != "Provider" {
		t.Fatalf("partial reset %+v", got)
	}
	if _, err = st.db.Exec("DROP TRIGGER refuse_reset"); err != nil {
		t.Fatal(err)
	}
	user, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	playlist, err := st.CreatePlaylist(ctx, user.ID, "History")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.AddToPlaylist(ctx, playlist.ID, []string{child}); err != nil {
		t.Fatal(err)
	}
	if err = st.Unmatch(ctx, show, "Filename", 0, nil); err != nil {
		t.Fatal(err)
	}
	retained, err := st.GetItem(ctx, child)
	if err != nil || retained == nil {
		t.Fatal("fileless playlist episode deleted", err)
	}
	if _, err = st.db.Exec("UPDATE items SET metadata_state='unmatched' WHERE id=?", show); err != nil {
		t.Fatal(err)
	}
	if err = st.RetryUnmatched(ctx, lib, 1<<62); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetItem(ctx, show)
	if got.MetadataState != "unmatched" || !got.MatchSuppressed {
		t.Fatal("retry revived explicitly unmatched show")
	}
}
