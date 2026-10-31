package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/settings"
)

func appCORSHandler(t *testing.T) (http.Handler, string) {
	t.Helper()
	s, u := securityServer(t)
	if err := s.store.CreateSession(t.Context(), auth.HashToken("tv-token"), u.ID, "password", "Living Room TV"); err != nil {
		t.Fatal(err)
	}
	if err := s.store.CreateSession(t.Context(), auth.HashToken("browser-cookie"), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	h := New(Deps{Store: s.store, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Version: "test",
		Settings: func() settings.Settings { return settings.Settings{LocalBypass: true, ServerName: "Den"} }})
	return h, u.ID
}

func TestAppCORSPreflightAndBearer(t *testing.T) {
	h, _ := appCORSHandler(t)

	r := httptest.NewRequest("OPTIONS", "http://192.168.1.10:8099/api/v1/libraries", nil)
	r.Header.Set("Origin", "file://")
	r.Header.Set("Access-Control-Request-Method", "GET")
	r.Header.Set("Access-Control-Request-Headers", "authorization")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent || w.Header().Get("Access-Control-Allow-Origin") != "*" ||
		!strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("preflight %d %v", w.Code, w.Header())
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("credentials must never be allowed cross-origin")
	}

	r = httptest.NewRequest("GET", "http://192.168.1.10:8099/api/v1/libraries", nil)
	r.RemoteAddr = "192.168.1.20:5000"
	r.Header.Set("Origin", "null")
	r.Header.Set("Authorization", "Bearer tv-token")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("bearer call %d %s", w.Code, w.Body.String())
	}
}

func TestAppCORSIgnoresCookiesAndHomeBypass(t *testing.T) {
	h, userID := appCORSHandler(t)

	// A page elsewhere, opened by someone on the home network, holding Arc's browser cookie.
	r := httptest.NewRequest("GET", "http://192.168.1.10:8099/api/v1/libraries", nil)
	r.RemoteAddr = "192.168.1.20:5000"
	r.Header.Set("Origin", "https://evil.example")
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "browser-cookie"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("cookie honoured cross-origin: %d", w.Code)
	}

	r = httptest.NewRequest("GET", "http://192.168.1.10:8099/api/v1/auth/status", nil)
	r.RemoteAddr = "192.168.1.20:5000"
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var status struct {
		Bypass   bool  `json:"bypass"`
		Profiles []any `json:"profiles"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil || status.Bypass || len(status.Profiles) != 0 {
		t.Fatalf("home bypass offered cross-origin: %s", w.Body.String())
	}

	r = httptest.NewRequest("POST", "http://192.168.1.10:8099/api/v1/auth/profile", strings.NewReader(`{"id":"`+userID+`"}`))
	r.RemoteAddr = "192.168.1.20:5000"
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code == 200 {
		t.Fatalf("profile picked without a password cross-origin: %s", w.Body.String())
	}

	r = httptest.NewRequest("POST", "http://192.168.1.10:8099/api/v1/auth/setup", strings.NewReader(`{}`))
	r.Header.Set("Origin", "https://evil.example")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("setup allowed cross-origin: %d", w.Code)
	}
}

func TestAppCORSLeavesSameOriginAndWebAppAlone(t *testing.T) {
	h, _ := appCORSHandler(t)
	for _, tc := range []struct{ path, origin, host string }{
		{"/api/v1/libraries", "http://arc.home:8099", "arc.home:8099"}, // Arc's own API docs page
		{"/api/libraries", "https://evil.example", "arc.home:8099"},    // the web app's API: no CORS
	} {
		r := httptest.NewRequest("GET", "http://"+tc.host+tc.path, nil)
		r.Header.Set("Origin", tc.origin)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "browser-cookie"})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatalf("%s from %s got CORS headers", tc.path, tc.origin)
		}
		if tc.origin == "http://arc.home:8099" && w.Code != 200 {
			t.Fatalf("same-origin cookie call failed: %d", w.Code)
		}
	}
}

func TestDiscoverAnswersOnlyTheHomeNetwork(t *testing.T) {
	h, _ := appCORSHandler(t)
	for _, tc := range []struct {
		remote string
		want   int
	}{{"192.168.1.20:5000", 200}, {"203.0.113.9:5000", 404}} {
		r := httptest.NewRequest("GET", "http://192.168.1.10:8099/api/v1/discover", nil)
		r.RemoteAddr = tc.remote
		r.Header.Set("Origin", "file://")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s: %d %s", tc.remote, w.Code, w.Body.String())
		}
		if tc.want != 200 {
			continue
		}
		var got discoveredServer
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Product != "arc" || got.Name != "Den" ||
			len(got.ID) != 32 || got.APIVersion != APIVersion || w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("discover %+v %s", got, w.Body.String())
		}
	}
}
