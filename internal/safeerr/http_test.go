package safeerr

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestTransportKeepsCancellationWithoutLeakingNestedLocations(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, errors.New("redirect https://secret.example/private-token failed")} {
		original := &url.Error{Op: "Get", URL: "https://provider.example/?api_key=private-key", Err: cause}
		safe := Transport(original)
		if !errors.Is(safe, cause) {
			t.Fatal("lost error identity")
		}
		if strings.Contains(safe.Error(), "http:") || strings.Contains(safe.Error(), "https:") || strings.Contains(safe.Error(), "private-") {
			t.Fatalf("leak: %s", safe)
		}
		var unsafe *url.Error
		if errors.As(safe, &unsafe) {
			t.Fatal("exposed raw url.Error through As")
		}
	}
}
func TestJSONErrorsPreservesArtworkAndIdentifierPrecision(t *testing.T) {
	raw := `{"id":9007199254740993,"poster":"https://image.example/art.jpg","steps":[{"error":"Get https://provider.example/?api_key=private-key: failed"}]}`
	got := JSONErrors(raw)
	if strings.Contains(got, "private-key") || !strings.Contains(got, "9007199254740993") || !strings.Contains(got, "https://image.example/art.jpg") {
		t.Fatalf("incorrect trace scrub: %s", got)
	}
	if JSONErrors(got) != got {
		t.Fatal("scrub not idempotent")
	}
}
