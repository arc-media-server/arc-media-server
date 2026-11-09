package ratings

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

type privacyTransport func(*http.Request) (*http.Response, error)

func (f privacyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOMDbTransportErrorDoesNotExposeKeyURL(t *testing.T) {
	c := &http.Client{Transport: privacyTransport(func(r *http.Request) (*http.Response, error) { return nil, errors.New("fixture connection failed") })}
	_, _, err := omdbScores(t.Context(), c, "fixture-private-key", "tt1234")
	if err == nil {
		t.Fatal("expected failure")
	}
	if strings.Contains(err.Error(), "fixture-private-key") || strings.Contains(err.Error(), "http") {
		t.Fatalf("transport error exposes request URL: %s", err)
	}
}
