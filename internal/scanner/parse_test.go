package scanner

import (
	"path/filepath"
	"testing"
)

func TestParseMovie(t *testing.T) {
	tests := []struct {
		path string
		want MovieName
	}{
		{"Movies/The Matrix (1999).mkv", MovieName{Title: "The Matrix", Year: 1999}},
		{"Movies/The Matrix (1999)/The Matrix (1999).mkv", MovieName{Title: "The Matrix", Year: 1999}},
		{"Movies/The Matrix (1999)/movie.mkv", MovieName{Title: "The Matrix", Year: 1999, FromFolder: true}},
		{"Movies/The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv", MovieName{Title: "The Matrix", Year: 1999}},
		{"Movies/Blade.Runner.2049.2017.2160p.UHD.mkv", MovieName{Title: "Blade Runner 2049", Year: 2017}},
		{"Movies/Blade Runner 2049.mkv", MovieName{Title: "Blade Runner 2049"}},
		{"Movies/1917.2019.1080p.mkv", MovieName{Title: "1917", Year: 2019}},
		{"Movies/2001 A Space Odyssey (1968).mkv", MovieName{Title: "2001 A Space Odyssey", Year: 1968}},
		{"Movies/Spider-Man (2002).mp4", MovieName{Title: "Spider-Man", Year: 2002}},
		{"Movies/Heat (1995) {tmdb-949}.mkv", MovieName{Title: "Heat", Year: 1995, TMDBID: 949}},
		{"Movies/Heat (1995) [imdbid-tt0113277]/Heat.mkv", MovieName{Title: "Heat", Year: 1995, IMDBID: "tt0113277", FromFolder: true}},
		{"Movies/Alien (1979) {edition-Director's Cut}.mkv", MovieName{Title: "Alien", Year: 1979}},
		{"Movies/Kill Bill (2003) - cd2.avi", MovieName{Title: "Kill Bill", Year: 2003, Part: 2}},
		{"Movies/[Group] Akira (1988) [1080p].mkv", MovieName{Title: "Akira", Year: 1988}},
		{"Movies/Amelie - 2001.mkv", MovieName{Title: "Amelie", Year: 2001}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ParseMovie(filepath.FromSlash(tt.path))
			if got != tt.want {
				t.Errorf("ParseMovie(%q) = %+v, want %+v", tt.path, got, tt.want)
			}
		})
	}
}
