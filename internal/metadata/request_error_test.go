package metadata

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type errorFixtureTransport func(*http.Request) (*http.Response, error)

func (f errorFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProviderFailuresDoNotEchoUntrustedStatusOrLoginMessage(t *testing.T) {
	for _, provider := range []string{"tmdb", "tvdb"} {
		t.Run(provider, func(t *testing.T) {
			const marker = "fixture-private-marker"
			transport := errorFixtureTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 401, Status: "401 " + marker, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"message":"` + marker + `"}`)), Request: r}, nil
			})
			client := &http.Client{Transport: transport}
			var out map[string]any
			var err error
			if provider == "tmdb" {
				c := NewTMDB("fixture-not-a-real-key")
				c.http = client
				err = c.get(context.Background(), "/movie/1", url.Values{}, &out)
			} else {
				c := NewTVDB("fixture-not-a-real-key")
				c.http = client
				err = c.get(context.Background(), "/series/1", nil, &out)
			}
			if err == nil {
				t.Fatal("provider failure ignored")
			}
			if strings.Contains(err.Error(), marker) {
				t.Errorf("provider failure echoed untrusted response: %s", err)
			}
		})
	}
}

func TestProviderErrorTypingPreservesExpectedNotFoundAndCancellation(t *testing.T) {
	for _, status := range []int{401, 404} {
		c := NewTMDB("fixture-not-a-real-key")
		c.http = &http.Client{Transport: errorFixtureTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Status: http.StatusText(status), Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
		})}
		var out map[string]any
		err := c.get(context.Background(), "/movie/1", nil, &out)
		if status == 404 {
			if err != ErrNotFound {
				t.Fatalf("not-found identity changed: %v", err)
			}
		} else {
			var upstream *RequestError
			if !errors.As(err, &upstream) || upstream.Provider != "tmdb" {
				t.Fatalf("provider error not typed: %v", err)
			}
		}
	}
	c := NewTMDB("fixture-not-a-real-key")
	c.http = &http.Client{Transport: errorFixtureTransport(func(r *http.Request) (*http.Response, error) { return nil, context.Canceled })}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out map[string]any
	if err := c.get(ctx, "/movie/1", nil, &out); err != context.Canceled {
		t.Fatalf("cancel identity changed: %v", err)
	}
}
