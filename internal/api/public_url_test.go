package api

import (
	"ams/internal/auth"
	"ams/internal/settings"
	"context"
	"encoding/json"
	"net/http"
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

func TestPublicURLHomeSignInThroughTrustedProxy(t *testing.T) {
	s, user := securityServer(t)
	cfg := settings.Settings{LocalBypass: true, PublicURL: "https://arc.example.com"}
	h := New(Deps{Store: s.store, Log: s.log, Settings: func() settings.Settings { return cfg }})

	proxies, _, err := auth.ParseNetworks("198.51.100.2")
	if err != nil {
		t.Fatal(err)
	}
	home, _, err := auth.ParseNetworks("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	auth.SetTrustedProxies(proxies)
	t.Cleanup(func() { auth.SetTrustedProxies(nil) })
	auth.SetHomeNetworks(home)
	t.Cleanup(func() { auth.SetHomeNetworks(nil) })

	for _, tc := range []struct {
		name, peer, client, host, origin string
		disabled, local, bypass          bool
	}{
		{name: "home", peer: "198.51.100.2", client: "192.0.2.5", host: "arc.example.com", local: true, bypass: true},
		{name: "away", peer: "198.51.100.2", client: "203.0.113.5", host: "arc.example.com"},
		{name: "different host", peer: "198.51.100.2", client: "192.0.2.5", host: "other.example.com", local: true},
		{name: "untrusted proxy", peer: "198.51.100.3", client: "192.0.2.5", host: "arc.example.com"},
		{name: "bypass disabled", peer: "198.51.100.2", client: "192.0.2.5", host: "arc.example.com", local: true, disabled: true},
		{name: "cross-origin", peer: "198.51.100.2", client: "192.0.2.5", host: "arc.example.com", origin: "https://other.example.com", local: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg.LocalBypass = !tc.disabled
			request := func(method, path, body string) *httptest.ResponseRecorder {
				r := httptest.NewRequest(method, "http://arc:8099"+path, strings.NewReader(body))
				r.RemoteAddr = tc.peer + ":4000"
				r.Header.Set("X-Forwarded-Host", tc.host)
				r.Header.Set("X-Forwarded-Proto", "https")
				r.Header.Set("X-Forwarded-For", tc.client)
				if tc.origin != "" {
					r.Header.Set("Origin", tc.origin)
				}
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				return w
			}
			status := request(http.MethodGet, "/api/v1/auth/status", "")
			var state authStatusDTO
			if err := json.Unmarshal(status.Body.Bytes(), &state); err != nil || status.Code != http.StatusOK {
				t.Fatalf("status = %d: %v", status.Code, err)
			}
			if state.Local != tc.local || state.Bypass != tc.bypass {
				t.Fatalf("status local=%v bypass=%v, want local=%v bypass=%v", state.Local, state.Bypass, tc.local, tc.bypass)
			}
			profile := request(http.MethodPost, "/api/v1/auth/profile", `{"id":"`+user.ID+`"}`)
			want := http.StatusUnauthorized
			if tc.bypass {
				want = http.StatusOK
			}
			if profile.Code != want || (len(profile.Result().Cookies()) > 0) != tc.bypass {
				t.Fatalf("profile selection status = %d, want %d", profile.Code, want)
			}
		})
	}
}
