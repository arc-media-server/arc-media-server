package metadata

import (
	"context"
	"testing"
)

// A library's country (from ctx) chooses the age rating; without one the server's region does.
func TestRatingRegionFollowsLibrary(t *testing.T) {
	c := NewTMDB("")
	c.SetRegion("GB")
	ratings := map[string]string{"GB": "12A", "US": "PG-13", "DE": "FSK 12"}
	if got := pickRating(c.ratingRegion(context.Background()), ratings); got != "12A" {
		t.Fatalf("server default = %q", got)
	}
	if got := pickRating(c.ratingRegion(WithRegion(context.Background(), "DE")), ratings); got != "FSK 12" {
		t.Fatalf("library DE = %q", got)
	}
	if got := pickRating(c.ratingRegion(WithRegion(context.Background(), "JP")), ratings); got != "PG-13" {
		t.Fatalf("missing country falls back to US, got %q", got)
	}
}
