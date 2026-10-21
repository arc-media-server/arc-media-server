package scanner

import (
	"context"
	"testing"

	"ams/internal/metadata"
	"ams/internal/store"
)

func TestEpisodeAdoptionAndProviderRekeyPreserveReferences(t *testing.T) {
	ctx := context.Background()
	st, s, lib, _ := libraryFixture(t, "shows", map[string]string{"Example/Example S01E01.mkv": "episode"})
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	shows, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	if err != nil || len(shows) != 1 {
		t.Fatalf("shows=%v err=%v", shows, err)
	}
	show := shows[0].ID
	kids, err := st.Children(ctx, show)
	if err != nil || len(kids) != 1 {
		t.Fatalf("episodes=%v err=%v", kids, err)
	}
	id := kids[0].ID
	files, err := st.ItemFiles(ctx, id)
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	fid := files[0].ID
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, u.ID, id, 120000, 3600000); err != nil {
		t.Fatal(err)
	}
	play, err := st.AddPlay(ctx, store.Play{UserID: u.ID, ItemID: id, Kind: "episode", LibraryID: lib, SeriesID: show, Title: "Original", StartedAt: 1, EndedAt: 2, WatchedMS: 120000, PositionMS: 120000, DurationMS: 3600000})
	if err != nil {
		t.Fatal(err)
	}
	title := "My episode title"
	if err := st.EditItem(ctx, id, store.ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	orders, err := st.ListOrderings(ctx, show)
	if err != nil || len(orders) != 1 {
		t.Fatalf("orders=%v err=%v", orders, err)
	}
	custom, err := st.CopyOrdering(ctx, &orders[0], "My order")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetEntry(ctx, custom, id, 2, 3); err != nil {
		t.Fatal(err)
	}
	oldKey := fileEpisodeKey(show, false, 1, 1)
	// Exercise the same adoption/rekey helper used by SyncSeriesMetadata: a file-only
	// episode is learned by TVDB, then switches to TMDB and back without changing its ID.
	for _, tc := range []struct {
		provider   string
		tmdb, tvdb int64
	}{
		{"tvdb", 0, 200}, {"tmdb", 100, 200}, {"tvdb", 100, 200},
	} {
		number := tc.tvdb
		if tc.provider == "tmdb" {
			number = tc.tmdb
		}
		key := episodeKey(show, tc.provider, number)
		if err := s.adoptEpisode(ctx, lib, oldKey, key); err != nil {
			t.Fatal(err)
		}
		it, err := st.GetItemByKey(ctx, lib, key)
		if err != nil || it == nil || it.ID != id {
			t.Fatalf("%s changed episode identity: %+v %v", tc.provider, it, err)
		}
		if err := st.UpdateMetadata(ctx, &store.Item{ID: id, Title: "Provider title", TMDBID: tc.tmdb, TVDBID: tc.tvdb, ReleaseDate: "2020-01-01", MetadataState: "matched"}); err != nil {
			t.Fatal(err)
		}
		it, err = st.GetItem(ctx, id)
		if err != nil || it.Title != title {
			t.Fatalf("locked edit lost: %+v %v", it, err)
		}
		files, err = st.ItemFiles(ctx, id)
		if err != nil || len(files) != 1 || files[0].ID != fid {
			t.Fatalf("file reference lost: %v %v", files, err)
		}
		data, err := st.UserDataFor(ctx, u.ID, []string{id})
		if err != nil || data[id].PositionMS != 120000 {
			t.Fatalf("resume lost: %v %v", data, err)
		}
		history, err := st.Plays(ctx, store.PlayFilter{UserID: u.ID})
		if err != nil || len(history) != 1 || history[0].ID != play || history[0].ItemID != id {
			t.Fatalf("history lost: %v %v", history, err)
		}
		entries, err := st.OrderingEntries(ctx, custom)
		if err != nil || len(entries) != 1 || entries[0].EpisodeID != id || !entries[0].Locked || entries[0].Season != 2 || entries[0].Episode != 3 {
			t.Fatalf("manual placement lost: %v %v", entries, err)
		}
		oldKey = key
	}
	// Cached cross-provider IDs should still map if providers disagree about titles/dates.
	mapped, err := s.mapEpisodes(ctx, show, "tmdb", 0,
		map[int64]*metadata.Episode{100: {ID: 100, Title: "TMDB title", AirDate: "2020-01-02"}},
		map[int64]*metadata.Episode{200: {ID: 200, Title: "TVDB title", AirDate: "2020-01-01"}}, false)
	if err != nil || mapped[200] != 100 {
		t.Fatalf("cached mapping lost: %v %v", mapped, err)
	}
}
