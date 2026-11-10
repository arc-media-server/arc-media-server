package store

import "testing"

func TestBulkRatingAgeFailureRollsBackRatingAndLabels(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `CREATE TRIGGER reject_age BEFORE UPDATE OF min_age ON items BEGIN SELECT RAISE(ABORT,'fixture age write refused'); END`); err != nil {
		t.Fatal(err)
	}
	rating := "18"
	if err := st.EditItems(ctx, []string{id}, BulkEdit{AddLabels: []string{"Changed"}, ContentRating: &rating}); err == nil {
		t.Fatal("expected age failure")
	}
	it, err := st.GetItem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	labels, err := st.ItemLabels(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if it.ContentRating != "" || len(it.LockedFields) > 0 || len(labels) > 0 {
		t.Fatalf("failed age update committed metadata: rating=%q labels=%v locks=%v", it.ContentRating, labels, it.LockedFields)
	}
}
