package metadata

import (
	"regexp"
	"strings"
)

var titleNumbers = regexp.MustCompile(`\d+`)

// providerTag is how a provider tells same-named titles apart at the end of a name: "Doctor
// Who (2005)", "The Office (US)". It isn't part of the title.
var providerTag = regexp.MustCompile(`\s*\((\d{4}|[A-Za-z]{2,3})\)\s*$`)

// mainTitle is the part before a subtitle ("Cosmos" in "Cosmos - A Spacetime Odyssey" or
// "Cosmos: A Spacetime Odyssey"), or "" when there's no subtitle.
func mainTitle(s string) string {
	for _, sep := range []string{" - ", ": ", " – "} {
		if i := strings.Index(s, sep); i > 0 {
			return strings.TrimSpace(s[:i])
		}
	}
	return ""
}

func comparableTitle(s string) string {
	s = providerTag.ReplaceAllString(strings.TrimSpace(s), "")
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
//
// When several titles are about as close (a remake, or "Alien" beside an obscure "The Alien"),
// more evidence decides, never result order alone: the file's year (results more than a year
// away don't count), then one being far better known (TMDB votes), then the exact year, then
// the title word for word. Anything still tied is left for Fix Match.
func confidentMatch(title string, year int, candidates []Candidate) int64 {
	want := comparableTitle(title)
	type contender struct {
		Candidate
		score float64
	}
	byID := map[int64]*contender{}
	var order []int64
	for _, c := range candidates {
		score := max(titleSimilarity(want, comparableTitle(c.Title)), titleSimilarity(want, comparableTitle(c.OriginalTitle)))
		if prev, ok := byID[c.ID]; ok {
			prev.score = max(prev.score, score)
			prev.Votes = max(prev.Votes, c.Votes)
			continue
		}
		byID[c.ID] = &contender{c, score}
		order = append(order, c.ID)
	}
	best := 0.0
	for _, c := range byID {
		best = max(best, c.score)
	}
	if best < 0.88 {
		return subtitleMatch(title, year, candidates)
	}
	// Everything within reach of the best title is a contender.
	var tied []*contender
	for _, id := range order {
		if c := byID[id]; c.score > best-0.08 {
			tied = append(tied, c)
		}
	}
	keep := func(ok func(*contender) bool) {
		var kept []*contender
		for _, c := range tied {
			if ok(c) {
				kept = append(kept, c)
			}
		}
		if len(kept) > 0 {
			tied = kept
		}
	}
	if year > 0 {
		keep(func(c *contender) bool { return c.Year >= year-1 && c.Year <= year+1 })
	}
	if len(tied) > 1 {
		// One of the closest titles far better known than every other contender: at least 100
		// votes and ten times any other.
		var top *contender
		for _, c := range tied {
			if c.score == best && (top == nil || c.Votes > top.Votes) {
				top = c
			}
		}
		next := 0
		for _, c := range tied {
			if c != top {
				next = max(next, c.Votes)
			}
		}
		if top.Votes >= 100 && top.Votes >= 10*next {
			tied = []*contender{top}
		}
	}
	if len(tied) > 1 && year > 0 {
		keep(func(c *contender) bool { return c.Year == year })
	}
	if len(tied) > 1 {
		literal := Normalize(strings.ToLower(strings.TrimSpace(title)))
		keep(func(c *contender) bool {
			return literal != "" && (Normalize(strings.ToLower(c.Title)) == literal || Normalize(strings.ToLower(c.OriginalTitle)) == literal)
		})
	}
	if len(tied) != 1 || tied[0].score < best {
		return 0
	}
	return tied[0].ID
}

// subtitleMatch accepts a title the provider knows without its subtitle ("Cosmos" for a folder
// named "Cosmos - A Spacetime Odyssey"), but only the one such candidate from exactly the year.
func subtitleMatch(title string, year int, candidates []Candidate) int64 {
	main := comparableTitle(mainTitle(title))
	if main == "" || year == 0 {
		return 0
	}
	var id int64
	for _, c := range candidates {
		if c.Year != year || (comparableTitle(c.Title) != main && comparableTitle(c.OriginalTitle) != main) {
			continue
		}
		if id != 0 && id != c.ID {
			return 0
		}
		id = c.ID
	}
	return id
}
