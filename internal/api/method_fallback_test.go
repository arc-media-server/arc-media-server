package api

import (
	"net/http/httptest"
	"testing"
)

func TestUnsupportedAPIMethodsReturnAllowWithoutRunningHandlers(t *testing.T) {
	s, _ := securityServer(t)
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	for _, prefix := range []string{"/api", "/api/v1"} {
		for _, tc := range []struct {
			method, path, allow string
			status              int
		}{
			{"POST", "/items", "GET, HEAD", 405},
			{"PUT", "/auth/status", "GET, HEAD", 405},
			{"POST", "/playlists/fixture", "DELETE, GET, HEAD, PATCH", 405},
			{"DELETE", "/no-such-endpoint", "", 404},
			{"GET", "/health", "", 200},
			{"HEAD", "/health", "", 200},
		} {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest(tc.method, prefix+tc.path, nil))
			if w.Code != tc.status || w.Header().Get("Allow") != tc.allow {
				t.Errorf("%s %s%s: status %d Allow=%q; wanted %d %q", tc.method, prefix, tc.path, w.Code, w.Header().Get("Allow"), tc.status, tc.allow)
			}
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/library/fixture", nil))
	if w.Code != 200 {
		t.Fatalf("SPA fallback changed: %d", w.Code)
	}
}
