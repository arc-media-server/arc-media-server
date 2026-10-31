package api

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Packaged TV apps (LG webOS .ipk, Samsung Tizen .wgt) run from their own local origin, so every
// call they make to /api/v1 is cross-origin. appCORS lets any origin call /api/v1, but a
// cross-origin request is treated as an app: its cookies are dropped and "sign in at home without
// a password" never applies, so the only thing that can authorise it is a Bearer token the app got
// by linking or signing in. A web page someone opens on the home network therefore can't use the
// browser's Arc cookie or the home-network bypass to read anything from Arc. The response never
// carries Access-Control-Allow-Credentials, so browsers won't attach cookies anyway.

type crossOriginKey struct{}

// isCrossOriginApp reports whether appCORS marked the request as a cross-origin app call.
func isCrossOriginApp(r *http.Request) bool {
	v, _ := r.Context().Value(crossOriginKey{}).(bool)
	return v
}

// appCORSRefused are /api/v1 endpoints a cross-origin caller may not use: first-run setup belongs
// in Arc's own web app, and must not be something any page on the home network can do and read.
var appCORSRefused = map[string]bool{"/api/v1/auth/setup": true}

func appCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") || origin == "" || sameOrigin(r, origin) {
			next.ServeHTTP(w, r)
			return
		}
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges, ETag, Retry-After")
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Range, If-None-Match")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if appCORSRefused[r.URL.Path] {
			writeError(w, http.StatusForbidden, "set up Arc in its own web app")
			return
		}
		r2 := r.Clone(context.WithValue(r.Context(), crossOriginKey{}, true))
		r2.Header.Del("Cookie")
		next.ServeHTTP(w, r2)
	})
}

// sameOrigin reports whether origin names the host the request was sent to (Arc's own web app,
// e.g. the API docs page), directly or through a proxy that reports the original host.
func sameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	fwd := r.Header.Get("X-Forwarded-Host")
	if i := strings.IndexByte(fwd, ','); i >= 0 {
		fwd = fwd[:i]
	}
	return fwd != "" && strings.EqualFold(u.Host, strings.TrimSpace(fwd))
}
