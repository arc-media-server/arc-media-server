package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
)

// A player that can't send the sign-in (Roku, LG/Samsung) plays a trailer through a cast link:
// POST /api/cast takes an extra's stream link, the link serves only that extra, and it stops
// working when the session is signed out.
func TestCastExtra(t *testing.T) {
	s, u := securityServer(t)
	ctx := context.Background()
	dir := t.TempDir()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Film"}, "film")
	if err != nil {
		t.Fatal(err)
	}
	add := func(name string, body []byte) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
		fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: path, Size: int64(len(body)), ModTime: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.store.SaveExtra(ctx, id, fid, "trailer", name); err != nil {
			t.Fatal(err)
		}
		extras, err := s.store.ItemExtras(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range extras {
			if e.Title == name {
				return e.ID
			}
		}
		t.Fatalf("extra %s not saved", name)
		return ""
	}
	trailer := add("trailer.mp4", []byte("trailer bytes"))
	other := add("other.mp4", []byte("other bytes"))

	_, hash := auth.NewToken()
	if err := s.store.CreateSession(ctx, hash, u.ID, "password", "roku"); err != nil {
		t.Fatal(err)
	}
	link := func(url string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"url": url})
		r := httptest.NewRequest("POST", "http://arc:8099/api/cast", bytes.NewReader(body))
		r = r.WithContext(context.WithValue(context.WithValue(r.Context(), userKey, u), sessionKey, hash))
		w := httptest.NewRecorder()
		s.castLink(w, r)
		return w
	}
	w := link("/api/extras/" + trailer + "/stream")
	if w.Code != 200 {
		t.Fatalf("cast link for an extra: %d %s", w.Code, w.Body.String())
	}
	var got struct {
		URL string `json:"url"`
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	prefix := "http://arc:8099/api/cast/"
	if !strings.HasPrefix(got.URL, prefix) || !strings.HasSuffix(got.URL, "/api/extras/"+trailer+"/stream") {
		t.Fatalf("link %q", got.URL)
	}
	token := strings.SplitN(strings.TrimPrefix(got.URL, prefix), "/", 2)[0]
	if w := link("/api/extras/ffffffffffffffffffffffff/stream"); w.Code != http.StatusNotFound {
		t.Fatalf("unknown extra: %d", w.Code)
	}

	fetch := func(extra string) *httptest.ResponseRecorder {
		rest := "api/extras/" + extra + "/stream"
		r := httptest.NewRequest("GET", "http://arc/api/cast/"+token+"/"+rest, nil)
		r.RemoteAddr = "192.168.1.2:45000"
		r.SetPathValue("token", token)
		r.SetPathValue("rest", rest)
		w := httptest.NewRecorder()
		s.castStream(w, r)
		return w
	}
	if w := fetch(trailer); w.Code != 200 || w.Body.String() != "trailer bytes" {
		t.Fatalf("cast extra stream: %d %q", w.Code, w.Body.String())
	}
	if w := fetch(other); w.Code != http.StatusNotFound {
		t.Fatalf("another extra through this link: %d", w.Code)
	}
	if err := s.store.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if w := fetch(trailer); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session still served the extra: %d", w.Code)
	}
}
