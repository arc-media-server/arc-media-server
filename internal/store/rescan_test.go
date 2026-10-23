package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestUnchangedLocalAssetsDoNotWaitForWriter(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Example"}, "example")
	if err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{st.SetLocalArt(ctx, id, "poster", "backdrop", "logo"), st.SetLocalTheme(ctx, id, "theme"), st.SetSeasonLocalPosters(ctx, id, map[int]string{1: "season"})} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.SetSeasonChosen(ctx, id, 1, "chosen"); err != nil {
		t.Fatal(err)
	}
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	read, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for _, err := range []error{st.SetLocalArt(read, id, "poster", "backdrop", "logo"), st.SetLocalTheme(read, id, "theme"), st.SetSeasonLocalPosters(read, id, map[int]string{1: "season", 2: ""})} {
		if err != nil {
			t.Fatalf("unchanged assets entered writer queue: %v", err)
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{st.SetLocalArt(ctx, id, "new", "", ""), st.SetLocalTheme(ctx, id, ""), st.SetSeasonLocalPosters(ctx, id, nil)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	item, err := st.GetItem(ctx, id)
	if err != nil || item.LocalPoster != "new" || item.LocalBackdrop != "" || item.LocalTheme != "" {
		t.Fatalf("changed assets lost: %+v %v", item, err)
	}
	art, err := st.SeasonPosters(ctx, id)
	if err != nil || art[1].Local != "" || art[1].Chosen != "chosen" {
		t.Fatalf("chosen season art lost: %+v %v", art, err)
	}
}

func TestFileLinkSnapshotsAcrossBatchesAndDuplicates(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Example"}, "example")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for n := range 270 {
		id, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), fmt.Sprint(n)+".mkv")}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetFileItems(ctx, id, []string{item}, n%3); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	ids = append(ids, ids[0])
	links, err := st.FileLinks(ctx, ids)
	if err != nil || len(links) != 270 || len(links[ids[0]]) != 1 {
		t.Fatalf("snapshot: %d %v", len(links), err)
	}
	for n, id := range ids[:270] {
		if len(links[id]) != 1 || links[id][0].ItemID != item || links[id][0].Part != n%3 {
			t.Fatalf("wrong link %d: %+v", id, links[id])
		}
	}
}
