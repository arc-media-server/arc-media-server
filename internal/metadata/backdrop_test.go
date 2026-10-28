package metadata

import "testing"

func TestBestBackdrop(t *testing.T) {
	img := func(path string, width int, lang string) tmdbImage {
		return tmdbImage{FilePath: path, Width: width, Height: width * 9 / 16, Language: lang}
	}
	tests := []struct {
		name string
		def  string
		list []tmdbImage
		want string
	}{
		{"TMDB's full-HD choice is kept", "/a.jpg",
			[]tmdbImage{img("/a.jpg", 1920, ""), img("/b.jpg", 3840, "")}, "/a.jpg"},
		{"TMDB's choice of unknown size is kept", "/a.jpg",
			[]tmdbImage{img("/b.jpg", 3840, "")}, "/a.jpg"},
		{"a 720p choice gives way to the most liked full-HD one", "/a.jpg",
			[]tmdbImage{img("/a.jpg", 1280, ""), img("/b.jpg", 1280, ""), img("/c.jpg", 1920, ""), img("/d.jpg", 3840, "")}, "/c.jpg"},
		{"textless stays textless", "/a.jpg",
			[]tmdbImage{img("/a.jpg", 1280, ""), img("/en.jpg", 3840, "en"), img("/c.jpg", 1920, "")}, "/c.jpg"},
		{"with English text, English text is preferred", "/a.jpg",
			[]tmdbImage{img("/a.jpg", 1280, "en"), img("/c.jpg", 1920, ""), img("/en.jpg", 1920, "en")}, "/en.jpg"},
		{"otherwise a textless one before another language", "/a.jpg",
			[]tmdbImage{img("/a.jpg", 1280, "en"), img("/fr.jpg", 1920, "fr"), img("/c.jpg", 1920, "")}, "/c.jpg"},
		{"no full-HD one: the widest bigger one", "/a.jpg",
			[]tmdbImage{img("/a.jpg", 1000, ""), img("/b.jpg", 1280, ""), img("/c.jpg", 1600, ""), img("/d.jpg", 1600, "")}, "/c.jpg"},
		{"no full-HD one and nothing bigger: TMDB's choice", "/a.jpg",
			[]tmdbImage{img("/a.jpg", 1280, ""), img("/b.jpg", 1280, "")}, "/a.jpg"},
		{"no choice from TMDB: the most liked full-HD one", "",
			[]tmdbImage{img("/a.jpg", 1280, ""), img("/en.jpg", 1920, "en"), img("/b.jpg", 1920, "")}, "/b.jpg"},
		{"nothing at all", "", nil, ""},
	}
	for _, tt := range tests {
		if got := bestBackdrop(tt.def, tt.list); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
