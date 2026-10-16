package scanner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// EpisodeName is what can be learned about an episode from its path. Numbers are in the
// library's source ordering (how the files are named).
type EpisodeName struct {
	Season   int   // -1 when unknown
	Episodes []int // several for multi-episode files
	Absolute bool  // Episodes are absolute numbers rather than per-season
	AirDate  string
}

var (
	// S01E02, s1e2, S01.E02, S2024E05
	seRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,4})[ ._-]*e(\d{1,4})`)
	// Further episodes after the first: E03, -E03, -03, " E03"
	moreEpRe = regexp.MustCompile(`(?i)^(?:[ ._]*(-)?[ ._]*e|(-))(\d{1,4})`)
	// 1x02, 1x02-03, 1x02-1x03
	xRe     = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(\d{1,2})x(\d{2,3})(?:[^0-9a-z]|$)`)
	xMoreRe = regexp.MustCompile(`(?i)^[ ._]*-[ ._]*(?:\d{1,2}x)?(\d{2,3})(?:[^0-9a-z]|$)`)
	// 2024-03-05, 2024.03.05
	dateRe = regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})[-. _](0[1-9]|1[0-2])[-. _](0[1-9]|[12]\d|3[01])(?:[^0-9]|$)`)
	// E05, Ep05, Episode 5 without a season
	epOnlyRe = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:e|ep|episode)[ ._-]?(\d{1,4})(?:[^0-9]|$)`)
	// Anime style: "Show Name - 012" or "Show Name - 012v2"
	dashNumRe = regexp.MustCompile(`(?:^|\s)-\s*(\d{1,4})(?:v\d)?(?:\s|$)`)

	seasonDirRe   = regexp.MustCompile(`(?i)^(?:season|series|staffel|saison|s)[ ._-]*(\d{1,4})\b`)
	specialsDirRe = regexp.MustCompile(`(?i)^specials?$`)
	parenGroupRe  = regexp.MustCompile(`\([^)]*\)`)
)

// ParseEpisodePath parses a file below a TV library root. It returns a key grouping files of the
// same series, the series name, and the episode. ok is false when the file doesn't look like an
// episode.
func ParseEpisodePath(root, path string) (seriesKey string, series MovieName, ep EpisodeName, ok bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", series, ep, false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	base := parts[len(parts)-1]
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	ep, matchStart, ok := parseEpisodeName(stem)
	if !ok {
		return "", series, ep, false
	}

	// Season folders between the series folder and the file.
	folderSeason := -1
	var dirs []string
	if len(parts) > 2 {
		dirs = parts[1 : len(parts)-1]
	}
	for _, dir := range dirs {
		if specialsDirRe.MatchString(dir) {
			folderSeason = 0
		} else if g := seasonDirRe.FindStringSubmatch(dir); g != nil {
			folderSeason, _ = strconv.Atoi(g[1])
		}
	}
	if ep.Season < 0 && !ep.Absolute && len(ep.Episodes) > 0 {
		if folderSeason >= 0 {
			ep.Season = folderSeason
		} else {
			ep.Absolute = true
		}
	}

	if len(parts) >= 2 {
		series = parseMovieName(parts[0])
		seriesKey = "dir:" + strings.ToLower(parts[0])
	} else {
		// File directly in the library root: the series name is whatever precedes the episode marker.
		series = parseMovieName(strings.TrimSpace(stem[:matchStart]))
		seriesKey = "name:" + strings.ToLower(series.Title)
	}
	if series.Title == "" {
		return "", series, ep, false
	}
	return seriesKey, series, ep, true
}

// parseEpisodeName finds the episode marker in a file name. matchStart is where it begins.
func parseEpisodeName(stem string) (ep EpisodeName, matchStart int, ok bool) {
	ep.Season = -1

	if m := seRe.FindStringSubmatchIndex(stem); m != nil {
		ep.Season = atoi(stem[m[2]:m[3]])
		ep.Episodes = []int{atoi(stem[m[4]:m[5]])}
		rest := stem[m[1]:]
		for {
			g := moreEpRe.FindStringSubmatchIndex(rest)
			if g == nil || !boundary(rest, g[1]) {
				break
			}
			n := atoi(rest[g[6]:g[7]])
			last := ep.Episodes[len(ep.Episodes)-1]
			isRange := g[2] >= 0 || g[4] >= 0
			if n <= last {
				break
			}
			if isRange && n-last <= 20 {
				for i := last + 1; i <= n; i++ {
					ep.Episodes = append(ep.Episodes, i)
				}
			} else {
				ep.Episodes = append(ep.Episodes, n)
			}
			rest = rest[g[1]:]
		}
		return ep, m[0], true
	}

	if m := xRe.FindStringSubmatchIndex(stem); m != nil {
		ep.Season = atoi(stem[m[2]:m[3]])
		first := atoi(stem[m[4]:m[5]])
		ep.Episodes = []int{first}
		if g := xMoreRe.FindStringSubmatch(stem[m[5]:]); g != nil {
			if n := atoi(g[1]); n > first && n-first <= 20 {
				for i := first + 1; i <= n; i++ {
					ep.Episodes = append(ep.Episodes, i)
				}
			}
		}
		return ep, m[0], true
	}

	if m := dateRe.FindStringSubmatchIndex(stem); m != nil {
		ep.AirDate = stem[m[2]:m[3]] + "-" + stem[m[4]:m[5]] + "-" + stem[m[6]:m[7]]
		return ep, m[0], true
	}

	if m := epOnlyRe.FindStringSubmatchIndex(stem); m != nil {
		ep.Episodes = []int{atoi(stem[m[2]:m[3]])}
		return ep, m[0], true
	}

	// Anime-style absolute numbering; strip [group]/[1080p] and (year) tags first so they don't confuse it.
	cleaned := parenGroupRe.ReplaceAllStringFunc(bracketRe.ReplaceAllStringFunc(stem, blank), blank)
	if m := dashNumRe.FindStringSubmatchIndex(cleaned); m != nil {
		ep.Episodes = []int{atoi(cleaned[m[2]:m[3]])}
		return ep, m[0], true
	}
	return ep, 0, false
}

// boundary reports whether position i in s is the end of a token (not followed by a letter or
// digit, other than the "e" of a further episode marker).
func boundary(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	c := s[i]
	return !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') || c == 'e' || c == 'E'
}

// blank replaces a match with spaces of the same length so indexes stay valid.
func blank(s string) string { return strings.Repeat(" ", len(s)) }

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
