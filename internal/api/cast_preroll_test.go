package api

import (
	"ams/internal/auth"
	"ams/internal/settings"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Native players need a clip-scoped link; ordinary pre-roll files still require sign-in.
func TestCastPreroll(t *testing.T) {
	s, u := securityServer(t)
	ctx := context.Background()
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first.mp4"), filepath.Join(dir, "second.mp4")
	for path, body := range map[string]string{first: "first clip bytes", second: "second clip bytes"} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	paths := first + ";" + second
	s.settings = func() settings.Settings { return settings.Settings{PrerollPaths: paths} }
	sessionToken, hash := auth.NewToken()
	if err := s.store.CreateSession(ctx, hash, u.ID, "password", "fixture"); err != nil {
		t.Fatal(err)
	}
	handler := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	unauth := httptest.NewRecorder()
	handler.ServeHTTP(unauth, httptest.NewRequest("GET", "http://arc:8099/api/v1/preroll/0", nil))
	if unauth.Code != 401 {
		t.Fatalf("unauthed pre-roll: %d", unauth.Code)
	}
	link := func(path string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"url": path})
		r := httptest.NewRequest("POST", "http://arc:8099/api/cast", bytes.NewReader(body))
		r = r.WithContext(context.WithValue(context.WithValue(r.Context(), userKey, u), sessionKey, hash))
		w := httptest.NewRecorder()
		s.castLink(w, r)
		return w
	}
	w := link("/api/preroll/0?v=fixture")
	if w.Code != 200 {
		t.Fatalf("pre-roll cast: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		URL string `json:"url"`
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	if strings.Contains(got.URL, hash) || strings.Contains(got.URL, sessionToken) {
		t.Fatal("cast link contains session credential")
	}
	token := strings.SplitN(strings.TrimPrefix(got.URL, "http://arc:8099/api/cast/"), "/", 2)[0]
	fetch := func(rest string, byteRange bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "http://arc/api/cast/"+token+"/"+rest, nil)
		r.RemoteAddr = "192.168.1.2:45000"
		r.SetPathValue("token", token)
		r.SetPathValue("rest", rest)
		if byteRange {
			r.Header.Set("Range", "bytes=0-4")
		}
		w := httptest.NewRecorder()
		s.castStream(w, r)
		return w
	}
	if w := fetch("api/preroll/0", false); w.Code != 200 || w.Body.String() != "first clip bytes" {
		t.Fatalf("clip: %d %q", w.Code, w.Body.String())
	}
	if w := fetch("api/preroll/0", true); w.Code != 206 || w.Body.String() != "first" {
		t.Fatalf("range: %d %q", w.Code, w.Body.String())
	}
	for _, rest := range []string{"api/preroll/1", "api/items/ffffffff/stream", "api/extras/ffffffff/stream", "api/settings", "api/transcode/other/index.m3u8"} {
		if w := fetch(rest, false); w.Code != 404 {
			t.Errorf("escaped scope via %s: %d", rest, w.Code)
		}
	}
	for _, path := range []string{"/api/preroll/99", "/api/preroll/-1", "/api/preroll/0/other"} {
		if w := link(path); w.Code == 200 {
			t.Errorf("invalid link accepted: %s", path)
		}
	}
	paths = second + ";" + first
	if w := fetch("api/preroll/0", false); w.Code != 404 {
		t.Fatalf("reordered clip reused grant: %d", w.Code)
	}
	paths = first + ";" + second
	if err := s.store.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if w := fetch("api/preroll/0", false); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session: %d", w.Code)
	}
}
