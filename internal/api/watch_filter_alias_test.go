package api

import (
	"ams/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"testing"
)

// Saved filter aliases: advertised aliases must produce the same final episode selection.
func TestUnwatchedAliasesSelectSamePlaylistEpisodes(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	lib, err := s.store.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	show, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Fixture"}, "fixture-show")
	if err != nil {
		t.Fatal(err)
	}
	var episodes []string
	for i := 0; i < 2; i++ {
		ep, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, ParentID: show, Kind: "episode", Title: fmt.Sprintf("Episode %d", i)}, fmt.Sprintf("fixture-episode-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: fmt.Sprintf("/fixture/alias-%d.mkv", i), Size: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetFileItems(ctx, fid, []string{ep}, 0); err != nil {
			t.Fatal(err)
		}
		episodes = append(episodes, ep)
	}
	if err := s.store.SetPlayed(ctx, u.ID, episodes[:1], true); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/v1/playlists/fixture", nil)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	results := make([][]string, 0, 2)
	for _, f := range []rowFilter{{LibraryID: lib, Unwatched: true}, {LibraryID: lib, WatchState: "unwatched"}} {
		if err := f.validate(); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		_, items, err := s.playlistContents(r, &store.Playlist{Filter: string(raw)})
		if err != nil {
			t.Fatal(err)
		}
		var ids, titles []string
		for _, item := range items {
			ids = append(ids, item.ID)
			titles = append(titles, item.Title)
		}
		t.Logf("filter=%s returned_titles=%v", raw, titles)
		results = append(results, ids)
	}
	if len(results[0]) != 1 || results[0][0] != episodes[1] {
		t.Fatalf("control unwatched=true fixture incorrect: %v", results[0])
	}
	if !reflect.DeepEqual(results[0], results[1]) {
		t.Fatalf("advertised aliases disagree: unwatched=true returned %d episodes; watch_state=unwatched returned %d (includes already watched)", len(results[0]), len(results[1]))
	}
}
