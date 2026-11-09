package notify

import (
	"ams/internal/store"
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

type privacyTransport func(*http.Request) (*http.Response, error)

func (f privacyTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestWebhookTransportErrorsDoNotExposeTokenURL(t *testing.T) {
	var logs bytes.Buffer
	n := New(nil, slog.New(slog.NewTextHandler(&logs, nil)))
	n.http = &http.Client{Transport: privacyTransport(func(r *http.Request) (*http.Response, error) { return nil, errors.New("fixture connection failed") })}
	d := n.deliver(t.Context(), store.Webhook{ID: "fixture", Name: "Test", URL: "https://hooks.slack.com/services/fixture-private-token"}, Event{Event: ServerProblem})
	if d.OK {
		t.Fatal("expected failure")
	}
	if strings.Contains(d.Status+logs.String(), "fixture-private-token") || strings.Contains(d.Status+logs.String(), "https://") {
		t.Fatalf("webhook result/log exposed request URL: %s %s", d.Status, logs.String())
	}
}
