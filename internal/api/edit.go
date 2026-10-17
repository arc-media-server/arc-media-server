package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"ams/internal/config"
	"ams/internal/scanner"
	"ams/internal/store"
)

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// editItem: PATCH /api/items/{id} changes metadata by hand. Each field sent is set and locked so
// refreshes keep it; "locked_fields" replaces the locked set (leave a field out to unlock it).
//
//	{"title": "Alien", "year": 1979, "locked_fields": ["title", "year", "poster"]}
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
		// A show's episode numbering; "" follows its library.
		SourceOrdering *string   `json:"source_ordering"`
		LockedFields   *[]string `json:"locked_fields"`
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
		Tagline: trim(body.Tagline), ContentRating: trim(body.ContentRating), Studio: trim(body.Studio),
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
	if err := s.store.EditItem(r.Context(), it.ID, e); err != nil {
		s.fail(w, err)
		return
	}
	if body.SourceOrdering != nil && *body.SourceOrdering != it.SourceOrdering {
		if err := s.store.SetSourceOrdering(r.Context(), it.ID, *body.SourceOrdering); err != nil {
			s.fail(w, err)
			return
		}
		// Match the files to episodes again with the new numbering.
		if lib, err := s.store.GetLibrary(r.Context(), it.LibraryID); err == nil && lib != nil {
			if err := s.scanner.SyncSeriesMetadata(r.Context(), lib, it.ID, false); err != nil {
				s.log.Warn("re-numbering a show failed", "series", it.Title, "err", err)
			}
			s.scanner.Start(lib.ID)
		}
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
	ctx := r.Context()
	var err error
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
			if err = s.scanner.SyncSeriesMetadata(ctx, lib, seriesID, false); err == nil {
				// Re-link files so episodes known only from file names get their positions back.
				s.scanner.Start(lib.ID)
			}
		}
	}
	if errors.Is(err, scanner.ErrNoProvider) {
		writeError(w, http.StatusConflict, "add a TMDB API key to ams.yaml to fetch metadata")
		return
	}
	if err != nil {
		s.log.Warn("refresh failed", "item", it.Title, "err", err)
		writeError(w, http.StatusBadGateway, "refresh failed: "+err.Error())
		return
	}
	s.getItem(w, r)
}
