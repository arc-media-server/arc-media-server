package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
)

func TestNavigationAndCompletionPreferencesAreProfileScoped(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := context.Background()
	other, err := s.store.CreateUser(ctx, "Other viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for token, id := range map[string]string{"first": viewer.ID, "second": other.ID} {
		if err := s.store.CreateSession(ctx, auth.HashToken(token), id, "password", "test"); err != nil {
			t.Fatal(err)
		}
	}
	call := func(token, method, path, body string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		w := httptest.NewRecorder()
		s.signedIn(handler)(w, r)
		return w
	}
	w := call("first", "PATCH", "/api/me/prefs", `{"library_order":[8,2,999],"watched_percent":100}`, s.updatePrefs)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"library_order":[8,8]}`, `{"library_order":[-1]}`, `{"watched_percent":49}`, `{"watched_percent":101}`, `{"watched_percent":90.5}`} {
		if w := call("first", "PATCH", "/api/me/prefs", body, s.updatePrefs); w.Code != 400 {
			t.Fatalf("accepted %s: %d", body, w.Code)
		}
	}
	w = call("first", "GET", "/api/me/prefs", "", s.getPrefs)
	var saved prefs
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.WatchedPercent != 100 || len(saved.LibraryOrder) != 3 || saved.LibraryOrder[0] != 8 {
		t.Fatalf("invalid update changed prefs: %+v", saved)
	}
	w = call("second", "GET", "/api/me/prefs", "", s.getPrefs)
	var untouched prefs
	if err := json.Unmarshal(w.Body.Bytes(), &untouched); err != nil {
		t.Fatal(err)
	}
	if untouched.WatchedPercent != 0 || len(untouched.LibraryOrder) != 0 {
		t.Fatalf("cross-profile preferences: %+v", untouched)
	}
	w = call("first", "PATCH", "/api/me/prefs", `{"library_order":[],"watched_percent":0}`, s.updatePrefs)
	if w.Code != 200 {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
}

func TestProgressUsesProfileThresholdForWatchStateAndHistory(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := context.Background()
	lib, err := s.store.SyncLibrary(ctx, "Test", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Long credits"}, "credits")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetUserPrefs(ctx, viewer.ID, `{"watched_percent":100}`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		position int64
		played   bool
		count    int
	}{{360000, false, 0}, {399999, false, 0}, {400000, true, 1}, {400000, true, 1}} {
		r := httptest.NewRequest("PUT", "/api/items/"+id+"/progress", strings.NewReader(`{"position_ms":`+jsonNumber(tc.position)+`,"duration_ms":400000}`))
		r.SetPathValue("id", id)
		r = r.WithContext(context.WithValue(r.Context(), userKey, viewer))
		w := httptest.NewRecorder()
		s.reportProgress(w, r)
		var d store.UserData
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || d.Played != tc.played || d.PlayCount != tc.count {
			t.Fatalf("position=%d: status=%d data=%+v", tc.position, w.Code, d)
		}
	}
	// History and played notifications use the same completion boundary, without
	// counting an already-completed report as a second completion.
	n := newNowPlaying()
	r := httptest.NewRequest("PUT", "/", nil)
	p := n.touch(r, "history", "")
	p.historyID = 1
	for i, pos := range []int64{360000, 400000, 400000} {
		_, u := n.progressAt(r, "history", pos, 400000, "playing", 100)
		if u == nil || u.completed != (i > 0) || u.justCompleted != (i == 1) {
			t.Fatalf("history threshold report %d: %+v", i, u)
		}
	}
}

func jsonNumber(n int64) string { b, _ := json.Marshal(n); return string(b) }
