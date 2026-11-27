package store

import "testing"

func TestEditionOf(t *testing.T) {
	for path, want := range map[string]string{
		"/m/Blade Runner (1982) {edition-Final Cut}.mkv":           "Final Cut",
		"/m/Blade.Runner.1982.Directors.Cut.1080p.BluRay.x264.mkv": "Director's Cut",
		"/m/Aliens (1986) - Director's Cut.mkv":                    "Director's Cut",
		"/m/LOTR (2001) Extended Edition.mkv":                      "Extended",
		"/m/Movie.2010.EXTENDED.2160p.mkv":                         "Extended",
		"/m/Movie (2010) Theatrical.mkv":                           "Theatrical Cut",
		"/m/Movie (2010) - IMAX.mkv":                               "IMAX",
		"/m/The Matrix (1999).mp4":                                 "",
		"/m/Extended Family (2020).mkv":                            "",         // words before the year are the title
		"/m/Extended Family.mkv":                                   "Extended", // without a year, a known limitation
		"/Extended/The Matrix (1999).mp4":                          "",         // only the file name counts
	} {
		if got := EditionOf(path); got != want {
			t.Errorf("EditionOf(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestRedundantCopies(t *testing.T) {
	file := func(path string, h int) MediaFile {
		return MediaFile{Path: path, Streams: []Stream{{Kind: "video", Width: h * 16 / 9, Height: h}}}
	}
	if !redundantCopies([]MediaFile{file("/a/M (1999).mkv", 1080), file("/b/M (1999) copy.mkv", 1080)}) {
		t.Error("two 1080p copies are redundant")
	}
	if redundantCopies([]MediaFile{file("/a/M (1999).mkv", 2160), file("/a/M (1999) - 1080p.mkv", 1080)}) {
		t.Error("4K and 1080p are versions")
	}
	if redundantCopies([]MediaFile{file("/a/M (1999).mkv", 1080), file("/a/M (1999) {edition-Director's Cut}.mkv", 1080)}) {
		t.Error("editions aren't copies")
	}
	widescreen := MediaFile{Path: "/a/W.mkv", Streams: []Stream{{Kind: "video", Width: 1920, Height: 800}}}
	if c := videoClass(&widescreen); c != "1080" {
		t.Errorf("1920x800 = %q", c)
	}
}
