// Package safeerr removes request locations and credentials from diagnostics that may be stored.
package safeerr

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"strings"
)

type transportError struct {
	cause   error
	message string
}

func (e transportError) Error() string        { return e.message }
func (e transportError) Is(target error) bool { return errors.Is(e.cause, target) }

// Transport classifies an HTTP construction/transport failure without rendering url.Error,
// its wrapped error, or a redirect location. Those strings can contain query or path secrets.
// Preserve cancellation identity without exposing the original error through Unwrap/As.
func Transport(err error) error {
	if err == nil {
		return nil
	}
	message := "HTTP request failed"
	var ne net.Error
	switch {
	case errors.Is(err, context.Canceled):
		message = "HTTP request canceled"
	case errors.Is(err, context.DeadlineExceeded):
		message = "HTTP request timed out"
	case errors.As(err, &ne) && ne.Timeout():
		message = "HTTP request timed out"
	}
	return transportError{err, message}
}

var urls = regexp.MustCompile(`(?i)https?://[^\s"'<>\\]+`)
var credentials = regexp.MustCompile(`(?i)(api[_-]?key|access_token|token|password)=([^\s&"'<>\\]+)`)

// JSONErrors also protects old provider explanations read from restored databases. Only error
// fields are scrubbed; artwork URLs and useful title/search information are preserved.
func JSONErrors(raw string) string {
	if raw == "" {
		return raw
	}
	var value any
	d := json.NewDecoder(strings.NewReader(raw))
	d.UseNumber()
	if d.Decode(&value) != nil {
		return ""
	}
	changed := false
	var visit func(any)
	visit = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			for key, entry := range v {
				if key == "error" {
					if msg, ok := entry.(string); ok {
						clean := credentials.ReplaceAllString(urls.ReplaceAllString(msg, "[request location removed]"), "$1=[redacted]")
						if clean != msg {
							v[key] = clean
							changed = true
						}
					}
				} else {
					visit(entry)
				}
			}
		case []any:
			for _, entry := range v {
				visit(entry)
			}
		}
	}
	visit(value)
	if !changed {
		return raw
	}
	b, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(b)
}
