package store

import (
	"context"
	"testing"
)

func TestHistoryAndSizes(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/m"})
	sam, _ := st.CreateUser(ctx, "Sam", false, "blue", "", "")
	add := func(title string, size int64) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: size, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		return id
	}
	big, small, started := add("Big", 5000), add("Small", 100), add("Started", 300)

	id, err := st.AddPlay(ctx, Play{UserID: sam.ID, ItemID: small, Kind: "movie", LibraryID: lib, Title: "Small",
		StartedAt: 100, EndedAt: 200, WatchedMS: 60_000, Method: "direct"})
	if err != nil {
		t.Fatal(err)
	}
	st.UpdatePlay(ctx, Play{ID: id, EndedAt: 300, WatchedMS: 90_000, Completed: true})
	st.UpdatePlay(ctx, Play{ID: id, EndedAt: 400, WatchedMS: 95_000}) // stays completed
	st.ReportProgress(ctx, sam.ID, started, 120_000, 6_000_000)

	plays, err := st.Plays(ctx, PlayFilter{UserID: sam.ID})
	if err != nil || len(plays) != 1 || !plays[0].Completed || plays[0].WatchedMS != 95_000 || plays[0].Method != "direct" {
		t.Fatalf("plays: %+v %v", plays, err)
	}
	if none, _ := st.Plays(ctx, PlayFilter{From: 150}); len(none) != 0 {
		t.Errorf("from filter: %+v", none)
	}

	sizes, err := st.TitleSizes(ctx)
	if err != nil || len(sizes) != 3 {
		t.Fatalf("sizes: %+v %v", sizes, err)
	}
	if sizes[0].ItemID != big || sizes[0].Size != 5000 || sizes[0].Watched {
		t.Errorf("biggest: %+v", sizes[0])
	}
	watched := map[string]bool{}
	for _, s := range sizes {
		watched[s.ItemID] = s.Watched
	}
	if !watched[small] || !watched[started] || watched[big] {
		t.Errorf("watched: %v", watched)
	}
}
