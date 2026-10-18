package markers

import (
	"regexp"
	"strings"

	"ams/internal/store"
)

// fits keeps the timings that make sense for a file (issue #44): they must lie within it, have a
// sensible length for what they are, and be where that part usually is. When the source says how
// long the version it timed was, a file of a clearly different length (another cut, or a release
// with an extra logo) isn't trusted at all.
func fits(markers []store.Marker, durationMS, sourceDurationMS int64) []store.Marker {
	if durationMS <= 0 {
		return nil
	}
	if sourceDurationMS > 0 {
		diff := durationMS - sourceDurationMS
		if diff < 0 {
			diff = -diff
		}
		if diff > max(30_000, durationMS/50) {
			return nil
		}
	}
	var out []store.Marker
	for _, m := range markers {
		if m.EndMS > durationMS {
			if m.EndMS-durationMS > 5_000 {
				continue // runs past the end: timed on something else
			}
			m.EndMS = durationMS
		}
		length := m.EndMS - m.StartMS
		if m.StartMS < 0 || length <= 0 {
			continue
		}
		switch m.Kind {
		case "intro":
			// 5 seconds to 5 minutes, starting in the first 40%.
			if length < 5_000 || length > 300_000 || m.StartMS > durationMS*2/5 {
				continue
			}
		case "recap":
			if length < 5_000 || length > 480_000 || m.StartMS > durationMS*2/5 {
				continue
			}
		case "credits":
			// 5 seconds to 15 minutes, starting after the halfway point.
			if length < 5_000 || length > 900_000 || m.StartMS < durationMS/2 {
				continue
			}
		default:
			continue
		}
		out = append(out, m)
	}
	return out
}

var (
	introChapter   = regexp.MustCompile(`(?i)^(intro|opening|opening (credits|theme|titles)|title sequence|op|theme song)\b`)
	recapChapter   = regexp.MustCompile(`(?i)^(recap|previously)\b`)
	creditsChapter = regexp.MustCompile(`(?i)^(credits|end credits|closing credits|ending|ending (credits|theme)|ed|outro)\b`)
)

// movieCredits keeps only a movie's credits from its chapter markers, and only from a chapter
// that says "credits": a movie's chapter called "Ending" or "Opening" is usually part of the story.
func movieCredits(ms []store.Marker, chapters []store.Chapter) []store.Marker {
	var out []store.Marker
	for _, m := range ms {
		if m.Kind != "credits" {
			continue
		}
		for _, c := range chapters {
			if c.StartMS == m.StartMS && strings.Contains(strings.ToLower(c.Title), "credit") {
				out = append(out, m)
				break
			}
		}
	}
	return out
}

// fromChapters reads intro, recap and credits markers from a file's chapter names ("Opening",
// "Ending", "Recap"), as many releases (anime especially) name them.
func fromChapters(chapters []store.Chapter) []store.Marker {
	var out []store.Marker
	seen := map[string]bool{}
	for _, c := range chapters {
		title := strings.TrimSpace(c.Title)
		var kind string
		switch {
		case introChapter.MatchString(title):
			kind = "intro"
		case recapChapter.MatchString(title):
			kind = "recap"
		case creditsChapter.MatchString(title):
			kind = "credits"
		}
		if kind == "" || seen[kind] || c.EndMS <= c.StartMS {
			continue
		}
		seen[kind] = true
		out = append(out, store.Marker{Kind: kind, StartMS: c.StartMS, EndMS: c.EndMS, Source: "chapters"})
	}
	return out
}
