package api

import (
	"ams/internal/store"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUnauthenticatedBodiesAreBounded(t *testing.T) {
	for _, kind := range []string{"setup", "login", "profile", "friend", "invite", "link", "approve", "password"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := securityServer(t)
			handlers := map[string]http.HandlerFunc{"setup": s.setup, "login": s.login, "profile": s.chooseProfile, "friend": s.friendLogin, "invite": s.acceptInvite, "link": s.startLink, "approve": s.approveLink, "password": s.changePassword}
			// Include every known field so no endpoint can ignore a large unknown JSON value.
			large := strings.Repeat("x", 128<<10)
			body := `{"name":"` + large + `","password":"` + large + `","pin":"` + large + `","id":"` + large + `","current":"` + large + `","new":"` + large + `","device":"` + large + `","code":"` + large + `"}`
			r := httptest.NewRequest("POST", "http://arc/api/auth/"+kind, strings.NewReader(body))
			r.RemoteAddr = "192.168.1.20:1234"
			r = r.WithContext(context.WithValue(r.Context(), userKey, &store.User{}))
			w := httptest.NewRecorder()
			handlers[kind](w, r)
			if w.Code != http.StatusBadRequest && w.Code != http.StatusRequestEntityTooLarge {
				t.Fatalf("oversized body reached %s handler: status=%d", kind, w.Code)
			}
		})
	}
}
func TestAnonymousLinkSlotsArePerClient(t *testing.T) {
	s, _ := securityServer(t)
	start := func(ip string) int {
		r := httptest.NewRequest("POST", "http://arc/api/auth/link", strings.NewReader(`{"device":"Test TV"}`))
		r.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		s.startLink(w, r)
		return w.Code
	}
	for range 5 {
		if got := start("192.168.1.20"); got != 200 {
			t.Fatalf("initial link: %d", got)
		}
	}
	if got := start("192.168.1.20"); got != 429 {
		t.Errorf("one client can exhaust global slots: %d", got)
	}
	if got := start("192.168.1.21"); got != 200 {
		t.Errorf("another client blocked: %d", got)
	}
}
