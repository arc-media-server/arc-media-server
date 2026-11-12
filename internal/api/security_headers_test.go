package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserSecurityHeadersOnAppDocsAndAPI(t *testing.T) {
	s, _ := securityServer(t)
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	for _, path := range []string{"/", "/docs/", "/movie/fixture", "/api/health", "/api/v1/health", "/api/not-found"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("X-Frame-Options") != "DENY" {
			t.Errorf("%s missing MIME/framing protection: %v", path, w.Header())
		}
		if !strings.HasPrefix(path, "/api/") {
			policy := w.Header().Get("Content-Security-Policy")
			for _, directive := range []string{"default-src 'self'", "object-src 'none'", "frame-ancestors 'none'", "base-uri 'none'"} {
				if !strings.Contains(policy, directive) {
					t.Errorf("%s missing %s", path, directive)
				}
			}
			if strings.Contains(policy, "'unsafe-eval'") {
				t.Error("unnecessary dynamic code execution allowed")
			}
		}
	}
}
