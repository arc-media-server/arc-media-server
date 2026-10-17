package metadata

import "strconv"

// Series is provider metadata for a TV series.
type Series struct {
	TVDBID        int64
	TMDBID        int64
	IMDBID        string
	Title         string
	OriginalTitle string
	Overview      string
	FirstAired    string
	Year          int
	PosterPath    string // TMDB path ("/x.jpg") or full URL
	BackdropPath  string
	// Season posters by aired season number (TMDB paths); TMDB only.
	SeasonPosters map[int]string
	// Genres, cast and so on: from TMDB (TheTVDB's series record leaves these empty).
	Details
}

// Episode is one provider episode. Its identity is (Provider, ID).
type Episode struct {
	Provider   string // "tvdb" or "tmdb"
	ID         int64
	Title      string
	Overview   string
	AirDate    string
	RuntimeMin int
	Still      string // TMDB path or full URL
	// TMDB only: the episode's position in TMDB aired order, needed for TMDB episode lookups.
	AiredSeason int
	AiredNumber int
}

// Ordering is one provider arrangement of a series' episodes into seasons.
type Ordering struct {
	Source      string // tvdb_aired, tvdb_dvd, tvdb_absolute, tvdb_<type>, tmdb_aired, tmdb_group
	SourceRef   string // TMDB episode group id
	Name        string
	Description string
	Seasons     map[int]string
	// Season summaries, where the provider has them (TMDB aired order).
	SeasonOverviews map[int]string
	Entries         []Entry
}

type Entry struct {
	Season  int
	Number  int
	Episode *Episode
}

// Provider returns which provider's episodes an ordering arranges.
func (o *Ordering) Provider() string {
	if len(o.Source) >= 4 {
		return o.Source[:4]
	}
	return ""
}

func seasonName(n int) string {
	if n == 0 {
		return "Specials"
	}
	return "Season " + strconv.Itoa(n)
}
