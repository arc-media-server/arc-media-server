package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBrowseDirectionPreservesFiltersAndPaging(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Fixture", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for i, title := range []string{"Alpha", "Zulu", "Long movie"} {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title, Year: 2000 + i}, title)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.db.ExecContext(ctx, "UPDATE items SET runtime_min = ?, rating = 8 WHERE id = ?", 80+i*10, id); err != nil {
			t.Fatal(err)
		}
		fid, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), title+".mkv")}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
	}
	for _, reverse := range []bool{false, true} {
		q := ItemQuery{LibraryID: lib, Kind: "movie", Sort: "title", Reverse: reverse, MaxRuntime: 90, MinRating: 8, Limit: 1}
		first, total, err := st.ListItems(ctx, q)
		if err != nil || total != 2 || len(first) != 1 {
			t.Fatalf("first page: %v %d %v", first, total, err)
		}
		want := "Alpha"
		if reverse {
			want = "Zulu"
		}
		if first[0].Title != want {
			t.Fatalf("reverse=%v got %s, want %s", reverse, first[0].Title, want)
		}
		q.Offset = 1
		second, total, err := st.ListItems(ctx, q)
		if err != nil || total != 2 || len(second) != 1 || second[0].ID == first[0].ID {
			t.Fatalf("second page: %v %d %v", second, total, err)
		}
	}
}
