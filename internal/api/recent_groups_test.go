package api

import (
	"ams/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestRecentGroupsAreNotStarvedByLargeImports(t *testing.T) {
	ctx := context.Background()
	s, _ := securityServer(t)
	lib, err := s.store.SyncLibrary(ctx, "Series", "shows", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for show := 1; show <= 6; show++ {
		title := fmt.Sprintf("Show %d", show)
		sid, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		for ep := 1; ep <= 100; ep++ {
			key := fmt.Sprintf("%d:%d", show, ep)
			id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "episode", ParentID: sid, Title: key}, key)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.store.UpdateMetadata(ctx, &store.Item{ID: id, Title: key, MetadataState: "matched", ReleaseDate: fmt.Sprintf("2020-%02d-01", show)}); err != nil {
				t.Fatal(err)
			}
			f := &store.MediaFile{LibraryID: lib, Path: "/isolated/" + key + ".mkv", Size: 1, DurationMS: 1000}
			fid, err := s.store.SaveFile(ctx, f, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	r := httptest.NewRequest("GET", "/api/libraries/1/recent-shows?limit=24", nil)
	r.SetPathValue("id", strconv.FormatInt(lib, 10))
	w := httptest.NewRecorder()
	s.recentlyAddedShows(w, r)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var cards []recentCard
	if err := json.Unmarshal(w.Body.Bytes(), &cards); err != nil {
		t.Fatal(err)
	}
	for _, c := range cards {
		if c.Episodes != 100 {
			t.Fatalf("partial group: %+v", c)
		}
		t.Logf("card=%s episodes=%d", c.Item.Title, c.Episodes)
	}
	t.Logf("six present shows / 600 episodes / requested 24 cards -> returned %d cards", len(cards))
	denied, err := s.store.RecentEpisodes(ctx, lib, store.Access{Libraries: []int64{lib + 1}}, 24)
	if err != nil || len(denied) != 0 {
		t.Fatalf("access leak: %v %v", denied, err)
	}
	if len(cards) != 6 {
		t.Fatalf("expected all six complete show groups; cards=%d", len(cards))
	}
}
