package store

type Library struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	SourceOrdering string   `json:"source_ordering"`
	Paths          []string `json:"paths"`
}

type Item struct {
	ID            string `json:"id"`
	LibraryID     int64  `json:"library_id"`
	Kind          string `json:"kind"`
	ParentID      string `json:"parent_id,omitempty"`
	Title         string `json:"title"`
	SortTitle     string `json:"sort_title"`
	OriginalTitle string `json:"original_title,omitempty"`
	Year          int    `json:"year,omitempty"`
	Overview      string `json:"overview,omitempty"`
	RuntimeMin    int    `json:"runtime_min,omitempty"`
	ReleaseDate   string `json:"release_date,omitempty"`
	TMDBID        int64  `json:"tmdb_id,omitempty"`
	TVDBID        int64  `json:"tvdb_id,omitempty"`
	IMDBID        string `json:"imdb_id,omitempty"`
	PosterPath    string `json:"-"`
	BackdropPath  string `json:"-"`
	MetadataState string `json:"metadata_state"`
	AddedAt       int64  `json:"added_at"`
	// Fields provider refreshes leave alone, because the user edited or locked them.
	LockedFields []string `json:"locked_fields"`

	Tagline       string  `json:"tagline,omitempty"`
	ContentRating string  `json:"content_rating,omitempty"`
	Studio        string  `json:"studio,omitempty"`
	Rating        float64 `json:"rating,omitempty"`
}

// Credit is a person's part in a movie or show.
type Credit struct {
	PersonID    int64  `json:"person_id"`
	TMDBID      int64  `json:"tmdb_id"`
	Name        string `json:"name"`
	ProfilePath string `json:"-"`
	Role        string `json:"role"` // actor, director, writer, creator
	Character   string `json:"character,omitempty"`
	Order       int    `json:"-"`
}

type MediaFile struct {
	ID         int64    `json:"id"`
	LibraryID  int64    `json:"-"`
	Path       string   `json:"path"`
	Size       int64    `json:"size"`
	ModTime    int64    `json:"-"`
	Container  string   `json:"container,omitempty"`
	DurationMS int64    `json:"duration_ms,omitempty"`
	Bitrate    int64    `json:"bitrate,omitempty"`
	ProbeError string   `json:"probe_error,omitempty"`
	Probed     bool     `json:"-"`
	Missing    bool     `json:"missing,omitempty"`
	Part       int      `json:"part"`
	Streams    []Stream `json:"streams"`
}

type Stream struct {
	Index     int    `json:"index"`
	Kind      string `json:"kind"`
	Codec     string `json:"codec"`
	Profile   string `json:"profile,omitempty"`
	Language  string `json:"language,omitempty"`
	Title     string `json:"title,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Channels  int    `json:"channels,omitempty"`
	IsDefault bool   `json:"is_default"`
	IsForced  bool   `json:"is_forced"`
}

// ItemQuery filters ListItems. Zero values mean "no filter".
type ItemQuery struct {
	LibraryID int64
	Kind      string
	Search    string
	Sort      string // title (default), added, year
	Limit     int
	Offset    int
	// Only items rated for this age or younger (0 = no limit; unrated items are left out).
	MaxAge int
}
