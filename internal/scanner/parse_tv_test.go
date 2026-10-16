package scanner

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseEpisodePath(t *testing.T) {
	root := filepath.FromSlash("/tv")
	tests := []struct {
		path      string
		series    string
		year      int
		tvdb      int64
		season    int
		episodes  []int
		absolute  bool
		airDate   string
	}{
		{"Firefly (2002)/Season 01/Firefly - S01E05 - Safe.mkv", "Firefly", 2002, 0, 1, []int{5}, false, ""},
		{"Firefly (2002) {tvdb-78874}/Season 1/Firefly.S01E11.Serenity.1080p.mkv", "Firefly", 2002, 78874, 1, []int{11}, false, ""},
		{"Doctor Who (2005)/Specials/Doctor Who - S00E03.mkv", "Doctor Who", 2005, 0, 0, []int{3}, false, ""},
		{"Lost/Season 01/Lost - S01E01-E02 - Pilot.mkv", "Lost", 0, 0, 1, []int{1, 2}, false, ""},
		{"Lost/Season 01/Lost - S01E01E02.mkv", "Lost", 0, 0, 1, []int{1, 2}, false, ""},
		{"Lost/Season 01/Lost - S01E01-03.mkv", "Lost", 0, 0, 1, []int{1, 2, 3}, false, ""},
		{"Lost/Season 01/Lost - S01E01 - 720p.mkv", "Lost", 0, 0, 1, []int{1}, false, ""},
		{"Lost/Season 01/Lost - S01E01-720p.mkv", "Lost", 0, 0, 1, []int{1}, false, ""},
		{"Lost/Season 01/Lost.S01E04.Episode.Title.mkv", "Lost", 0, 0, 1, []int{4}, false, ""},
		{"Frasier/Frasier 3x05.avi", "Frasier", 0, 0, 3, []int{5}, false, ""},
		{"Frasier/Frasier 3x05-06.avi", "Frasier", 0, 0, 3, []int{5, 6}, false, ""},
		{"Frasier/Frasier 1920x1080 3x07.avi", "Frasier", 0, 0, 3, []int{7}, false, ""},
		{"The Daily Show/The Daily Show 2024-03-05.mkv", "The Daily Show", 0, 0, -1, nil, false, "2024-03-05"},
		{"Planet Earth/Season 2/Episode 3.mkv", "Planet Earth", 0, 0, 2, []int{3}, false, ""},
		{"One Piece/[SubGroup] One Piece - 1071 [1080p].mkv", "One Piece", 0, 0, -1, []int{1071}, true, ""},
		{"One Piece/One Piece - 012v2.mkv", "One Piece", 0, 0, -1, []int{12}, true, ""},
		{"Firefly - S01E02.mkv", "Firefly", 0, 0, 1, []int{2}, false, ""},
		{"Heroes (2006)/Season 2007/Heroes - S2007E01.mkv", "Heroes", 2006, 0, 2007, []int{1}, false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			_, s, ep, ok := ParseEpisodePath(root, filepath.Join(root, filepath.FromSlash(tt.path)))
			if !ok {
				t.Fatalf("not parsed as an episode")
			}
			if s.Title != tt.series || s.Year != tt.year || s.TVDBID != tt.tvdb {
				t.Errorf("series = %q (%d) tvdb %d, want %q (%d) tvdb %d", s.Title, s.Year, s.TVDBID, tt.series, tt.year, tt.tvdb)
			}
			if ep.Season != tt.season || !reflect.DeepEqual(ep.Episodes, tt.episodes) ||
				ep.Absolute != tt.absolute || ep.AirDate != tt.airDate {
				t.Errorf("episode = %+v, want season %d episodes %v absolute %v date %q",
					ep, tt.season, tt.episodes, tt.absolute, tt.airDate)
			}
		})
	}
}

func TestParseEpisodePathRejects(t *testing.T) {
	root := filepath.FromSlash("/tv")
	for _, p := range []string{"Firefly (2002)/poster.mkv", "Firefly (2002)/Firefly.mkv"} {
		if _, _, ep, ok := ParseEpisodePath(root, filepath.Join(root, filepath.FromSlash(p))); ok {
			t.Errorf("%s: parsed as %+v, want not an episode", p, ep)
		}
	}
}
