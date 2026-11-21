package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A collection without a poster of its own answers its image link with its first title's, as its
// listing says it does: a title's list of collections links straight to that route.
func TestCollectionImageFallsBackToFirstTitle(t *testing.T) {
	s, _ := securityServer(t)
	owner, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(t.Context(), auth.HashToken("owner"), owner.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	poster := filepath.Join(t.TempDir(), "poster.jpg")
	if err = os.WriteFile(poster, []byte("first title's poster"), 0o644); err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "First"}, "first")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetLocalArt(t.Context(), item, poster, "", ""); err != nil {
		t.Fatal(err)
	}
	file, err := s.store.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: "/fixture/first.mkv", Size: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetFileItems(t.Context(), file, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	do := func(method, url, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://arc"+url, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer owner")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	created := do("POST", "/api/v1/collections", fmt.Sprintf(`{"library_id":%d,"kind":"manual","title":"No art","item_ids":[%q]}`, lib, item))
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body.String())
	}
	var c collectionDTO
	if err = json.Unmarshal(created.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	got := do("GET", "/api/v1/collections/"+c.ID+"/images/primary", "")
	if got.Code != 200 || got.Body.String() != "first title's poster" {
		t.Fatalf("collection image = %d %q, want the first title's poster", got.Code, got.Body.String())
	}
	if bad := do("GET", "/api/v1/collections/"+c.ID+"/images/nonsense", ""); bad.Code != 400 {
		t.Errorf("unknown type = %d, want 400", bad.Code)
	}
	empty := do("POST", "/api/v1/collections", fmt.Sprintf(`{"library_id":%d,"kind":"manual","title":"Empty"}`, lib))
	if empty.Code != 201 {
		t.Fatal(empty.Code, empty.Body.String())
	}
	var e collectionDTO
	_ = json.Unmarshal(empty.Body.Bytes(), &e)
	if none := do("GET", "/api/v1/collections/"+e.ID+"/images/primary", ""); none.Code != 404 {
		t.Errorf("empty collection image = %d, want 404", none.Code)
	}
}
