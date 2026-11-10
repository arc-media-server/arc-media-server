package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/settings"
	"ams/internal/store"
)

func securityServer(t testing.TB) (*Server, *store.User) {
	t.Helper()
	return securityServerAt(t, filepath.Join(t.TempDir(), "test.db"))
}

func securityServerAt(t testing.TB, path string) (*Server, *store.User) {
	t.Helper()
	st, err := store.Open(context.Background(), path, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	u, err := st.CreateUser(context.Background(), "Viewer", false, "violet", "", "")
	if err != nil {
		t.Fatal(err)
	}
	return &Server{store: st, playing: newNowPlaying(), limiter: auth.NewLimiter(),
		settings: func() settings.Settings { return settings.Settings{LocalBypass: true} },
		log:      slog.New(slog.NewTextHandler(io.Discard, nil))}, u
}

func TestProfileBypassRejectsForwardedRequests(t *testing.T) {
	s, u := securityServer(t)
	for _, forwarded := range []bool{false, true} {
		r := httptest.NewRequest("POST", "http://arc/api/auth/profile", strings.NewReader(`{"id":"`+u.ID+`"}`))
		r.RemoteAddr = "127.0.0.1:45000"
		want := http.StatusOK
		if forwarded {
			r.Header.Set("X-Forwarded-For", "203.0.113.5")
			want = http.StatusUnauthorized
		}
		w := httptest.NewRecorder()
		s.chooseProfile(w, r)
		if w.Code != want {
			t.Fatalf("forwarded=%v: got %d, want %d: %s", forwarded, w.Code, want, w.Body.String())
		}
		if forwarded && len(w.Result().Cookies()) != 0 {
			t.Fatal("forwarded visitor received a session")
		}
	}
}

func TestSessionCookieTransport(t *testing.T) {
	s, u := securityServer(t)
	for _, tc := range []struct {
		scheme, forwarded string
		secure            bool
	}{
		{"http", "", false}, {"https", "", true}, {"http", "https", true}, {"https", "http", true},
	} {
		r := httptest.NewRequest("POST", tc.scheme+"://arc/api/auth/profile", nil)
		if tc.forwarded != "" {
			r.Header.Set("X-Forwarded-Proto", tc.forwarded)
		}
		w := httptest.NewRecorder()
		if _, err := s.startSession(w, r, u.ID, "password"); err != nil {
			t.Fatal(err)
		}
		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatal("expected session cookie")
		}
		c := cookies[0]
		if c.Secure != tc.secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
			t.Errorf("%+v: unexpected cookie policy: %+v", tc, c)
		}
		logout := httptest.NewRecorder()
		s.logout(logout, r)
		if cs := logout.Result().Cookies(); len(cs) != 1 || cs[0].Secure != c.Secure || cs[0].MaxAge != -1 {
			t.Errorf("%+v: invalid logout cookie", tc)
		}
	}
}

func TestCastInheritsSessionRevocation(t *testing.T) {
	s, u := securityServer(t)
	ctx := context.Background()
	dir := t.TempDir()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Test"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(path, []byte("clip"), 0600); err != nil {
		t.Fatal(err)
	}
	fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: path, Size: 4, ModTime: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"logout", "password", "profile", "home-only"} {
		t.Run(mode, func(t *testing.T) {
			_, hash := auth.NewToken()
			via := "password"
			if mode == "home-only" {
				via = "local"
			}
			if err := s.store.CreateSession(ctx, hash, u.ID, via, "review"); err != nil {
				t.Fatal(err)
			}
			token := s.cast.add(&castGrant{userID: u.ID, session: hash, itemID: id})
			fetch := func(peer string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("GET", "http://arc/api/cast/"+token+"/api/items/"+id+"/stream", nil)
				r.RemoteAddr = peer
				r.SetPathValue("token", token)
				r.SetPathValue("rest", "api/items/"+id+"/stream")
				w := httptest.NewRecorder()
				s.castStream(w, r)
				return w
			}
			if w := fetch("192.168.1.2:45000"); w.Code != 200 || w.Body.String() != "clip" {
				t.Fatalf("valid cast: %d %s", w.Code, w.Body.String())
			}
			var err error
			peer := "192.168.1.2:45000"
			switch mode {
			case "logout":
				err = s.store.DeleteSession(ctx, hash)
			case "password":
				err = s.store.DeletePasswordSessions(ctx, "")
			case "profile":
				err = s.store.SetSessionUser(ctx, hash, "")
			case "home-only":
				peer = "203.0.113.5:45000"
			}
			if err != nil {
				t.Fatal(err)
			}
			if w := fetch(peer); w.Code != http.StatusUnauthorized {
				t.Fatalf("invalidated cast still accepted: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestAuthStatusEnvelopeKeepsNullAndEmptyCollections(t *testing.T) {
	s, _ := securityServer(t)
	r := httptest.NewRequest("GET", "http://arc/api/auth/status", nil)
	w := httptest.NewRecorder()
	s.authStatus(w, r)
	var got map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 8 || string(got["profile"]) != "null" || string(got["profiles"]) != "[]" || string(got["signed_in"]) != "false" {
		t.Fatal("auth JSON changed", w.Body.String())
	}
}
