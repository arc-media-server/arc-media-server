package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestLookupAndBatchRespectAccessOrderAndBounds(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	lib, err := s.store.SyncLibrary(ctx, "Allowed", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	hiddenLib, err := s.store.SyncLibrary(ctx, "Hidden", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i, l := range []int64{lib, lib, hiddenLib} {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: l, Kind: "movie", Title: "Fixture", TMDBID: 42, IMDBID: "tt0042"}, string(rune('a'+i)))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	allowed := []int64{lib}
	if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{LibraryIDs: &allowed}); err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(ctx, auth.HashToken("lookup"), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://arc/api/v1"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer lookup")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	w := request("GET", "/items/lookup?provider=tmdb&provider_id=42&limit=1", "")
	var page itemPage
	if err = json.Unmarshal(w.Body.Bytes(), &page); err != nil || w.Code != 200 || page.Total != 2 || len(page.Items) != 1 {
		t.Fatalf("lookup %d %s %v", w.Code, w.Body.String(), err)
	}
	w = request("GET", "/items/lookup?provider=imdb&provider_id=TT0042", "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = request("POST", "/items/batch", `{"ids":["`+ids[1]+`","`+ids[2]+`","00000000000000000000000000000000","`+ids[0]+`","`+ids[1]+`"]}`)
	var batch itemBatch
	if err = json.Unmarshal(w.Body.Bytes(), &batch); err != nil || w.Code != 200 || len(batch.Items) != 3 {
		t.Fatalf("batch %d %s %v", w.Code, w.Body.String(), err)
	}
	if batch.Items[0].ID != ids[1] || batch.Items[1].ID != ids[0] || batch.Items[2].ID != ids[1] {
		t.Fatal("batch input order/duplicates changed")
	}
	for _, path := range []string{"/items/lookup?provider=foo&provider_id=1", "/items/lookup?provider=tmdb&provider_id=0", "/items/lookup?provider=tmdb&provider_id=42&offset=-1", "/items/lookup?provider=tmdb&provider_id=42&limit=101"} {
		if w = request("GET", path, ""); w.Code != 400 {
			t.Fatal("invalid lookup accepted", path)
		}
	}
	for _, body := range []string{`{"ids":[]}`, `{"ids":["invalid"]}`, `{"ids":["` + ids[0] + `"]} {}`} {
		if w = request("POST", "/items/batch", body); w.Code != 400 {
			t.Fatal("invalid batch accepted", body)
		}
	}
	unknown := request("GET", "/items/lookup?provider=tmdb&provider_id=999", "")
	denied := request("GET", "/items/lookup?provider=tmdb&provider_id=42&library_id="+strconv.FormatInt(hiddenLib, 10), "")
	if unknown.Code != 200 || denied.Code != 200 || unknown.Body.String() != denied.Body.String() {
		t.Fatal("hidden lookup distinguishable from unknown")
	}
}
