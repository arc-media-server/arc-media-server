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
		{"episode.nfo", `<episodedetails><title>Local text</title><plot>Not imported</plot><uniqueid type="tvdb">12345</uniqueid></episodedetails>`, NFO{TVDBID: 12345}},
		{"plot-only.nfo", `<tvshow><title>Local show</title><plot>Local synopsis</plot><actor><name>Local actor</name></actor></tvshow>`, NFO{}},
		{"malformed.nfo", `<movie><uniqueid type="tmdb">949`, NFO{}},
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

func TestClassifyExtra(t *testing.T) {
	root := filepath.FromSlash("/media/Movies")
	p := func(s string) string { return filepath.Join(root, filepath.FromSlash(s)) }
	tests := []struct {
		path                        string
		ok                          bool
		kind, ownerDir, stem, title string
	}{
		{"Heat (1995)/Extras/Making of.mp4", true, "other", "Heat (1995)", "", "Making of"},
		{"Heat (1995)/Behind The Scenes/On set.mkv", true, "behind_the_scenes", "Heat (1995)", "", "On set"},
		{"Heat (1995)/Heat (1995)-trailer.mkv", true, "trailer", "Heat (1995)", "Heat (1995)", "Trailer"},
		{"Heat (1995)/Heat (1995) - Final Trailer-trailer.mkv", true, "trailer", "Heat (1995)", "Heat (1995) - Final Trailer", "Trailer"},
		{"Heat (1995)/Heat (1995).mkv", false, "", "", "", ""},
	}
	for _, tt := range tests {
		ex, ok := classifyExtra(root, p(tt.path))
		if ok != tt.ok {
			t.Errorf("%s: extra = %v, want %v", tt.path, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if ex.kind != tt.kind || ex.ownerDir != p(tt.ownerDir) || ex.ownerStem != tt.stem {
			t.Errorf("%s: got %+v", tt.path, ex)
		}
		if got := extraTitle(ex); got != tt.title {
			t.Errorf("%s: title %q, want %q", tt.path, got, tt.title)
		}
	}
	if !isSample("sample.mkv") || !isSample("movie-sample.mkv") || isSample("Sampler (2020).mkv") {
		t.Error("isSample")
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

func TestSeasonPosters(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"season01-poster.jpg", "Season2-Poster.PNG", "season-specials-poster.webp", "season03-fanart.jpg", "poster.jpg"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := seriesArt(dir).Seasons
	if len(got) != 3 || filepath.Base(got[1]) != "season01-poster.jpg" || filepath.Base(got[2]) != "Season2-Poster.PNG" ||
		filepath.Base(got[0]) != "season-specials-poster.webp" {
		t.Errorf("season posters: %v", got)
	}
}
