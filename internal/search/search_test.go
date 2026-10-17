package search

import "testing"

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"Amélie":                     "amelie",
		"Ocean's Eleven":             "oceans eleven",
		"Fast & Furious":             "fast and furious",
		"  Spider-Man: No Way Home ": "spider man no way home",
		"Léon: The Professional":     "leon the professional",
		"Æon Flux":                   "aeon flux",
	} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScore(t *testing.T) {
	q := func(s string) string { return Normalize(s) }
	cases := []struct {
		query, text string
		match       bool
	}{
		{"matrix", "The Matrix", true},
		{"the matrix", "The Matrix", true},
		{"matr", "The Matrix", true},
		{"matirx", "The Matrix", true},     // swapped letters
		{"matrx", "The Matrix", true},      // a missing letter
		{"amelie", "Amélie", true},         // accents
		{"oceans", "Ocean's Eleven", true}, // punctuation
		{"runner blade", "Blade Runner", true},
		{"interstelar", "Interstellar", true},
		{"heat", "Heat", true},
		{"hwat", "Heat", false}, // too short to forgive a typo
		{"xyz", "The Matrix", false},
		{"matrix", "Inception", false},
		{"the", "Matthew McConaughey", false}, // not inside a word for short queries
		{"atri", "The Matrix", true},
	}
	for _, c := range cases {
		if got := Score(q(c.query), c.text); (got > 0) != c.match {
			t.Errorf("Score(%q, %q) = %d, want match=%v", c.query, c.text, got, c.match)
		}
	}
	// Better matches score higher.
	order := []string{"Heat", "Heat Wave", "The Heat", "Cheating Heat", "Haet"}
	for i := 1; i < len(order)-1; i++ {
		if Score("heat", order[i-1]) < Score("heat", order[i]) {
			t.Errorf("%q (%d) should rank above %q (%d)", order[i-1], Score("heat", order[i-1]), order[i], Score("heat", order[i]))
		}
	}
}
