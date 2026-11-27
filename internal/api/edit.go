package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"ams/internal/activity"
	"ams/internal/config"
	"ams/internal/scanner"
	"ams/internal/store"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// cleanLabels trims labels and drops blanks and repeats (ignoring case), up to 50.
func cleanLabels(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, l := range in {
		if l = strings.TrimSpace(l); l != "" && len(l) <= 60 && !seen[strings.ToLower(l)] && len(out) < 50 {
			seen[strings.ToLower(l)] = true
			out = append(out, l)
		}
	}
	return out
}

// listLabels: GET /api/labels lists every label in use (admin), for suggestions.
func (s *Server) listLabels(w http.ResponseWriter, r *http.Request) {
	labels, err := s.store.AllLabels(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, labels)
}

// editItem: PATCH /api/items/{id}. Edits metadata (including genres and labels; labels are
// never locked); edited fields are locked. locked_fields replaces the locked set. For a show,
// source_ordering changes file numbering (empty follows the library). Show details use the
// library metadata provider; legacy details_provider inputs are rejected. Episode metadata
// follows file numbering. Display order is changed separately through the series API.
func (s *Server) editItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Title         *string   `json:"title"`
		SortTitle     *string   `json:"sort_title"`
		OriginalTitle *string   `json:"original_title"`
		Year          *int      `json:"year"`
		Overview      *string   `json:"overview"`
		Tagline       *string   `json:"tagline"`
		ContentRating *string   `json:"content_rating"`
		Studio        *string   `json:"studio"`
		ReleaseDate   *string   `json:"release_date"`
		RuntimeMin    *int      `json:"runtime_min"`
		Genres        *[]string `json:"genres"`
		Labels        *[]string `json:"labels"`
		// A show's episode numbering; "" follows its library.
		SourceOrdering *string `json:"source_ordering"`
		// A show label override; "" follows the library.
		SeasonLabelMode *string `json:"season_label_mode"`
		// Where a show's details come from: "" for TMDB, or "tvdb".
		DetailsProvider *string   `json:"details_provider" api:"deprecated" description:"Legacy input is rejected; choose the metadata provider in library settings."`
		LockedFields    *[]string `json:"locked_fields"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	trim := func(p *string) *string {
		if p == nil {
			return nil
		}
		v := strings.TrimSpace(*p)
		return &v
	}
	e := store.ItemEdit{
		Title: trim(body.Title), SortTitle: trim(body.SortTitle), OriginalTitle: trim(body.OriginalTitle),
		Year: body.Year, Overview: trim(body.Overview), ReleaseDate: trim(body.ReleaseDate), RuntimeMin: body.RuntimeMin,
		Tagline: trim(body.Tagline), ContentRating: trim(body.ContentRating), Studio: trim(body.Studio), SeasonLabelMode: body.SeasonLabelMode,
	}
	switch {
	case e.Title != nil && *e.Title == "":
		writeError(w, http.StatusBadRequest, "title can't be empty")
		return
	case e.Year != nil && *e.Year != 0 && (*e.Year < 1870 || *e.Year > 2200):
		writeError(w, http.StatusBadRequest, "year must be between 1870 and 2200, or 0 for none")
		return
	case e.ReleaseDate != nil && *e.ReleaseDate != "" && !dateRe.MatchString(*e.ReleaseDate):
		writeError(w, http.StatusBadRequest, "release_date must look like 2024-03-05")
		return
	case e.RuntimeMin != nil && (*e.RuntimeMin < 0 || *e.RuntimeMin > 100000):
		writeError(w, http.StatusBadRequest, "runtime_min must be 0 or more")
		return
	case body.DetailsProvider != nil && (it.Kind != "series" || (*body.DetailsProvider != "" && *body.DetailsProvider != "tvdb")):
		writeError(w, http.StatusBadRequest, `details_provider is for shows: "" (TMDB) or "tvdb"`)
		return
	case body.SeasonLabelMode != nil && (it.Kind != "series" || *body.SeasonLabelMode != "" && !validSeasonLabelMode(*body.SeasonLabelMode)):
		writeError(w, http.StatusBadRequest, `season_label_mode is for shows: "number", "name" or "" to follow the library`)
		return
	case body.SourceOrdering != nil && (it.Kind != "series" || *body.SourceOrdering != "" && !config.ValidSourceOrdering(*body.SourceOrdering)):
		writeError(w, http.StatusBadRequest, "source_ordering is for shows: tmdb_aired, tvdb_aired, tvdb_dvd, tvdb_absolute, or \"\" for the library's")
		return
	}
	if body.Genres != nil {
		genres := []string{}
		seen := map[string]bool{}
		for _, g := range *body.Genres {
			if g = strings.TrimSpace(g); g != "" && !seen[strings.ToLower(g)] && len(genres) < 30 {
				seen[strings.ToLower(g)] = true
				genres = append(genres, g)
			}
		}
		e.Genres = &genres
	}
	if body.Labels != nil {
		labels := cleanLabels(*body.Labels)
		e.Labels = &labels
	}
	if body.LockedFields != nil {
		e.Locked = *body.LockedFields
		if e.Locked == nil {
			e.Locked = []string{}
		}
		for _, f := range e.Locked {
			if store.LockableFields[f] == "" {
				writeError(w, http.StatusBadRequest, "unknown field in locked_fields: "+f)
				return
			}
		}
	}

	if body.DetailsProvider != nil {
		writeError(w, 400, "Choose the metadata provider in library settings; episode metadata follows File episode order")
		return
	}
	if body.SourceOrdering != nil {
		lib, err := s.store.GetLibrary(r.Context(), it.LibraryID)
		if err != nil {
			s.fail(w, err)
			return
		}
		err = s.scanner.RefreshSeries(r.Context(), lib, it.ID, false, func(ctx context.Context) error {
			if err := s.store.EditItem(ctx, it.ID, e); err != nil {
				return err
			}
			return s.store.SetSourceOrdering(ctx, it.ID, *body.SourceOrdering)
		})
		if err != nil {
			s.seriesRefreshError(w, err)
			return
		}
	} else if err := s.store.EditItem(r.Context(), it.ID, e); err != nil {
		s.fail(w, err)
		return
	}

	s.getItem(w, r)
}

// refreshItem: POST /api/items/{id}/refresh fetches metadata again now. Locked fields are kept.
// Episodes refresh their whole series.
func (s *Server) refreshItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if it.MatchSuppressed {
		writeError(w, 409, "This title was explicitly unmatched; choose Fix Match before refreshing metadata")
		return
	}
	ctx := r.Context()
	var err error
	var task *activity.Handle
	if it.Kind == "movie" {
		task = s.activity.Start("refresh", "Refreshing "+it.Title)
	}
	defer func() { task.Finish(err) }()
	switch it.Kind {
	case "movie":
		_, err = s.scanner.RefreshMovie(ctx, it.ID)
	default:
		seriesID := it.ID
		if it.Kind == "episode" {
			seriesID = it.ParentID
		}
		var lib *store.Library
		if lib, err = s.store.GetLibrary(ctx, it.LibraryID); err == nil && lib != nil {
			err = s.scanner.RefreshSeries(ctx, lib, seriesID, false, nil)
		}
	}
	if errors.Is(err, scanner.ErrLibraryBusy) {
		s.seriesRefreshError(w, err)
		return
	}
	if errors.Is(err, scanner.ErrNoProvider) {
		writeError(w, http.StatusConflict, "add a TMDB API key in Settings to fetch metadata")
		return
	}
	if err != nil {
		s.log.Warn("refresh failed", "item", it.Title, "err", err)
		writeError(w, http.StatusBadGateway, "refresh failed: "+err.Error())
		return
	}
	s.getItem(w, r)
}
