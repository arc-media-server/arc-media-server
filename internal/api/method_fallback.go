package api

import (
	"net/http"
	"slices"
	"strings"
)

// The path-only /api/ fallback otherwise masks ServeMux's normal 405 behavior. Ask the same
// mux which methods match this path without executing authentication, handlers or DB work.
// Only the error path does these bounded lookups; normal requests retain their original route.
func apiFallback(mux *http.ServeMux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var allowed []string
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodConnect, http.MethodTrace} {
			probe := *r
			probe.Method = method
			_, pattern := mux.Handler(&probe)
			if pattern != "" && pattern != "/api/" && pattern != "/" {
				allowed = append(allowed, method)
			}
		}
		if len(allowed) > 0 {
			slices.Sort(allowed)
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		writeError(w, http.StatusNotFound, "no such endpoint")
	}
}
