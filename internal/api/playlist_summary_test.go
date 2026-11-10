package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"

	"ams/internal/store"
)

func TestPlaylistListingKeepsCountsDurationAndDistinctPosterOrder(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	tv, err := s.store.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	add := func(library int64, kind, title, parent, poster string) string {
		t.Helper()
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: library, Kind: kind, Title: title, ParentID: parent}, title)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.UpdateMetadata(ctx, &store.Item{ID: id, Title: title, RuntimeMin: 90, PosterPath: poster, MetadataState: "matched"}); err != nil {
			t.Fatal(err)
		}
		if kind != "series" {
			fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: library, Path: filepath.Join(t.TempDir(), title)}, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	blank := add(lib, "movie", "Blank", "", "")
	a := add(lib, "movie", "Art A", "", "/a.jpg")
	b := add(lib, "movie", "Art B", "", "/b.jpg")
	c := add(lib, "movie", "Art C", "", "/c.jpg")
	// A locked provider poster wins over a local sidecar in the full DTO.
	if err := s.store.SetLocalArt(ctx, a, "/fixture-sidecar.jpg", "", ""); err != nil {
		t.Fatal(err)
	}
	locks := []string{"poster"}
	if err := s.store.EditItem(ctx, a, store.ItemEdit{Locked: locks}); err != nil {
		t.Fatal(err)
	}

	showA, showB := add(tv, "series", "Show A", "", ""), add(tv, "series", "Show B", "", "")
	ep1, ep2 := add(tv, "episode", "A1", showA, ""), add(tv, "episode", "A2", showA, "")
	ep3 := add(tv, "episode", "B1", showB, "")
	manual, err := s.store.CreatePlaylist(ctx, u.ID, "Manual")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.AddToPlaylist(ctx, manual.ID, []string{blank, a, ep1, ep2, ep3, b, c, a}); err != nil {
		t.Fatal(err)
	}
	smart, err := s.store.CreatePlaylist(ctx, u.ID, "Smart")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetPlaylistFilter(ctx, smart.ID, fmt.Sprintf("{\"library_id\":%d,\"sort\":\"title\"}", lib)); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/playlists", nil)
	request = request.WithContext(context.WithValue(request.Context(), userKey, u))
	list := func() map[string]playlistDTO {
		response := httptest.NewRecorder()
		s.listPlaylists(response, request)
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		var values []playlistDTO
		if err := json.Unmarshal(response.Body.Bytes(), &values); err != nil {
			t.Fatal(err)
		}
		out := map[string]playlistDTO{}
		for _, p := range values {
			out[p.ID] = p
		}
		return out
	}
	got := list()
	ia, _ := s.store.GetItem(ctx, a)
	ib, _ := s.store.GetItem(ctx, b)
	ic, _ := s.store.GetItem(ctx, c)
	want := []string{toDTO(*ia).Images["primary"], "/api/items/" + showA + "/images/primary", "/api/items/" + showB + "/images/primary", toDTO(*ib).Images["primary"]}
	if p := got[manual.ID]; p.Count != 8 || p.DurationMS != 8*90*60000 || !slices.Equal(p.Images, want) {
		t.Fatalf("manual changed: %+v wantimages=%v", p, want)
	}
	want = []string{toDTO(*ia).Images["primary"], toDTO(*ib).Images["primary"], toDTO(*ic).Images["primary"]}
	if p := got[smart.ID]; p.Count != 4 || p.DurationMS != 4*90*60000 || !slices.Equal(p.Images, want) {
		t.Fatalf("smart changed: %+v wantimages=%v", p, want)
	}
	// Keep per-entry visibility and totals when the TV library is excluded.
	u.LibraryIDs = []int64{lib}
	got = list()
	if p := got[manual.ID]; p.Count != 5 || p.DurationMS != 5*90*60000 || !slices.Equal(p.Images, want) {
		t.Fatalf("access-controlled manual changed: %+v", p)
	}
	u.LibraryIDs = []int64{}
	got = list()
	for _, p := range got {
		if p.Count != 0 || p.DurationMS != 0 || len(p.Images) != 0 {
			t.Fatalf("hidden contents leaked: %+v", p)
		}
	}
}
