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
	w := call("first", "PATCH", "/api/me/prefs", `{"library_order":[8,2,999],"watched_percent":100,"watchlist_position":2}`, s.updatePrefs)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"library_order":[8,8]}`, `{"library_order":[-1]}`, `{"watched_percent":49}`, `{"watched_percent":101}`, `{"watched_percent":90.5}`, `{"watchlist_position":-1}`, `{"watchlist_position":501}`} {
		if w := call("first", "PATCH", "/api/me/prefs", body, s.updatePrefs); w.Code != 400 {
			t.Fatalf("accepted %s: %d", body, w.Code)
		}
	}
	w = call("first", "GET", "/api/me/prefs", "", s.getPrefs)
	var saved prefs
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.WatchedPercent != 100 || len(saved.LibraryOrder) != 3 || saved.LibraryOrder[0] != 8 || saved.WatchlistPosition != 2 {
		t.Fatalf("invalid update changed prefs: %+v", saved)
	}
	w = call("second", "GET", "/api/me/prefs", "", s.getPrefs)
	var untouched prefs
	if err := json.Unmarshal(w.Body.Bytes(), &untouched); err != nil {
		t.Fatal(err)
	}
	if untouched.WatchedPercent != 0 || len(untouched.LibraryOrder) != 0 || untouched.WatchlistPosition != 0 {
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

func TestHomeHeroPreferenceIsPersistedAndProfileScoped(t *testing.T) {
	s, first := securityServer(t)
	ctx := t.Context()
	second, err := s.store.CreateUser(ctx, "Other viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for token, id := range map[string]string{"hero-first": first.ID, "hero-second": second.ID} {
		if err := s.store.CreateSession(ctx, auth.HashToken(token), id, "password", "fixture"); err != nil {
			t.Fatal(err)
		}
	}
	call := func(token, method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/me/prefs", strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		w := httptest.NewRecorder()
		handler := s.getPrefs
		if method == "PATCH" {
			handler = s.updatePrefs
		}
		s.signedIn(handler)(w, r)
		return w
	}
	read := func(token string) prefs {
		t.Helper()
		w := call(token, "GET", "")
		if w.Code != 200 {
			t.Fatalf("get: %d %s", w.Code, w.Body.String())
		}
		var p prefs
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if read("hero-first").ShowHomeHero != nil {
		t.Fatal("existing default rewritten")
	}
	for _, body := range []string{`{"show_home_hero":false}`, `{"theme":"dark"}`, `{"show_home_hero":null}`} {
		w := call("hero-first", "PATCH", body)
		if w.Code != 200 {
			t.Fatalf("patch %s: %d %s", body, w.Code, w.Body.String())
		}
	}
	if p := read("hero-first"); p.ShowHomeHero == nil || *p.ShowHomeHero || p.Theme != "dark" {
		t.Fatalf("hidden preference lost: %+v", p)
	}
	if read("hero-second").ShowHomeHero != nil {
		t.Fatal("another profile inherited the hidden hero")
	}
	for _, body := range []string{`{"show_home_hero":"false"}`, `{"show_home_hero":0}`} {
		if w := call("hero-first", "PATCH", body); w.Code != 400 {
			t.Fatalf("accepted invalid boolean: %s", body)
		}
	}
	if w := call("hero-first", "PATCH", `{"show_home_hero":true}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if p := read("hero-first"); p.ShowHomeHero == nil || !*p.ShowHomeHero {
		t.Fatal("could not restore hero")
	}
}

func TestPosterSizePreferencesAreValidatedAndProfileScoped(t *testing.T) {
	s, user := securityServer(t)
	other, err := s.store.CreateUser(t.Context(), "Other poster viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	call := func(u *store.User, method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/me/prefs", strings.NewReader(body))
		r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		w := httptest.NewRecorder()
		if method == "PATCH" {
			s.updatePrefs(w, r)
		} else {
			s.getPrefs(w, r)
		}
		return w
	}
	read := func(u *store.User) prefs {
		t.Helper()
		w := call(u, "GET", "")
		if w.Code != 200 {
			t.Fatalf("get: %d %s", w.Code, w.Body.String())
		}
		var p prefs
		if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := read(user); p.HomePosterSize != 0 || p.LibraryPosterSize != 0 {
		t.Fatal("new profile lost default size")
	}
	for _, body := range []string{`{"home_poster_size":125,"library_poster_size":90,"theme":"dark"}`, `{"home_poster_size":150}`, `{"home_poster_size":75}`, `{"home_poster_size":115}`} {
		if w := call(user, "PATCH", body); w.Code != 200 {
			t.Fatalf("save: %d %s", w.Code, w.Body.String())
		}
	}
	for _, body := range []string{`{"home_poster_size":74}`, `{"library_poster_size":151}`, `{"home_poster_size":-1}`, `{"home_poster_size":100.5}`, `{"home_poster_size":"100"}`, `{"home_poster_size":125,"library_poster_size":999}`} {
		if w := call(user, "PATCH", body); w.Code != 400 {
			t.Fatalf("accepted %s: %d", body, w.Code)
		}
	}
	if p := read(user); p.HomePosterSize != 115 || p.LibraryPosterSize != 90 || p.Theme != "dark" {
		t.Fatalf("partial/invalid patch lost preferences: %+v", p)
	}
	if p := read(other); p.HomePosterSize != 0 || p.LibraryPosterSize != 0 {
		t.Fatalf("size leaked to another profile: %+v", p)
	}
	raw, err := s.store.UserPrefs(t.Context(), user.ID)
	if err != nil || !strings.Contains(raw, `"home_poster_size":115`) {
		t.Fatalf("size not persisted: %s %v", raw, err)
	}
	if w := call(user, "PATCH", `{"home_poster_size":0,"library_poster_size":0}`); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if p := read(user); p.HomePosterSize != 0 || p.LibraryPosterSize != 0 || p.Theme != "dark" {
		t.Fatalf("reset changed unrelated prefs: %+v", p)
	}
}
