package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
)

func TestWatchSyncProfileStateAndHistory(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	other, err := s.store.CreateUser(ctx, "Other viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	hiddenLib, err := s.store.SyncLibrary(ctx, "Other movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	item := func(lib int64, kind string) string {
		t.Helper()
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Title: "Fixture", Kind: kind}, kind)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	movie, hidden, series := item(lib, "movie"), item(hiddenLib, "movie"), item(lib, "series")
	allowed := []int64{lib}
	if err := s.store.UpdateUser(ctx, u.ID, store.UserUpdate{LibraryIDs: &allowed}); err != nil {
		t.Fatal(err)
	}
	for token, user := range map[string]string{"sync": u.ID, "other": other.ID} {
		if err := s.store.CreateSession(ctx, auth.HashToken(token), user, "password", ""); err != nil {
			t.Fatal(err)
		}
	}
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "http://arc/api/v1"+path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	check := func(method, path, body, token string, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := request(method, path, body, token)
		if w.Code != status {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	resume := "/items/" + movie + "/resume"
	history := "/items/" + movie + "/watch-history"
	check("GET", "/watch-history", "", "", 401)
	check("PUT", "/items/"+hidden+"/resume", `{"position_ms":1}`, "sync", 404)
	check("POST", "/items/"+hidden+"/watch-history", `{"watched_at":1700000000}`, "sync", 404)
	check("PUT", "/items/"+series+"/resume", `{"position_ms":1}`, "sync", 400)
	check("PUT", resume, `{}`, "sync", 400)
	check("PUT", resume, `{"position_ms":-1}`, "sync", 400)
	check("POST", history, `{"watched_at":1700000000000}`, "sync", 400)
	check("GET", "/watch-history?before=0", "", "sync", 400)
	check("PUT", "/items/"+movie+"/rating", `{"rating":8}`, "sync", 204)
	check("PUT", "/items/"+movie+"/watchlist", `{"on":true}`, "sync", 204)
	check("PUT", resume, `{"position_ms":12000}`, "sync", 204)
	check("PUT", resume, `{"position_ms":12000}`, "sync", 204)
	state := func(position int64) {
		t.Helper()
		data, err := s.store.UserDataFor(ctx, u.ID, []string{movie})
		want := store.UserData{PositionMS: position, Rating: 8, Watchlisted: true}
		if err != nil || data[movie] != want {
			t.Fatalf("watch state = %+v, want %+v: %v", data[movie], want, err)
		}
	}
	state(12000)
	plays, err := s.store.Plays(ctx, store.PlayFilter{UserID: u.ID})
	if err != nil || len(plays) != 0 {
		t.Fatalf("resume generated a viewing: %v %v", plays, err)
	}
	// Parallel retries record a single viewing and return the same ID.
	const stamp = 1700000000
	older := `{"watched_at":1700000000}`
	ids := make(chan int64, 4)
	var group sync.WaitGroup
	for range cap(ids) {
		group.Go(func() {
			w := request("POST", history, older, "sync")
			var viewing store.WatchViewing
			if err := json.Unmarshal(w.Body.Bytes(), &viewing); err != nil || w.Code != 200 || viewing.WatchedAt != stamp || viewing.ItemID != movie || viewing.ID == 0 {
				t.Errorf("unexpected viewing: %d %s", w.Code, w.Body.String())
			}
			ids <- viewing.ID
		})
	}
	group.Wait()
	close(ids)
	firstID := <-ids
	for id := range ids {
		if firstID != id {
			t.Fatalf("retry returned a different viewing: %d vs %d", firstID, id)
		}
	}
	check("POST", history, `{"watched_at":1600000000}`, "sync", 200)
	check("POST", history, older, "other", 200)
	if _, err := s.store.AddWatchViewing(ctx, u.ID, hidden, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.AddPlay(ctx, store.Play{UserID: u.ID, ItemID: movie, Kind: "movie", LibraryID: lib, Title: "Fixture", StartedAt: stamp + 1}); err != nil {
		t.Fatal(err)
	}
	page := func(path, token string) watchHistoryPage {
		t.Helper()
		var p watchHistoryPage
		if err := json.Unmarshal(check("GET", path, "", token, 200).Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := page("/watch-history?limit=1", "sync")
	if len(p.Viewings) != 1 || p.Viewings[0].WatchedAt != 1600000000 || p.NextBefore == 0 {
		t.Fatalf("first history page = %+v", p)
	}
	p = page(fmt.Sprintf("/watch-history?limit=1&before=%d", p.NextBefore), "sync")
	if len(p.Viewings) != 1 || p.Viewings[0].ID != firstID || p.NextBefore != 0 {
		t.Fatalf("second history page = %+v", p)
	}
	remove := fmt.Sprintf("%s/%d?watched_at=", history, firstID)
	check("DELETE", remove+"1700000000", "", "other", 204)
	check("DELETE", remove+"1600000000", "", "sync", 204)
	if p := page("/watch-history", "sync"); len(p.Viewings) != 2 {
		t.Fatalf("mismatched deletion removed history: %+v", p)
	}
	check("DELETE", remove+"1700000000", "", "sync", 204)
	check("DELETE", remove+"1700000000", "", "sync", 204)
	if p := page("/watch-history", "sync"); len(p.Viewings) != 1 || p.Viewings[0].WatchedAt != 1600000000 {
		t.Fatalf("wrong viewing deleted: %+v", p)
	}
	if p := page("/watch-history", "other"); len(p.Viewings) != 1 || p.Viewings[0].WatchedAt != stamp {
		t.Fatalf("other profile affected: %+v", p)
	}
	state(12000)
	check("PUT", resume, `{"position_ms":0}`, "sync", 204)
	state(0)
}
