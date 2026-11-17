package api

import (
	"ams/internal/auth"
	"ams/internal/settings"
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicURLSettingsValidationAndClear(t *testing.T) {
	s, _ := securityServer(t)
	s.ffprobeStatus = func() any { return nil }
	current := settings.Settings{UPnP: false}
	s.settings = func() settings.Settings { return current }
	s.applySettings = func(_ context.Context, _ settings.Settings, next settings.Settings) error { current = next; return nil }
	for _, tc := range []struct {
		body string
		code int
		want string
	}{
		{`{"public_url":" https://ARC.example.com:443/ "}`, 200, "https://arc.example.com"},
		{`{"public_url":"https://u:secret@example.com"}`, 400, "https://arc.example.com"},
		{`{"public_url":"https://arc.example.com/path"}`, 400, "https://arc.example.com"},
		{`{"public_url":null}`, 400, "https://arc.example.com"},
		{`{"public_url":""}`, 200, ""},
	} {
		w := httptest.NewRecorder()
		s.updateSettings(w, httptest.NewRequest("PATCH", "/api/settings", strings.NewReader(tc.body)))
		if w.Code != tc.code || current.PublicURL != tc.want || current.UPnP {
			t.Fatalf("%s: %d URL%q upnp%v", tc.body, w.Code, current.PublicURL, current.UPnP)
		}
	}
}

func TestPublicURLCastAddressKeepsLANAndDoesNotTrustHeaders(t *testing.T) {
	s, _ := securityServer(t)
	s.settings = func() settings.Settings {
		return settings.Settings{RemoteAccess: true, PublicURL: "https://arc.example.com"}
	}
	r := httptest.NewRequest("POST", "http://192.168.1.2:8099/api/cast", nil)
	r.RemoteAddr = "203.0.113.5:4000"
	if got := s.castAddress(r); got != "https://arc.example.com" {
		t.Fatal(got)
	}
	r.RemoteAddr = "127.0.0.1:4000"
	if got := s.castAddress(r); got != "http://192.168.1.2:8099" {
		t.Fatal("local override", got)
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := castBase(r); got != "http://192.168.1.2:8099" {
		t.Fatal("untrusted HTTPS header accepted", got)
	}
	nets, _, err := auth.ParseNetworks("127.0.0.1/32")
	if err != nil {
		t.Fatal(err)
	}
	auth.SetTrustedProxies(nets)
	defer auth.SetTrustedProxies(nil)
	if got := castBase(r); got != "https://192.168.1.2:8099" {
		t.Fatal("trusted proxy scheme lost", got)
	}
	s.settings = func() settings.Settings {
		return settings.Settings{RemoteAccess: false, PublicURL: "https://arc.example.com"}
	}
	r.RemoteAddr = "203.0.113.5:4000"
	if got := s.castAddress(r); got != "http://192.168.1.2:8099" {
		t.Fatal("disabled override", got)
	}
}

func TestPublicURLDoesNotGrantHomeSignInBypass(t *testing.T) {
	s, _ := securityServer(t)
	s.settings = func() settings.Settings {
		return settings.Settings{LocalBypass: true, RemoteAccess: true, PublicURL: "https://arc.example.com"}
	}
	r := httptest.NewRequest("GET", "https://arc.example.com/api/auth/status", nil)
	r.RemoteAddr = "127.0.0.1:4000"
	if s.bypassApplies(r) {
		t.Fatal("public URL became a password-free trusted hostname")
	}
}
