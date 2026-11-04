package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
)

const requestIDHeader = "X-Request-ID"

type correlationKey struct{}

func (s *Server) requestID(w http.ResponseWriter, r *http.Request) *http.Request {
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return r
	}
	if id, ok := r.Context().Value(correlationKey{}).(uint64); ok {
		w.Header().Set(requestIDHeader, strconv.FormatUint(id, 10))
		return r
	}
	id := s.requests.nextID.Add(1)
	// Never accept a caller-supplied ID: tokens, URLs and log-spoofing text are not identifiers.
	w.Header().Set(requestIDHeader, strconv.FormatUint(id, 10))
	return r.WithContext(context.WithValue(r.Context(), correlationKey{}, id))
}
func (s *Server) correlateRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, s.requestID(w, r)) })
}
func errorCode(status int) string {
	switch status {
	case 400:
		return "invalid_request"
	case 401:
		return "authentication_required"
	case 403:
		return "forbidden"
	case 404:
		return "not_found"
	case 405:
		return "method_not_allowed"
	case 408, 504:
		return "timeout"
	case 409:
		return "conflict"
	case 410:
		return "gone"
	case 413:
		return "payload_too_large"
	case 415:
		return "unsupported_media_type"
	case 416:
		return "range_not_satisfiable"
	case 422:
		return "validation_failed"
	case 429:
		return "rate_limited"
	case 500:
		return "internal_error"
	case 502:
		return "upstream_error"
	case 503:
		return "unavailable"
	default:
		return "request_failed"
	}
}

// Preserve error/needs/domain fields and never mutate a handler's map. New metadata is additive.
func errorMetadata(w http.ResponseWriter, status int, value any) any {
	if status < 400 {
		return value
	}
	out := map[string]any{}
	switch body := value.(type) {
	case map[string]string:
		if _, ok := body["error"]; !ok {
			return value
		}
		for k, v := range body {
			out[k] = v
		}
	case map[string]any:
		if _, ok := body["error"]; !ok {
			return value
		}
		for k, v := range body {
			out[k] = v
		}
	default:
		return value
	}
	if _, exists := out["code"]; !exists {
		out["code"] = errorCode(status)
	}
	if id := w.Header().Get(requestIDHeader); id != "" {
		out["request_id"] = id
	}
	return out
}
