package store

import "testing"

func TestRatingAge(t *testing.T) {
	for rating, want := range map[string]int{
		"U": 0, "PG": 8, "12A": 12, "15": 15, "18": 18, "PG-13": 13, "R": 17, "TV-MA": 17, "TV-Y7": 7,
		"tv-14": 14, "FSK 16": 16, "16+": 16, "K-12": 12, "": -1, "NR": -1, "Unrated": -1, "XYZ": -1,
	} {
		if got := RatingAge(rating); got != want {
			t.Errorf("RatingAge(%q) = %d, want %d", rating, got, want)
		}
	}
}
