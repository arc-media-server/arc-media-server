package metadata

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestTMDBTransportErrorsDoNotExposeRequestURL(t *testing.T) {
	c := NewTMDB("fixture-private-key")
	c.http = &http.Client{Transport: scanTransport(func(r *http.Request) (*http.Response, error) { return nil, errors.New("fixture connection failed") })}
	tr := &Trace{}
	_, err := c.MatchMovie(WithTrace(t.Context(), tr), "Example", 2000, 0, "")
	if err == nil {
		t.Fatal("expected failure")
	}
	b, _ := json.Marshal(tr)
	for _, value := range []string{err.Error(), string(b)} {
		if strings.Contains(value, "fixture-private-key") || strings.Contains(value, "https://") || strings.Contains(value, "api_key") {
			t.Fatalf("transport error exposed request URL: %s", value)
		}
	}
}
