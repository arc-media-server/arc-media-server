package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadNFO(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	tests := []struct {
		name, body string
		want       NFO
	}{
		{"kodi.nfo", `<?xml version="1.0" encoding="UTF-8"?>
<movie><title>Heat</title><uniqueid type="imdb" default="true">tt0113277</uniqueid><uniqueid type="tmdb">949</uniqueid></movie>`,
			NFO{TMDBID: 949, IMDBID: "tt0113277"}},
		{"radarr.nfo", `<movie><title>Heat</title><id>tt0113277</id><tmdbid>949</tmdbid></movie>`,
			NFO{TMDBID: 949, IMDBID: "tt0113277"}},
		{"sonarr.nfo", `<tvshow><title>Firefly</title><uniqueid type="tvdb">78874</uniqueid><tvdbid>78874</tvdbid></tvshow>`,
			NFO{TVDBID: 78874}},
		{"link.nfo", "https://www.imdb.com/title/tt0133093/\n", NFO{IMDBID: "tt0133093"}},
		{"empty.nfo", "<movie><title>Nothing</title></movie>", NFO{}},
	}
	for _, tt := range tests {
		if got := readNFO(write(tt.name, tt.body)); got != tt.want {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, tt.want)
		}
	}
	if got := readNFO(filepath.Join(dir, "missing.nfo")); !got.empty() {
		t.Errorf("missing file: %+v", got)
	}
}

func TestMovieArt(t *testing.T) {
	dir := t.TempDir()
	touch := func(name string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	touch("Heat (1995).mkv")
	touch("poster.jpg")
	touch("Heat (1995)-fanart.png")
	movie := filepath.Join(dir, "Heat (1995).mkv")

	if art := movieArt(movie, true); art.Poster != filepath.Join(dir, "poster.jpg") || art.Backdrop != filepath.Join(dir, "Heat (1995)-fanart.png") {
		t.Errorf("own folder: %+v", art)
	}
	// In a shared folder, poster.jpg isn't this movie's.
	if art := movieArt(movie, false); art.Poster != "" || art.Backdrop == "" {
		t.Errorf("shared folder: %+v", art)
	}
}
