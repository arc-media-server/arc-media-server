package metadata

import "testing"

func TestShowStatus(t *testing.T) {
	for in, want := range map[string]string{
		"Returning Series": "continuing", "Continuing": "continuing", "Ended": "ended",
		"Canceled": "cancelled", "In Production": "upcoming", "Upcoming": "upcoming", "": "",
	} {
		if got := showStatus(in); got != want {
			t.Errorf("showStatus(%q) = %q, want %q", in, got, want)
		}
	}
}
