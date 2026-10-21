package metadata

import (
	"regexp"
	"strings"
)

var titleNumbers = regexp.MustCompile(`\d+`)

func comparableTitle(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	for _, article := range []string{"the ", "an ", "a "} {
		if strings.HasPrefix(s, article) {
			s = strings.TrimPrefix(s, article)
			break
		}
	}
	return Normalize(s)
}

func titleSimilarity(a, b string) float64 {
	if a == b && a != "" {
		return 1
	}
	if strings.Join(titleNumbers.FindAllString(a, -1), ",") != strings.Join(titleNumbers.FindAllString(b, -1), ",") {
		return 0
	}
	x, y := []rune(a), []rune(b)
	if len(x) == 0 || len(y) == 0 || len(x) > 256 || len(y) > 256 {
		return 0
	}
	row := make([]int, len(y)+1)
	for i := range row {
		row[i] = i
	}
	for i, c := range x {
		prev := row[0]
		row[0] = i + 1
		for j, d := range y {
			old := row[j+1]
			cost := 0
			if c != d {
				cost = 1
			}
			row[j+1] = min(row[j+1]+1, row[j]+1, prev+cost)
			prev = old
		}
	}
	return 1 - float64(row[len(y)])/float64(max(len(x), len(y)))
}

// Require a plausible, clearly better candidate. Explicit provider IDs bypass this
// entirely; uncertain titles stay visible for Fix Match instead of guessing.
func confidentMatch(title string, candidates []Candidate) int64 {
	want := comparableTitle(title)
	scores := map[int64]float64{}
	for _, c := range candidates {
		score := max(titleSimilarity(want, comparableTitle(c.Title)), titleSimilarity(want, comparableTitle(c.OriginalTitle)))
		scores[c.ID] = max(scores[c.ID], score)
	}
	var id int64
	best, second := 0.0, 0.0
	for candidate, score := range scores {
		if score > best {
			second = best
			best = score
			id = candidate
		} else if score > second {
			second = score
		}
	}
	if best < 0.88 || best-second < 0.08 {
		return 0
	}
	return id
}
