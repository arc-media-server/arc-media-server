package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"ams/internal/store"
)

// Each profile's own preferences: how its home page is laid out and whether unwatched episodes are
// kept spoiler-free.

type prefs struct {
	// Blur stills and hide descriptions of episodes the profile hasn't started.
	HideSpoilers bool `json:"hide_spoilers"`
	// "dark", "light", or "" to follow the device.
	Theme string `json:"theme"`
	// The subtitle language last chosen (e.g. "eng"), used to turn subtitles on by themselves;
	// "" means off unless there are forced subtitles.
	SubtitleLanguage string `json:"subtitle_language"`
	// Show Up Next as its own row; otherwise the next episodes are part of Continue Watching, as in Plex.
	SplitUpNext bool `json:"split_up_next"`
	// The home rows in order. Empty means the default layout; rows the app knows about that
	// aren't listed (e.g. a new library's) are added in their usual place.
	Home []homeRow `json:"home"`
}

// homeRow is a built-in row ("continue", "next-up", "recent:<library id>", "seasonal") or a
// custom one ("custom:<anything>") with a title and a filter.
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
	LibraryID   int64   `json:"library_id,omitempty"`
	Genre       string  `json:"genre,omitempty"`
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
	if p.Theme != "" && p.Theme != "dark" && p.Theme != "light" {
		return fmt.Errorf(`theme is "dark", "light" or "" (follow the device)`)
	}
	if len(p.SubtitleLanguage) > 8 {
		return fmt.Errorf("subtitle_language is a language code like \"eng\"")
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
		if r.Style != "" && r.Style != "poster" && r.Style != "thumb" {
			return fmt.Errorf(`a row's style is "poster" or "thumb"`)
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
	p := prefs{Home: []homeRow{}}
	raw, err := s.store.UserPrefs(r.Context(), currentUserID(r))
	if err != nil {
		return p, err
	}
	// Preferences that can't be read fall back to the defaults rather than breaking the app.
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		s.log.Warn("ignoring unreadable profile preferences", "profile", currentUser(r).Name, "err", err)
		p = prefs{}
	}
	if p.Home == nil {
		p.Home = []homeRow{}
	}
	return p, nil
}

// getPrefs: GET /api/me/prefs is the current profile's preferences.
func (s *Server) getPrefs(w http.ResponseWriter, r *http.Request) {
	p, err := s.loadPrefs(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

// updatePrefs: PATCH /api/me/prefs changes the fields sent ({"hide_spoilers": true} or
// {"home": [...]}) and returns them all.
func (s *Server) updatePrefs(w http.ResponseWriter, r *http.Request) {
	p, err := s.loadPrefs(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	var body struct {
		HideSpoilers     *bool     `json:"hide_spoilers"`
		Theme            *string   `json:"theme"`
		SubtitleLanguage *string   `json:"subtitle_language"`
		SplitUpNext      *bool     `json:"split_up_next"`
		Home             []homeRow `json:"home"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "send the preferences to change as JSON")
		return
	}
	if body.HideSpoilers != nil {
		p.HideSpoilers = *body.HideSpoilers
	}
	if body.Theme != nil {
		p.Theme = *body.Theme
	}
	if body.SubtitleLanguage != nil {
		p.SubtitleLanguage = *body.SubtitleLanguage
	}
	if body.SplitUpNext != nil {
		p.SplitUpNext = *body.SplitUpNext
	}
	if body.Home != nil {
		p.Home = body.Home
	}
	if err := p.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	raw, _ := json.Marshal(p)
	if err := s.store.SetUserPrefs(r.Context(), currentUserID(r), string(raw)); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}
