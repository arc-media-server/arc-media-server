package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestCreateCollectionValidatesMembersBeforeCreatingAndKeepsOptions(t *testing.T) {
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
	item, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	request := func(body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "http://arc/api/v1/collections", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer owner")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	failed := request(fmt.Sprintf(`{"library_id":%d,"kind":"manual","title":"Must not exist","item_ids":[%q,"missing"]}`, lib, item))
	if failed.Code != 400 {
		t.Fatal(failed.Code, failed.Body.String())
	}
	collections, err := s.store.Collections(t.Context(), lib, store.Access{})
	if err != nil || len(collections) != 0 {
		t.Errorf("failed create left %d collections (%v)", len(collections), err)
	}
	created := request(fmt.Sprintf(`{"library_id":%d,"kind":"manual","title":"Valid","sort_title":"custom order","overview":"My description","grid_mode":"both","locked_fields":["overview"],"item_ids":[%q,%q]}`, lib, item, item))
	if created.Code != 201 {
		t.Fatal(created.Code, created.Body.String())
	}
	var response collectionDTO
	if err = json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	saved, err := s.store.GetCollection(t.Context(), response.ID)
	if err != nil || saved == nil {
		t.Fatal(err)
	}
	if saved.SortTitle != "custom order" || saved.GridMode != "both" || !slices.Contains(saved.LockedFields, "overview") || saved.Overview != "My description" {
		t.Errorf("validated create options were dropped: %+v", saved)
	}
	members, err := s.store.CollectionItems(t.Context(), saved, store.Access{})
	if err != nil {
		t.Fatal(err)
	}
	// CollectionItems hides files not marked present; inspect the collection's membership
	// through ItemCollections as well so this fixture need not read media from disk.
	refs, err := s.store.ItemCollections(t.Context(), &store.Item{ID: item, LibraryID: lib, Kind: "movie"}, store.Access{})
	if err != nil || len(refs) != 1 {
		t.Fatal("initial member not persisted", refs, members, err)
	}
}
