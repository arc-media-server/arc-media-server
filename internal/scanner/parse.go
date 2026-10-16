package scanner

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MovieName is what can be learned about a movie from its file and folder names.
type MovieName struct {
	Title  string
	Year   int
	TMDBID int64
	IMDBID string
	Part   int // 1 for "cd1"/"part1", 0 when the movie is a single file
}

var (
	// {tmdb-603}, [tmdbid-603], {imdb-tt0133093} as used by Plex/Jellyfin naming guides.
	idTagRe     = regexp.MustCompile(`(?i)[{\[](tmdb|tmdbid|imdb|imdbid|tvdb|tvdbid)[-=]([a-z0-9]+)[}\]]`)
	braceRe     = regexp.MustCompile(`\{[^}]*\}`)
	bracketRe   = regexp.MustCompile(`\[[^\]]*\]`)
	parenYearRe = regexp.MustCompile(`^(.*?)[\s._-]*[(\[]((?:19|20)\d{2})[)\]]`)
	partRe      = regexp.MustCompile(`(?i)[\s._-]+(?:cd|part|pt|disc|disk)[\s._-]*([1-9])$`)
	sepRe       = regexp.MustCompile(`[\s._]+`)
	yearTokRe   = regexp.MustCompile(`^(?:19|20)\d{2}$`)
)

// Release-name tokens that mark the end of the title.
var stopTokens = map[string]bool{
	"480p": true, "576p": true, "720p": true, "1080p": true, "1080i": true, "2160p": true, "4k": true,
	"uhd": true, "bluray": true, "blu-ray": true, "bdrip": true, "brrip": true, "webrip": true,
	"web-dl": true, "webdl": true, "hdtv": true, "dvdrip": true, "remux": true, "x264": true,
	"x265": true, "h264": true, "h265": true, "hevc": true, "avc": true, "hdr": true, "hdr10": true,
	"10bit": true, "aac": true, "ac3": true, "dts": true, "atmos": true, "truehd": true,
}

// ParseMovie extracts a title and year from a movie path. The file name is preferred; the parent
// folder is used when only it carries a year (e.g. "The Matrix (1999)/movie.mkv").
func ParseMovie(path string) MovieName {
	base := filepath.Base(path)
	file := parseMovieName(strings.TrimSuffix(base, filepath.Ext(base)))
	dir := parseMovieName(filepath.Base(filepath.Dir(path)))

	m := file
	if file.Year == 0 && dir.Year != 0 {
		m = dir
		m.Part = file.Part
	}
	if m.TMDBID == 0 {
		m.TMDBID = max(file.TMDBID, dir.TMDBID)
	}
	if m.IMDBID == "" {
		m.IMDBID = file.IMDBID
		if m.IMDBID == "" {
			m.IMDBID = dir.IMDBID
		}
	}
	return m
}

func parseMovieName(s string) MovieName {
	var m MovieName
	for _, g := range idTagRe.FindAllStringSubmatch(s, -1) {
		switch strings.ToLower(g[1]) {
		case "tmdb", "tmdbid":
			m.TMDBID, _ = strconv.ParseInt(g[2], 10, 64)
		case "imdb", "imdbid":
			m.IMDBID = strings.ToLower(g[2])
		}
	}
	s = idTagRe.ReplaceAllString(s, " ")
	s = braceRe.ReplaceAllString(s, " ") // {edition-...} and similar
	// Drop [release group] / [1080p] tags but keep a bracketed year like [1999].
	s = bracketRe.ReplaceAllStringFunc(s, func(b string) string {
		if yearTokRe.MatchString(b[1 : len(b)-1]) {
			return b
		}
		return " "
	})
	s = strings.TrimSpace(s)

	if g := partRe.FindStringSubmatch(s); g != nil {
		m.Part, _ = strconv.Atoi(g[1])
		s = s[:len(s)-len(g[0])]
	}

	if g := parenYearRe.FindStringSubmatch(s); g != nil && strings.TrimSpace(g[1]) != "" {
		m.Title = cleanTitle(sepRe.ReplaceAllString(g[1], " "))
		m.Year, _ = strconv.Atoi(g[2])
		return m
	}

	// Scene-style names: Title.Year.1080p.BluRay...
	toks := strings.Fields(sepRe.ReplaceAllString(s, " "))
	for i, t := range toks {
		if i > 0 && stopTokens[strings.ToLower(t)] {
			toks = toks[:i]
			break
		}
	}
	// The last plausible year wins, so "Blade.Runner.2049.2017" keeps "2049" in the title.
	// A year as the first token is part of the title ("1917", "2012").
	maxYear := time.Now().Year() + 1
	yearIdx := -1
	for i := 1; i < len(toks); i++ {
		if yearTokRe.MatchString(toks[i]) {
			if y, _ := strconv.Atoi(toks[i]); y <= maxYear {
				yearIdx = i
			}
		}
	}
	if yearIdx > 0 {
		m.Year, _ = strconv.Atoi(toks[yearIdx])
		toks = toks[:yearIdx]
	}
	m.Title = cleanTitle(strings.Join(toks, " "))
	return m
}

func cleanTitle(s string) string {
	return strings.Trim(strings.Join(strings.Fields(s), " "), " -")
}
