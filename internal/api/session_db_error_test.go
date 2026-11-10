package api

import (
	"ams/internal/auth"
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSessionDatabaseFailureDoesNotRevokeCredentials(t *testing.T) {
	for _, mode := range []string{"cookie", "bearer"} {
		t.Run(mode, func(t *testing.T) {
			s, u := securityServer(t)
			if err := s.store.CreateSession(t.Context(), auth.HashToken("fixture-token"), u.ID, "password", ""); err != nil {
				t.Fatal(err)
			}
			s.store.Close()
			reached := false
			h := s.signedIn(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(204) })
			r := httptest.NewRequest("GET", "http://arc/api/libraries", nil)
			if mode == "cookie" {
				r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "fixture-token"})
			} else {
				r.Header.Set("Authorization", "Bearer fixture-token")
			}
			w := httptest.NewRecorder()
			h(w, r)
			if w.Code != 503 || w.Header().Get("Retry-After") == "" || reached {
				t.Fatalf("transient lookup failure status=%d retry=%q handler=%v", w.Code, w.Header().Get("Retry-After"), reached)
			}
			if len(w.Result().Cookies()) != 0 {
				t.Fatal("transient failure cleared session cookie")
			}
		})
	}
}

func TestSessionLookupFailureIsLoggedOnce(t *testing.T) {
	s, _ := securityServer(t)
	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, nil))
	s.store.Close()
	h := s.logRequests(s.signedIn(func(http.ResponseWriter, *http.Request) { t.Fatal("handler reached") }))
	r := httptest.NewRequest("GET", "http://arc/api/libraries", nil)
	r.Header.Set("Authorization", "Bearer fixture-private-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 503 || strings.Count(logs.String(), "level=ERROR") != 1 || strings.Contains(logs.String(), "level=WARN") || strings.Contains(logs.String(), "fixture-private-token") {
		t.Fatalf("incorrect failure diagnostics: status=%d logs=%s", w.Code, logs.String())
	}
}
func TestMissingOrRevokedSessionStillReturnsUnauthorized(t *testing.T) {
	s, _ := securityServer(t)
	for _, token := range []string{"", "revoked-token"} {
		r := httptest.NewRequest("GET", "http://arc/api/libraries", nil)
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.signedIn(func(http.ResponseWriter, *http.Request) { t.Fatal("handler reached") })(w, r)
		if w.Code != 401 {
			t.Fatalf("missing/revoked session status=%d", w.Code)
		}
	}
}
func TestEventPrincipalPropagatesSessionDatabaseFailure(t *testing.T) {
	s, u := securityServer(t)
	s.store.Close()
	r := httptest.NewRequest("GET", "http://arc/api/events", nil)
	r.Header.Set("Authorization", "Bearer fixture-token")
	if _, ok, err := s.eventPrincipal(r, u.ID); err == nil || ok {
		t.Fatal("DB failure classified as an ordinary revoked principal")
	}
}
