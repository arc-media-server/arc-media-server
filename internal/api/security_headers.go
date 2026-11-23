package api

import "net/http"

// MIME/framing protections also cover API responses and early CORS/authentication errors.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// ProtectDatabaseSetup applies the normal browser protections while Arc awaits its database.
func ProtectDatabaseSetup(next http.Handler) http.Handler {
	return securityHeaders(appCORS(next))
}
