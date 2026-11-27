package store

import (
	"path/filepath"
	"regexp"
	"strings"
)

// A title can have several files of the same part: different editions (Director's Cut) and
// versions (4K and 1080p). The edition is read from the file name each time it's needed.

var (
	editionTagRe = regexp.MustCompile(`(?i)\{edition-([^}]+)\}`)
	yearRe       = regexp.MustCompile(`\b(?:19|20)\d{2}\b`)
)

// Words in file names that name an edition, longest first so "extended cut" wins over "extended".
var editionWords = []struct {
	re   *regexp.Regexp
	name string
}{
	{regexp.MustCompile(`(?i)\bdirector'?s[\s._-]*cut\b`), "Director's Cut"},
	{regexp.MustCompile(`(?i)\bextended[\s._-]*(?:cut|edition|version)?\b`), "Extended"},
	{regexp.MustCompile(`(?i)\btheatrical(?:[\s._-]*(?:cut|edition|version))?\b`), "Theatrical Cut"},
	{regexp.MustCompile(`(?i)\bfinal[\s._-]*cut\b`), "Final Cut"},
	{regexp.MustCompile(`(?i)\bultimate[\s._-]*(?:cut|edition)\b`), "Ultimate Edition"},
	{regexp.MustCompile(`(?i)\bspecial[\s._-]*edition\b`), "Special Edition"},
	{regexp.MustCompile(`(?i)\bunrated\b`), "Unrated"},
	{regexp.MustCompile(`(?i)\buncut\b`), "Uncut"},
	{regexp.MustCompile(`(?i)\bremastered\b`), "Remastered"},
	{regexp.MustCompile(`(?i)\bimax\b`), "IMAX"},
	{regexp.MustCompile(`(?i)\bcriterion\b`), "Criterion"},
}

// EditionOf names the edition a file is, from its name ("{edition-Director's Cut}" as Plex and
// Jellyfin name them, or words like "Extended"), or "" for none.
func EditionOf(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if m := editionTagRe.FindStringSubmatch(base); m != nil {
		return strings.TrimSpace(m[1])
	}
	// Only after the year, if there is one, so a title like "Extended Family (2020)" isn't one.
	if loc := yearRe.FindStringIndex(base); loc != nil {
		base = base[loc[1]:]
	}
	for _, w := range editionWords {
		if w.re.MatchString(base) {
			return w.name
		}
	}
	return ""
}

// ResolutionClass is "4k", "1080", "720" or "sd" for a video's size ("" if unknown), counting a
// widescreen 1920×800 as 1080.
func ResolutionClass(width, height int) string {
	h := max(height, width*9/16)
	for _, c := range []string{"4k", "1080", "720", "sd"} {
		r := resolutionHeights[c]
		if h >= r[0] && h < r[1] {
			return c
		}
	}
	return ""
}

// videoClass is a file's resolution class from its first video stream.
func videoClass(f *MediaFile) string {
	for _, st := range f.Streams {
		if st.Kind == "video" {
			return ResolutionClass(st.Width, st.Height)
		}
	}
	return ""
}

// redundantCopies reports whether some of these files are copies of each other: the same part,
// edition and resolution. Different editions or resolutions are deliberate versions.
func redundantCopies(files []MediaFile) bool {
	seen := map[string]bool{}
	for i := range files {
		if files[i].Missing {
			continue
		}
		key := strings.Join([]string{string(rune('0' + files[i].Part)), strings.ToLower(EditionOf(files[i].Path)), videoClass(&files[i])}, "|")
		if seen[key] {
			return true
		}
		seen[key] = true
	}
	return false
}
