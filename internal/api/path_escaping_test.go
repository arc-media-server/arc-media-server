package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionedAliasPreservesEscapedSegmentBoundaries(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "query": r.URL.RawQuery})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { writeError(w, 404, "no such endpoint") })
	h := versioned(mux)
	for _, suffix := range []string{"plain", "a%2Fb", "a%2fb", "%2e%2e", "a%252Fb", "space%20name"} {
		t.Run(suffix, func(t *testing.T) {
			call := func(prefix string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("GET", "http://arc"+prefix+"/items/"+suffix+"?q=keep%2Fquery", nil)
				path, raw, uri := r.URL.Path, r.URL.RawPath, r.RequestURI
				w := httptest.NewRecorder()
				h.ServeHTTP(w, r)
				if r.URL.Path != path || r.URL.RawPath != raw || r.RequestURI != uri {
					t.Fatal("alias rewrote caller request URL")
				}
				return w
			}
			legacy := call("/api")
			for _, prefix := range []string{"/api/v1", "/api/v%31", "/%61pi/v1"} {
				current := call(prefix)
				if current.Code != legacy.Code || current.Body.String() != legacy.Body.String() {
					t.Errorf("segment %s changed through %s: status/body %d %s vs %d %s", suffix, prefix, current.Code, current.Body.String(), legacy.Code, legacy.Body.String())
				}
			}
		})
	}
}

func TestVersionedAliasDoesNotDecodeNamespaceSeparators(t *testing.T) {
	reached := false
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(204) })
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	for _, path := range []string{"/api%2Fv1/health", "/api/v1%2Fhealth"} {
		reached = false
		w := httptest.NewRecorder()
		versioned(mux).ServeHTTP(w, httptest.NewRequest("GET", "http://arc"+path, nil))
		if reached || w.Code != 404 {
			t.Errorf("encoded namespace separator became a real endpoint: %s status=%d", path, w.Code)
		}
	}
}

func TestActualAPIAliasesAgreeOnEscapedItemAuthentication(t *testing.T) {
	s, _ := securityServer(t)
	h := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	for _, prefix := range []string{"/api", "/api/v1"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://arc"+prefix+"/items/a%2Fb", nil))
		if w.Code != 401 || !strings.Contains(w.Body.String(), "sign in") {
			t.Errorf("alias used a different route boundary: %s %d %s", prefix, w.Code, w.Body.String())
		}
	}
}
