package store

type Library struct {
	DetailsProvider string   `json:"details_provider"`
	ID              int64    `json:"id"`
	Name            string   `json:"name"`
	Kind            string   `json:"kind"`
	SourceOrdering  string   `json:"source_ordering"`
	Paths           []string `json:"paths"`
	// Make thumbnails for scrubbing along the timeline (off unless turned on).
	Trickplay bool `json:"trickplay"`
	// The fewest films a TMDB collection needs before it's shown; 0 for no automatic collections.
	CollectionMin int `json:"collection_min"`
	// The language titles and descriptions are fetched in (e.g. "fr-FR"); "" for English.
	MetadataLanguage string `json:"metadata_language"`
	// The country whose age ratings are used (e.g. "GB"), as Plex's per-library Country.
	RatingRegion string `json:"rating_region"`
	// Offer Skip Intro (and Skip Recap) and Skip Credits, finding where they are.
	SkipIntros  bool `json:"skip_intros"`
	SkipCredits bool `json:"skip_credits"`
	// How collections show in the grid unless they say: "" their titles, "collection" the
	// collection instead, "both".
	CollectionGrid string `json:"collection_grid"`
}

type Item struct {
	ID              string `json:"id"`
	LibraryID       int64  `json:"library_id"`
	Kind            string `json:"kind"`
	ParentID        string `json:"parent_id,omitempty"`
	Title           string `json:"title"`
	SortTitle       string `json:"sort_title"`
	OriginalTitle   string `json:"original_title,omitempty"`
	Year            int    `json:"year,omitempty"`
	Overview        string `json:"overview,omitempty"`
	RuntimeMin      int    `json:"runtime_min,omitempty"`
	ReleaseDate     string `json:"release_date,omitempty"`
	TMDBID          int64  `json:"tmdb_id,omitempty"`
	TVDBID          int64  `json:"tvdb_id,omitempty"`
	IMDBID          string `json:"imdb_id,omitempty"`
	PosterPath      string `json:"-"`
	BackdropPath    string `json:"-"`
	MetadataState   string `json:"metadata_state"`
	MatchSuppressed bool   `json:"match_suppressed"`
	AddedAt         int64  `json:"added_at"`
	// Fields provider refreshes leave alone, because the user edited or locked them.
	LockedFields []string `json:"locked_fields"`

	Tagline       string  `json:"tagline,omitempty"`
	ContentRating string  `json:"content_rating,omitempty"`
	Studio        string  `json:"studio,omitempty"`
	Rating        float64 `json:"rating,omitempty"`
	// Artwork files next to the media, shown instead of the provider's.
	LocalPoster   string `json:"-"`
	LocalBackdrop string `json:"-"`
	// Clear logo: provider path, and a logo file next to the media.
	LogoPath  string `json:"-"`
	LocalLogo string `json:"-"`
	// A show's own episode numbering, overriding its library's; "" follows the library.
	SourceOrdering string `json:"source_ordering,omitempty"`
	// Legacy per-show preference retained for rollback; new refreshes use Library.DetailsProvider.
	DetailsProvider string `json:"details_provider,omitempty"`
	// The language it was made in (2-letter code, e.g. "ja").
	OriginalLanguage string `json:"original_language,omitempty"`
	// Shows: continuing, ended, cancelled or upcoming, and the last and next air dates.
	ShowStatus string `json:"status,omitempty"`
	LastAired  string `json:"last_aired,omitempty"`
	NextAired  string `json:"next_aired,omitempty"`
	// Other sites' ratings: IMDb (out of 10, with its vote count), Rotten Tomatoes' critics (percent)
	// and Metacritic (out of 100); nil when there's none.
	IMDbRating     float64 `json:"imdb_rating,omitempty"`
	IMDbVotes      int     `json:"imdb_votes,omitempty"`
	RottenTomatoes *int    `json:"rotten_tomatoes,omitempty"`
	Metacritic     *int    `json:"metacritic,omitempty"`
	// A theme tune next to the media (a file path).
	LocalTheme string `json:"-"`
}

// Credit is a person's part in a movie or show.
type Credit struct {
	PersonID     int64  `json:"person_id"`
	TMDBID       int64  `json:"tmdb_id"`
	Name         string `json:"name"`
	ProfilePath  string `json:"-"`
	Role         string `json:"role"` // actor, director, writer, creator
	Character    string `json:"character,omitempty"`
	Order        int    `json:"-"`
	EpisodeCount int    `json:"episode_count,omitempty"`
}

type MediaFile struct {
	ID                 int64  `json:"id"`
	LibraryID          int64  `json:"-"`
	Path               string `json:"path"`
	Size               int64  `json:"size"`
	ModTime            int64  `json:"-"`
	Container          string `json:"container,omitempty"`
	DurationMS         int64  `json:"duration_ms,omitempty"`
	Bitrate            int64  `json:"bitrate,omitempty"`
	ProbeError         string `json:"probe_error,omitempty"`
	Probed             bool   `json:"-"`
	FormatProbeVersion int    `json:"-"`
	Missing            bool   `json:"missing,omitempty"`
	Part               int    `json:"part"`
	// The edition, from the file name (e.g. "Director's Cut"); "" for none.
	Edition string   `json:"edition,omitempty"`
	Streams []Stream `json:"streams"`
	// Chapter marks; filled when probing, and read separately for playback.
	Chapters []Chapter `json:"-"`
}

// Chapter is a chapter mark in a media file.
type Chapter struct {
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
	Title   string `json:"title,omitempty"`
}

type Stream struct {
	Index               int    `json:"index"`
	Kind                string `json:"kind"`
	Codec               string `json:"codec"`
	Profile             string `json:"profile,omitempty"`
	Language            string `json:"language,omitempty"`
	Title               string `json:"title,omitempty"`
	Width               int    `json:"width,omitempty"`
	Height              int    `json:"height,omitempty"`
	Channels            int    `json:"channels,omitempty"`
	IsDefault           bool   `json:"is_default"`
	IsForced            bool   `json:"is_forced"`
	ChannelLayout       string `json:"channel_layout,omitempty"`
	DolbyVision         bool   `json:"dolby_vision,omitempty"`
	DVBLCompatibilityID *int   `json:"dv_bl_signal_compatibility_id,omitempty"`
	DolbyVisionProfile  int    `json:"dolby_vision_profile,omitempty"`
	HDR10Plus           bool   `json:"hdr10_plus,omitempty"`
	HDRProbeStatus      string `json:"hdr_probe_status,omitempty"`
	// Video details used to decide how a file can be played.
	PixFmt         string  `json:"pix_fmt,omitempty"`
	BitDepth       int     `json:"bit_depth,omitempty"`
	ColorTransfer  string  `json:"color_transfer,omitempty"`
	ColorPrimaries string  `json:"color_primaries,omitempty"`
	FrameRate      float64 `json:"frame_rate,omitempty"`
	Bitrate        int64   `json:"bitrate,omitempty"`
	// Display aspect ratio, e.g. 1.78 or 2.39; 0 until read.
	AspectRatio float64 `json:"aspect_ratio,omitempty"`
}

// ItemQuery filters ListItems. Zero values mean "no filter".
type ItemQuery struct {
	VideoCodec, AudioCodec, DynamicRange string
	AudioChannels                        int

	LibraryID int64
	Kind      string
	Search    string
	Sort      string // title (default), added, year
	Reverse   bool   // reverse the selected sort direction
	Limit     int
	Offset    int
	// Jump starts at an A–Z title boundary within the same filtered/access-controlled list.
	// Only for title sort. ListItemsPage returns the effective offset for further paging.
	Jump string
	// What the profile may see: its age limit and libraries.
	Access Access

	// Filters for browsing. Zero values mean no filter.
	Genre      string
	Language   string // original language, 2-letter code
	Label      string
	Decade     int    // e.g. 1990 for 1990–1999
	Resolution string // 4k, 1080, 720 or sd (movies)
	// The profile whose watch state Unwatched and the "watched" sort use.
	UserID    string
	Unwatched bool
	// Only titles Arc couldn't identify, and shows with episodes it couldn't place.
	Unmatched bool
	// Added in the last this many days.
	AddedWithinDays int
	// Runtime in minutes, at least / at most.
	MinRuntime, MaxRuntime int
	MinRating              float64
	// Any of these words in the title, tagline or description (e.g. "christmas", "santa").
	About []string
	// For UserID's watch state: titles not started (nothing watched or part-watched, for a show
	// none of its episodes), titles watched (a show: any episode; an episode: itself), and titles
	// finished before this time (unix seconds) and not played since.
	Unstarted    bool
	Played       bool
	PlayedBefore int64
	// Titles crediting this person (people.id), in PersonRole (actor or director) when it's set.
	PersonID   int64
	PersonRole string
}

// Sorts ListItems understands. "shuffle" is a random order that stays the same all day.
var ItemSorts = map[string]bool{"title": true, "added": true, "year": true, "release": true, "rating": true,
	"imdb": true, "critics": true,
	"runtime": true, "watched": true, "shuffle": true}
