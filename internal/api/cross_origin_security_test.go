package api

import (
	"ams/internal/auth"
	"ams/internal/settings"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCrossOriginCookieMutationIsRejected(t *testing.T) {
	s, _ := securityServer(t)
	owner, err := s.store.CreateUser(t.Context(), "Owner", true, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(t.Context(), auth.HashToken("owner-cookie"), owner.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	cfg := settings.Settings{ServerName: "Original"}
	applied := 0
	h := New(Deps{Store: s.store, Log: s.log, FFprobeStatus: func() any { return nil }, Settings: func() settings.Settings { return cfg }, ApplySettings: func(_ context.Context, next settings.Settings) error { cfg = next; applied++; return nil }})
	for _, origin := range []string{"http://evil.arc.home:8099", "https://evil.example"} {
		for _, spoof := range []bool{false, true} {
			r := httptest.NewRequest("PATCH", "http://arc.home:8099/api/settings", strings.NewReader(`{"server_name":"Changed"}`))
			r.RemoteAddr = "192.168.1.20:45000"
			r.Header.Set("Origin", origin)
			if spoof {
				r.Header.Set("X-Forwarded-Host", strings.TrimPrefix(origin, "https://"))
				r.Header.Set("X-Forwarded-Host", strings.TrimPrefix(r.Header.Get("X-Forwarded-Host"), "http://"))
			}
			r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "owner-cookie"})
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden || applied != 0 || cfg.ServerName != "Original" {
				t.Errorf("cross-origin settings mutation admitted: status=%d applied=%d name=%s", w.Code, applied, cfg.ServerName)
			}
		}
	}
}
func TestUnversionedSetupRejectsCrossOriginBeforeCreatingAdmin(t *testing.T) {
	s, _ := securityServer(t)
	h := New(Deps{Store: s.store, Log: s.log, Settings: s.settings})
	// Rejection must precede normal setup/body validation, even on an already configured fixture.
	for _, path := range []string{"/api/auth/setup", "/api/v1/auth/setup"} {
		r := httptest.NewRequest("POST", "http://arc.home"+path, strings.NewReader(`{}`))
		r.Header.Set("Origin", "https://evil.example")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Errorf("%s: status %d", path, w.Code)
		}
	}
}
func TestFetchMetadataCrossSiteWithoutOriginIsRejected(t *testing.T) {
	calls := 0
	h := appCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(204) }))
	for _, site := range []string{"cross-site", "same-site"} {
		r := httptest.NewRequest("POST", "http://arc.home/api/auth/setup", nil)
		r.Header.Set("Sec-Fetch-Site", site)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 || calls != 0 {
			t.Errorf("%s mutation admitted", site)
		}
	}
}

func TestSameOriginChecksSchemeAndTrustedForwardedHost(t *testing.T) {
	for _, tc := range []struct {
		target, origin, forwarded string
		want                      bool
	}{
		{"http://arc.home:8099/api/settings", "http://arc.home:8099", "", true},
		{"http://arc.home/api/settings", "http://arc.home:80", "", true},
		{"http://arc.home/api/settings", "https://arc.home", "", false},
		{"http://arc.home/api/settings", "http://evil.example", "evil.example", false},
		{"http://arc.home/api/settings", "http://user@arc.home", "", false},
	} {
		r := httptest.NewRequest("PATCH", tc.target, nil)
		r.RemoteAddr = "192.168.1.20:1234"
		r.Header.Set("X-Forwarded-Host", tc.forwarded)
		if got := sameOrigin(r, tc.origin); got != tc.want {
			t.Errorf("%s origin %s: %v", tc.target, tc.origin, got)
		}
	}
	nets, _, err := auth.ParseNetworks("192.168.1.2")
	if err != nil {
		t.Fatal(err)
	}
	auth.SetTrustedProxies(nets)
	defer auth.SetTrustedProxies(nil)
	r := httptest.NewRequest("PATCH", "http://internal:8099/api/settings", nil)
	r.RemoteAddr = "192.168.1.2:1234"
	r.Header.Set("X-Forwarded-Host", "arc.example")
	r.Header.Set("X-Forwarded-Proto", "https")
	if !sameOrigin(r, "https://arc.example") {
		t.Fatal("trusted proxy origin rejected")
	}
}
