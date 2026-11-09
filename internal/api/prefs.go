package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"ams/internal/store"
)

// Each profile's own preferences: how its home page is laid out and whether unwatched episodes are
// kept spoiler-free.

type prefs struct {
	// Large featured banner on Home; absent keeps the existing visible default.
	ShowHomeHero *bool `json:"show_home_hero,omitempty"`
	// Library navigation order only; never changes library visibility or Home rows.
	LibraryOrder []int64 `json:"library_order,omitempty"`
	// Where the Watchlist sits among the libraries in navigation (as a pinned source in Plex):
	// how many libraries come before it. Absent or 0 puts it first; its Home row follows it.
	WatchlistPosition int `json:"watchlist_position,omitempty"`
	// Percentage for future progress reports; zero keeps the existing 90% default.
	WatchedPercent int `json:"watched_percent,omitempty"`
	// Blur stills and hide descriptions of episodes the profile hasn't started.
	HideSpoilers bool `json:"hide_spoilers"`
	// "dark", "light", or "" to follow the device.
	Theme string `json:"theme"`
	// The subtitle language last chosen (e.g. "eng"), used to turn subtitles on by themselves;
	// "" means off unless there are forced subtitles.
	SubtitleLanguage string `json:"subtitle_language"`
	// The audio language last chosen (e.g. "jpn"), played by default when a file has it; "" for
	// each file's own default.
	AudioLanguage string `json:"audio_language"`
	// Show Up Next as its own row; otherwise the next episodes are part of Continue Watching, as in Plex.
	SplitUpNext bool `json:"split_up_next"`
	// Play the next episode after a countdown when one ends; on unless turned off, as in Plex.
	AutoPlay *bool `json:"auto_play,omitempty"`
	// Play a title's theme tune quietly on its page; on unless turned off, as in Plex.
	ThemeMusic *bool `json:"theme_music,omitempty"`
	// Skip intros and recaps without asking (off unless turned on).
	AutoSkip bool `json:"auto_skip"`
	// Internet streaming quality, as Plex's: the most a stream away from home may use (kbit/s, one
	// of the player's quality limits), or 0 for Maximum. Never applies on the home network.
	RemoteQualityKbps int `json:"remote_quality_kbps,omitempty"`
	// When subtitles turn on by themselves, as Plex's subtitle mode: "" in the language last chosen,
	// "manual" only when picked (forced ones still show), "foreign" when the audio isn't in the
	// subtitle language, "always" whenever there are some in it.
	SubtitleMode string `json:"subtitle_mode,omitempty"`
	// How subtitles look: size (small, medium, large, huge), colour (white, yellow), background
	// (box, solid, none) and position (bottom, top); "" for each default.
	SubtitleStyle subtitleStyle `json:"subtitle_style"`
	// The home rows in order. Empty means the default layout; rows the app knows about that
	// aren't listed (e.g. a new library's) are added in their usual place.
	Home []homeRow `json:"home"`
	// Set in responses when Home is the admin's default home (the profile hasn't arranged its own).
	HomeIsDefault bool `json:"home_is_default,omitempty"`
}

type subtitleStyle struct {
	Size       string `json:"size,omitempty"`
	Color      string `json:"color,omitempty"`
	Background string `json:"background,omitempty"`
	Position   string `json:"position,omitempty"`
}

// homeRow is a built-in row ("continue", "next-up", "recent:<library id>", "seasonal"), a custom
// one ("custom:<anything>") with a title and a filter, or a collection ("collection:<id>") or
// playlist ("playlist:<id>").
type homeRow struct {
	ID     string     `json:"id"`
	Hidden bool       `json:"hidden,omitempty"`
	Title  string     `json:"title,omitempty"`
	Filter *rowFilter `json:"filter,omitempty"`
	// "poster" or "thumb" (landscape thumbnails); "" for the row's usual look.
	Style string `json:"style,omitempty"`
	// A TV library's Recently Added: one card per show, instead of grouping new episodes by season.
	ShowsOnly bool `json:"shows_only,omitempty"`
	// For episodes and seasons on a poster row: the show's poster ("series") or the season's
	// ("season"); "" for the row's usual choice.
	Posters string `json:"posters,omitempty"`
}

// rowFilter is the same set of filters as browsing a library (GET /api/items).
type rowFilter struct {
	VideoCodec    string `json:"video_codec,omitempty"`
	AudioCodec    string `json:"audio_codec,omitempty"`
	AudioChannels int    `json:"audio_channels,omitempty"`
	DynamicRange  string `json:"dynamic_range,omitempty"`
	WatchState    string `json:"watch_state,omitempty"`
	PersonID      int64  `json:"person_id,omitempty"`
	PersonRole    string `json:"person_role,omitempty"`

	LibraryID   int64   `json:"library_id,omitempty"`
	Genre       string  `json:"genre,omitempty"`
	Language    string  `json:"language,omitempty"`
	Label       string  `json:"label,omitempty"`
	Decade      int     `json:"decade,omitempty"`
	Resolution  string  `json:"resolution,omitempty"`
	Unwatched   bool    `json:"unwatched,omitempty"`
	AddedWithin int     `json:"added_within,omitempty"`
	MinRuntime  int     `json:"min_runtime,omitempty"`
	MaxRuntime  int     `json:"max_runtime,omitempty"`
	MinRating   float64 `json:"min_rating,omitempty"`
	Sort        string  `json:"sort,omitempty"`
}

func (p *prefs) validate() error {
	if p.WatchedPercent != 0 && (p.WatchedPercent < 50 || p.WatchedPercent > 100) {
		return fmt.Errorf("watched_percent must be 50–100, or 0 for the 90%% default")
	}
	if len(p.LibraryOrder) > 500 {
		return fmt.Errorf("library_order is limited to 500 libraries")
	}
	if p.WatchlistPosition < 0 || p.WatchlistPosition > 500 {
		return fmt.Errorf("watchlist_position is how many libraries come before the Watchlist (0–500)")
	}
	seenLibraries := map[int64]bool{}
	for _, id := range p.LibraryOrder {
		if id <= 0 || seenLibraries[id] {
			return fmt.Errorf("library_order must contain distinct positive IDs")
		}
		seenLibraries[id] = true
	}
	if p.Theme != "" && p.Theme != "dark" && p.Theme != "light" {
		return fmt.Errorf(`theme is "dark", "light" or "" (follow the device)`)
	}
	if len(p.SubtitleLanguage) > 8 {
		return fmt.Errorf("subtitle_language is a language code like \"eng\"")
	}
	if len(p.AudioLanguage) > 8 {
		return fmt.Errorf("audio_language is a language code like \"jpn\"")
	}
	if p.RemoteQualityKbps != 0 && !slices.Contains(qualityLimits, p.RemoteQualityKbps) {
		return fmt.Errorf("remote_quality_kbps is 0 (Maximum) or one of %v", qualityLimits)
	}
	if !slices.Contains([]string{"", "manual", "foreign", "always"}, p.SubtitleMode) {
		return fmt.Errorf(`subtitle_mode is "", "manual", "foreign" or "always"`)
	}
	st := p.SubtitleStyle
	if !slices.Contains([]string{"", "small", "medium", "large", "huge"}, st.Size) ||
		!slices.Contains([]string{"", "white", "yellow"}, st.Color) ||
		!slices.Contains([]string{"", "box", "solid", "none"}, st.Background) ||
		!slices.Contains([]string{"", "bottom", "top"}, st.Position) {
		return fmt.Errorf("subtitle_style: size small/medium/large/huge, color white/yellow, background box/solid/none, position bottom/top")
	}
	if len(p.Home) > 60 {
		return fmt.Errorf("a home page can have up to 60 rows")
	}
	seen := map[string]bool{}
	for i := range p.Home {
		r := &p.Home[i]
		r.ID = strings.TrimSpace(r.ID)
		r.Title = strings.TrimSpace(r.Title)
		if r.ID == "" || len(r.ID) > 64 || seen[r.ID] {
			return fmt.Errorf("each row needs its own id")
		}
		seen[r.ID] = true
		if r.Posters != "" && r.Posters != "series" && r.Posters != "season" {
			return fmt.Errorf(`a row's posters are "series" or "season"`)
		}
		if r.Style != "" && r.Style != "poster" && r.Style != "thumb" && r.Style != "arc" {
			return fmt.Errorf(`a row's style is "poster", "thumb" or "arc"`)
		}
		if strings.HasPrefix(r.ID, "collection:") || strings.HasPrefix(r.ID, "playlist:") {
			// A collection or playlist: its name is kept for Edit Home; the row shows the current one.
			r.Filter = nil
			if len([]rune(r.Title)) > 120 {
				return fmt.Errorf("a row's name can be up to 120 characters")
			}
			continue
		}
		if !strings.HasPrefix(r.ID, "custom:") {
			r.Title, r.Filter = "", nil
			continue
		}
		if r.Title == "" || len([]rune(r.Title)) > 60 {
			return fmt.Errorf("give the row a name of up to 60 characters")
		}
		f := r.Filter
		if f == nil || f.LibraryID == 0 {
			return fmt.Errorf("choose which library %q shows", r.Title)
		}
		if err := f.validateExtended(); err != nil {
			return err
		}
		if f.Sort != "" && !store.ItemSorts[f.Sort] {
			return fmt.Errorf("unknown sort %q", f.Sort)
		}
		if f.AddedWithin < 0 || f.MinRuntime < 0 || f.MaxRuntime < 0 || f.MinRating < 0 || f.MinRating > 10 {
			return fmt.Errorf("%q has a filter out of range", r.Title)
		}
	}
	return nil
}

func (s *Server) loadPrefs(r *http.Request) (prefs, error) {
	return s.loadPrefsFor(r.Context(), currentUserID(r))
}

// loadPrefsFor reads a profile's preferences as stored (an empty home means the default layout).
func (s *Server) loadPrefsFor(ctx context.Context, userID string) (prefs, error) {
	raw, err := s.store.UserPrefs(ctx, userID)
	if err != nil {
		return prefs{Home: []homeRow{}}, err
	}
	return s.prefsFromJSON(raw, userID), nil
}

func (s *Server) prefsFromJSON(raw, userID string) prefs {
	p := prefs{Home: []homeRow{}}
	// Preserve the existing fallback for unreadable stored preferences.
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		s.log.Warn("ignoring unreadable profile preferences", "profile", userID, "err", err)
		p = prefs{}
	}
	if p.Home == nil {
		p.Home = []homeRow{}
	}
	return p
}

const defaultHomeKey = "default_home"

// defaultHome is the home layout the admin set for profiles that haven't arranged their own;
// empty for Arc's own.
func (s *Server) defaultHome(ctx context.Context) []homeRow {
	rows := []homeRow{}
	if raw, err := s.store.Setting(ctx, defaultHomeKey); err == nil && raw != "" {
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			s.log.Warn("ignoring an unreadable default home", "err", err)
			return []homeRow{}
		}
	}
	return rows
}

// withDefaultHome fills in the admin's default home for a profile that hasn't arranged its own,
// for showing (it's never saved into the profile's preferences).
func (s *Server) withDefaultHome(ctx context.Context, p prefs) prefs {
	if len(p.Home) == 0 {
		if def := s.defaultHome(ctx); len(def) > 0 {
			p.Home, p.HomeIsDefault = def, true
		}
	}
	return p
}

// getPrefs: GET /api/me/prefs is the current profile's preferences, including library_order
// (navigation only, never access) and watched_percent (50–100; absent/0 keeps 90%).
func (s *Server) getPrefs(w http.ResponseWriter, r *http.Request) {
	s.writePrefs(w, r, currentUserID(r))
}

// getProfilePrefs: GET /api/profiles/{id}/prefs is a profile's preferences (admin), e.g. to
// arrange its home page.
func (s *Server) getProfilePrefs(w http.ResponseWriter, r *http.Request) {
	if u, ok := s.profileFromPath(w, r); ok {
		s.writePrefs(w, r, u.ID)
	}
}

func (s *Server) writePrefs(w http.ResponseWriter, r *http.Request, userID string) {
	p, err := s.loadPrefsFor(r.Context(), userID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.withDefaultHome(r.Context(), p))
}

func (s *Server) profileFromPath(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u, err := s.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	if u == nil {
		writeError(w, http.StatusNotFound, "no such profile")
		return nil, false
	}
	return u, true
}

// updatePrefs: PATCH /api/me/prefs changes only the fields sent, e.g. {"library_order": [2,1]},
// {"watched_percent": 95}, {"hide_spoilers": true} or {"home": [...]}, and returns them all.
// An empty library_order resets navigation order; watched_percent 0 resets completion to 90%.
// remote_quality_kbps is the profile's internet streaming quality: 0 for Maximum, or one of 20000,
// 12000, 8000, 4000, 3000, 2000, 1500 or 720 (kbit/s) to limit streams away from home (the lower of
// it and the server's remote limit applies; never on the home network).
func (s *Server) updatePrefs(w http.ResponseWriter, r *http.Request) {
	s.patchPrefs(w, r, currentUserID(r))
}

// updateProfilePrefs: PATCH /api/profiles/{id}/prefs changes another profile's preferences
// (admin), e.g. arranging its home page.
func (s *Server) updateProfilePrefs(w http.ResponseWriter, r *http.Request) {
	if u, ok := s.profileFromPath(w, r); ok {
		s.patchPrefs(w, r, u.ID)
	}
}

// getDefaultHome: GET /api/home/default is the home layout for profiles that haven't arranged
// their own ([] for Arc's own).
func (s *Server) getDefaultHome(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"home": s.defaultHome(r.Context())})
}

// setDefaultHome: PUT /api/home/default {"home": [...]} sets it ([] goes back to Arc's own).
func (s *Server) setDefaultHome(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Home []homeRow `json:"home"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&body); err != nil || body.Home == nil {
		writeError(w, http.StatusBadRequest, `send {"home": [rows]}`)
		return
	}
	p := prefs{Home: body.Home}
	if err := p.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	raw, _ := json.Marshal(p.Home)
	if len(p.Home) == 0 {
		raw = nil
	}
	if err := s.store.SetSetting(r.Context(), defaultHomeKey, string(raw)); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"home": p.Home})
}

func (s *Server) patchPrefs(w http.ResponseWriter, r *http.Request, userID string) {
	var body struct {
		ShowHomeHero     *bool          `json:"show_home_hero"`
		HideSpoilers     *bool          `json:"hide_spoilers"`
		LibraryOrder     *[]int64       `json:"library_order"`
		WatchlistAt      *int           `json:"watchlist_position"`
		WatchedPercent   *int           `json:"watched_percent"`
		Theme            *string        `json:"theme"`
		SubtitleLanguage *string        `json:"subtitle_language"`
		AudioLanguage    *string        `json:"audio_language"`
		SplitUpNext      *bool          `json:"split_up_next"`
		AutoPlay         *bool          `json:"auto_play"`
		ThemeMusic       *bool          `json:"theme_music"`
		AutoSkip         *bool          `json:"auto_skip"`
		RemoteQuality    *int           `json:"remote_quality_kbps"`
		SubtitleMode     *string        `json:"subtitle_mode"`
		SubtitleStyle    *subtitleStyle `json:"subtitle_style"`
		Home             []homeRow      `json:"home"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "send the preferences to change as JSON")
		return
	}
	var p prefs
	err := s.store.UpdateUserPrefs(r.Context(), userID, func(raw string) (string, error) {
		p = s.prefsFromJSON(raw, userID)
		if body.ShowHomeHero != nil {
			p.ShowHomeHero = body.ShowHomeHero
		}
		if body.HideSpoilers != nil {
			p.HideSpoilers = *body.HideSpoilers
		}
		if body.LibraryOrder != nil {
			p.LibraryOrder = *body.LibraryOrder
		}
		if body.WatchlistAt != nil {
			p.WatchlistPosition = *body.WatchlistAt
		}
		if body.WatchedPercent != nil {
			p.WatchedPercent = *body.WatchedPercent
		}
		if body.Theme != nil {
			p.Theme = *body.Theme
		}
		if body.SubtitleLanguage != nil {
			p.SubtitleLanguage = *body.SubtitleLanguage
		}
		if body.AudioLanguage != nil {
			p.AudioLanguage = *body.AudioLanguage
		}
		if body.SplitUpNext != nil {
			p.SplitUpNext = *body.SplitUpNext
		}
		if body.AutoSkip != nil {
			p.AutoSkip = *body.AutoSkip
		}
		if body.RemoteQuality != nil {
			p.RemoteQualityKbps = *body.RemoteQuality
		}
		if body.SubtitleMode != nil {
			p.SubtitleMode = *body.SubtitleMode
		}
		if body.SubtitleStyle != nil {
			p.SubtitleStyle = *body.SubtitleStyle
		}
		if body.ThemeMusic != nil {
			p.ThemeMusic = body.ThemeMusic
		}
		if body.AutoPlay != nil {
			p.AutoPlay = body.AutoPlay
		}
		if body.Home != nil {
			p.Home = body.Home
		}
		if err := p.validate(); err != nil {
			return "", errMsg(err.Error())
		}
		p.HomeIsDefault = false
		encoded, err := json.Marshal(p)
		return string(encoded), err
	})
	if err != nil {
		var invalid errMsg
		switch {
		case errors.As(err, &invalid):
			writeError(w, http.StatusBadRequest, invalid.Error())
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusNotFound, "profile not found")
		default:
			s.fail(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, s.withDefaultHome(r.Context(), p))
}
