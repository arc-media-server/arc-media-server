package notify

import (
	"ams/internal/store"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

type retryTransport func(*http.Request) (*http.Response, error)

func (f retryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestWebhookRetriesTransientFailures(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses []int
		want     int
		ok       bool
	}{
		{"recovers", []int{500, 503, 204}, 3, true},
		{"exhausted", []int{503, 503, 503}, 3, false},
		{"rate limited", []int{429, 204}, 2, true},
		{"bad request", []int{400}, 1, false},
		{"transport", []int{0, 0, 204}, 3, true},
		{"transport exhausted", []int{0, 0, 0}, 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			var logs bytes.Buffer
			n := New(nil, slog.New(slog.NewTextHandler(&logs, nil)))
			var delays []time.Duration
			n.waitRetry = func(ctx context.Context, d time.Duration) error { delays = append(delays, d); return ctx.Err() }

			n.http = &http.Client{Transport: retryTransport(func(r *http.Request) (*http.Response, error) {
				status := tc.statuses[min(calls, len(tc.statuses)-1)]
				calls++
				if status == 0 {
					return nil, fmt.Errorf("transport error for https://example.invalid/token?key=fake-secret")
				}
				return &http.Response{StatusCode: status, Status: fmt.Sprintf("%d %s", status, http.StatusText(status)), Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header), Request: r}, nil
			})}
			got := n.deliver(t.Context(), store.Webhook{Name: "Fixture", URL: "https://example.invalid/hook"}, Event{Event: ServerProblem})
			if len(delays) != tc.want-1 {
				t.Errorf("backoff count=%d", len(delays))
			}
			for i, d := range delays {
				if d != time.Duration((i+1)*5)*time.Second {
					t.Errorf("wrong backoff %v", delays)
				}
			}
			if strings.Contains(logs.String(), "fake-secret") || strings.Contains(got.Status, "fake-secret") || strings.Contains(logs.String(), "https://") {
				t.Fatalf("unsafe diagnostics: %s %+v", logs.String(), got)
			}
			if calls != tc.want || got.OK != tc.ok {
				t.Fatalf("requests=%d ok=%v, want %d/%v", calls, got.OK, tc.want, tc.ok)
			}
		})
	}
}

func TestWebhookBackoffCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := waitWebhookRetry(ctx, time.Minute); !errors.Is(err, context.Canceled) {
		t.Fatalf("backoff ignored cancellation: %v", err)
	}
	var logs bytes.Buffer
	n := New(nil, slog.New(slog.NewTextHandler(&logs, nil)))
	attempts := 0
	n.http = &http.Client{Transport: retryTransport(func(r *http.Request) (*http.Response, error) {
		attempts++
		return &http.Response{StatusCode: 503, Status: "503 https://example.invalid/fake-secret", Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header), Request: r}, nil
	})}
	work, stop := context.WithCancel(t.Context())
	defer stop()
	n.waitRetry = func(ctx context.Context, _ time.Duration) error { stop(); return ctx.Err() }
	got := n.deliver(work, store.Webhook{Name: "Fixture", URL: "https://example.invalid/hook"}, Event{Event: ServerProblem})
	if attempts != 1 || got.OK || strings.Contains(logs.String(), "webhook failed") || strings.Contains(got.Status, "fake-secret") {
		t.Fatalf("canceled retry: attempts=%d result=%+v logs=%s", attempts, got, logs.String())
	}
}
