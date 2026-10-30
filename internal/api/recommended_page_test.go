package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestRecommendedContinuationAndHiddenRows(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	lib, _ := s.store.SyncLibrary(ctx, "Paging", "movies", "tmdb_aired", nil)
	hidden, _ := s.store.SyncLibrary(ctx, "Denied", "movies", "tmdb_aired", nil)
	for i := range 53 {
		lid := lib
		if i == 52 {
			lid = hidden
		}
		title := fmt.Sprintf("Film %03d", i)
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lid, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		file, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lid, Path: "/fixture/" + title + ".mkv"}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err = s.store.SetFileItems(ctx, file, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
	}
	allowed := []int64{lib}
	s.store.UpdateUser(ctx, u.ID, store.UserUpdate{LibraryIDs: &allowed})
	s.store.CreateSession(ctx, auth.HashToken("rec-pages"), u.ID, "password", "")
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	get := func(path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer rec-pages")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := get(fmt.Sprintf("/api/v1/libraries/%d/recommended", lib))
	var rows recommendedResponse
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil || w.Code != 200 {
		t.Fatal(w.Body.String(), err)
	}
	var row recommendedRow
	for _, r := range rows.Rows {
		if r.Type == "recently_added" {
			row = r
		}
	}
	if row.ID == "" || !row.HasMore || len(row.Items) != 20 {
		t.Fatalf("missing continuation: %+v", row)
	}
	seen := map[string]bool{}
	for _, i := range row.Items {
		seen[i.ID] = true
	}
	for _, off := range []int{20, 40} {
		w = get(fmt.Sprintf("%s?limit=20&offset=%d", row.ItemsURL, off))
		var page recommendedItemsPage
		json.Unmarshal(w.Body.Bytes(), &page)
		if w.Code != 200 || page.Total != 52 || page.Offset != off || page.Bounded {
			t.Fatalf("page %d %s", w.Code, w.Body.String())
		}
		for _, it := range page.Items {
			if seen[it.ID] {
				t.Fatal("duplicate across pages")
			}
			seen[it.ID] = true
		}
	}
	if len(seen) != 52 {
		t.Fatal("not all items paged", len(seen))
	}
	if w = get(row.ItemsURL + "?limit=101"); w.Code != 400 {
		t.Fatal("invalid limit accepted")
	}
	s.store.SetRecommendedLayout(ctx, lib, `[{"type":"recently_added","hidden":true}]`)
	if w = get(row.ItemsURL); w.Code != 404 {
		t.Fatal("hidden row accessible", w.Code)
	}
	if w = get(fmt.Sprintf("/api/v1/libraries/%d/recommended", hidden)); w.Code != 403 && w.Code != 404 {
		t.Fatal("denied library accessible", w.Code)
	}
}
