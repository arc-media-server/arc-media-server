// Package search ranks how well text matches what someone typed: regardless of case, accents
// and punctuation, preferring whole and leading matches, and forgiving a small typo in longer
// words ("matirx" finds The Matrix).
package search

import (
	"strings"
	"unicode"
)

var folds = map[rune]string{
	'à': "a", 'á': "a", 'â': "a", 'ã': "a", 'ä': "a", 'å': "a", 'ā': "a",
	'ç': "c", 'č': "c", 'ć': "c",
	'è': "e", 'é': "e", 'ê': "e", 'ë': "e", 'ē': "e", 'ě': "e",
	'ì': "i", 'í': "i", 'î': "i", 'ï': "i", 'ī': "i",
	'ñ': "n", 'ń': "n", 'ň': "n",
	'ò': "o", 'ó': "o", 'ô': "o", 'õ': "o", 'ö': "o", 'ø': "o", 'ō': "o",
	'ù': "u", 'ú': "u", 'û': "u", 'ü': "u", 'ū': "u", 'ů': "u",
	'ý': "y", 'ÿ': "y", 'ž': "z", 'ź': "z", 'ż': "z", 'š': "s", 'ś': "s", 'ř': "r", 'ł': "l", 'đ': "d",
	'ß': "ss", 'æ': "ae", 'œ': "oe", 'þ': "th",
}

// Normalize lowercases text, drops accents and turns punctuation into spaces ("Amélie!" -> "amelie").
func Normalize(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if f, ok := folds[r]; ok {
			b.WriteString(f)
			space = false
			continue
		}
		switch {
		case r == '\'' || r == '’':
			// "Ocean's" matches "oceans".
		case r == '&':
			if !space {
				b.WriteByte(' ')
			}
			b.WriteString("and ")
			space = true
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			space = false
		default:
			if !space {
				b.WriteByte(' ')
				space = true
			}
		}
	}
	return strings.TrimSpace(b.String())
}

// Score is how well text matches the query: 0 for no match, up to 100 for an exact one.
// query must already be normalized.
func Score(query, text string) int {
	if query == "" {
		return 0
	}
	t := Normalize(text)
	switch {
	case t == query:
		return 100
	case strings.HasPrefix(t, query):
		return 90
	case strings.HasPrefix(t, "the "+query) || strings.HasPrefix(t, "a "+query):
		return 88
	case strings.Contains(" "+t, " "+query):
		return 80
	case len(query) >= 4 && strings.Contains(t, query):
		// Inside a word, only for longer queries ("the" shouldn't find Matthew).
		return 70
	}
	// Every word typed matches a word of the text: exactly, as its start, or with a small typo.
	words := strings.Fields(t)
	total, fuzzy := 0, false
	for _, qw := range strings.Fields(query) {
		best := 0
		for _, w := range words {
			switch {
			case w == qw:
				best = max(best, 3)
			case strings.HasPrefix(w, qw):
				best = max(best, 2)
			case closeEnough(qw, w):
				best = max(best, 1)
			}
		}
		if best == 0 {
			return 0
		}
		if best == 1 {
			fuzzy = true
		}
		total += best
	}
	if fuzzy {
		return 30 + total
	}
	return 50 + total
}

// closeEnough allows one typo in words of 5 to 7 letters and two in longer ones, comparing the
// typed word with the text word (or its start, for a word still being typed).
func closeEnough(typed, word string) bool {
	n := len([]rune(typed))
	allowed := 0
	switch {
	case n >= 8:
		allowed = 2
	case n >= 5:
		allowed = 1
	}
	if allowed == 0 {
		return false
	}
	if distance(typed, word) <= allowed {
		return true
	}
	// "matri" against "matrix" is a prefix; "matirx" against "matrix..." is a typo in a prefix.
	if wr := []rune(word); len(wr) > n {
		return distance(typed, string(wr[:n])) <= allowed
	}
	return false
}

// distance is the Damerau-Levenshtein distance (with swapped neighbours as one edit).
func distance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	if d := len(ar) - len(br); d > 2 || d < -2 {
		return 3
	}
	prev2 := make([]int, len(br)+1)
	prev := make([]int, len(br)+1)
	cur := make([]int, len(br)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ar); i++ {
		cur[0] = i
		for j := 1; j <= len(br); j++ {
			cost := 1
			if ar[i-1] == br[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			if i > 1 && j > 1 && ar[i-1] == br[j-2] && ar[i-2] == br[j-1] {
				cur[j] = min(cur[j], prev2[j-2]+1)
			}
		}
		prev2, prev, cur = prev, cur, prev2
	}
	return prev[len(br)]
}
