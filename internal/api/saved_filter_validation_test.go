package api

import (
	"ams/internal/store"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestSmartPlaylistRejectsInvalidSavedRulesBeforeCreation(t *testing.T) {
	s, u := securityServer(t)
	lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		filter rowFilter
	}{
		{"sort", rowFilter{Sort: "not-a-sort"}},
		{"negative days", rowFilter{AddedWithin: -1}},
		{"negative minimum", rowFilter{MinRuntime: -1}},
		{"negative maximum", rowFilter{MaxRuntime: -1}},
		{"rating above ten", rowFilter{MinRating: 11}},
		{"negative rating", rowFilter{MinRating: -1}},
		{"inverted runtime", rowFilter{MinRuntime: 120, MaxRuntime: 90}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.filter.LibraryID = lib
			body, _ := json.Marshal(map[string]any{"title": "Rules", "filter": tc.filter})
			r := httptest.NewRequest("POST", "/api/v1/playlists", bytes.NewReader(body))
			r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			w := httptest.NewRecorder()
			s.createPlaylist(w, r)
			if w.Code != 400 {
				t.Errorf("invalid %s accepted: %d %s", tc.name, w.Code, w.Body.String())
			}
			lists, err := s.store.Playlists(t.Context(), u.ID, access(r))
			if err != nil || len(lists) != 0 {
				t.Errorf("invalid rules persisted playlists: count=%d err=%v", len(lists), err)
			}
		})
	}
}

func TestSavedFilterRuntimeBoundsAgreeAcrossUses(t *testing.T) {
	f := rowFilter{LibraryID: 1, MinRuntime: 120, MaxRuntime: 90}
	p := prefs{Home: []homeRow{{ID: "custom:test", Title: "Test", Filter: &f}}}
	if p.validate() == nil {
		t.Error("home accepted inverted runtime")
	}
	c := store.Collection{LibraryID: 1, Kind: "smart", Title: "Test"}
	if applyCollectionBody(&c, &collectionBody{Filter: &f}) == "" {
		t.Error("collection accepted inverted runtime")
	}
}

func TestSavedRulesKeepValidOpenRangesAndFutureCodecIdentifiers(t *testing.T) {
	for _, f := range []rowFilter{
		{LibraryID: 1}, {LibraryID: 1, MinRuntime: 120}, {LibraryID: 1, MaxRuntime: 90},
		{LibraryID: 1, MinRuntime: 90, MaxRuntime: 90, MinRating: 10, Sort: "added"},
		{LibraryID: 1, VideoCodec: "future-codec.2", AudioCodec: "truehd", WatchState: "unwatched"},
	} {
		if err := f.validate(); err != nil {
			t.Errorf("valid rules rejected: %+v: %v", f, err)
		}
	}
	// A collection name is not a Home-row title: its existing 100-character limit applies.
	c := store.Collection{LibraryID: 1, Kind: "smart", Title: strings.Repeat("a", 80)}
	f := rowFilter{LibraryID: 1, Sort: "title"}
	if msg := applyCollectionBody(&c, &collectionBody{Filter: &f}); msg != "" {
		t.Fatal(msg)
	}
}

func TestSmartCollectionAndPlaylistEvaluateSameRules(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	libID, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	var want string
	for i, runtime := range []int{60, 90, 120} {
		title := fmt.Sprintf("Movie %d", i)
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: libID, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.UpdateMetadata(ctx, &store.Item{ID: id, Title: title, RuntimeMin: runtime, MetadataState: "matched"}); err != nil {
			t.Fatal(err)
		}
		fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: libID, Path: filepath.Join(t.TempDir(), title)}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		if runtime == 90 {
			want = id
		}
	}
	f := rowFilter{LibraryID: libID, MinRuntime: 80, MaxRuntime: 100, Sort: "title"}
	raw, _ := json.Marshal(f)
	r := httptest.NewRequest("GET", "/api/v1/playlists", nil)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	_, playlist, err := s.playlistContents(r, &store.Playlist{Filter: string(raw)})
	if err != nil || len(playlist) != 1 || playlist[0].ID != want {
		t.Fatalf("playlist: %+v %v", playlist, err)
	}
	collection, n, err := s.collectionItems(r, &store.Collection{LibraryID: libID, Kind: "smart", Filter: string(raw)}, 0)
	if err != nil || n != 1 || len(collection) != 1 || collection[0].ID != want {
		t.Fatalf("collection: %+v n=%d %v", collection, n, err)
	}
	// Caller access still wins over the filter's library identifier.
	u.LibraryIDs = []int64{}
	_, playlist, err = s.playlistContents(r, &store.Playlist{Filter: string(raw)})
	if err != nil || len(playlist) != 0 {
		t.Fatalf("hidden library returned playlist titles: %+v %v", playlist, err)
	}
}
