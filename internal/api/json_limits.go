package api

import "net/http"

const authenticatedJSONBytes int64 = 2 << 20

// boundAuthenticatedBody runs after authentication. Raw artwork has its own 20 MiB
// bound; identify those routes by mux pattern, never by a spoofable Content-Type.
// Smaller endpoint-specific decoder limits remain in effect inside this ceiling.
func boundAuthenticatedBody(w http.ResponseWriter, r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return true
	}
	switch r.Pattern {
	case "POST /api/items/{id}/artwork/{type}", "POST /api/collections/{id}/artwork/{type}", "POST /api/series/{id}/seasons/{n}/artwork":
		return true
	}
	if r.ContentLength > authenticatedJSONBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "JSON requests can be up to 2 MiB")
		return false
	}
	if r.Body != nil && r.Body != http.NoBody {
		r.Body = http.MaxBytesReader(w, r.Body, authenticatedJSONBytes)
	}
	return true
}
