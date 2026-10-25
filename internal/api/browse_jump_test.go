package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
)

func TestBrowseJumpValidationAndProfileBoundary(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := context.Background()
	lib, err := s.store.SyncLibrary(ctx, "Jump fixture", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{"Alpha", "Sierra", "Zulu"} {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), title)}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.CreateSession(ctx, auth.HashToken("jump-viewer"), viewer.ID, "password", "fixture"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query          string
		status, offset int
		title          string
	}{
		{"jump=S&limit=1", 200, 1, "Sierra"},
		{"jump=s&reverse=1&limit=1", 200, 1, "Sierra"},
		{"offset=2&limit=1", 200, 2, "Zulu"},
		{"jump=SS", 400, 0, ""}, {"jump=s&sort=rating", 400, 0, ""}, {"jump=%27", 400, 0, ""},
	} {
		r := httptest.NewRequest("GET", "/api/items?"+tc.query, nil)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "jump-viewer"})
		w := httptest.NewRecorder()
		s.signedIn(s.listItems)(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: status=%d %s", tc.query, w.Code, w.Body.String())
		}
		if tc.status != 200 {
			continue
		}
		var page struct {
			Total, Offset int
			Items         []struct{ Title string }
		}
		if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if page.Total != 3 || page.Offset != tc.offset || len(page.Items) != 1 || page.Items[0].Title != tc.title {
			t.Fatalf("%s: %+v", tc.query, page)
		}
	}
	viewer.LibraryIDs = []int64{}
	// Persist a profile with no visible libraries through the existing profile API.
	if err := s.store.UpdateUser(ctx, viewer.ID, store.UserUpdate{LibraryIDs: &viewer.LibraryIDs}); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/api/items?jump=S", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "jump-viewer"})
	w := httptest.NewRecorder()
	s.signedIn(s.listItems)(w, r)
	var denied struct {
		Total, Offset int
		Items         []itemDTO
	}
	if err := json.Unmarshal(w.Body.Bytes(), &denied); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || denied.Total != 0 || denied.Offset != 0 || len(denied.Items) != 0 {
		t.Fatalf("jump leaked restricted library: %d %s", w.Code, w.Body.String())
	}
}
