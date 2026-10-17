package api

import (
	"net/http"
	"strconv"

	"ams/internal/artwork"
	"ams/internal/store"
)

type creditDTO struct {
	PersonID  int64  `json:"person_id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Character string `json:"character,omitempty"`
	Image     string `json:"image,omitempty"`
}

// addDetails fills in a movie or show's genres, cast and crew.
func (s *Server) addDetails(r *http.Request, d *itemDTO) error {
	if d.Kind != "movie" && d.Kind != "series" {
		return nil
	}
	genres, err := s.store.ItemGenres(r.Context(), d.ID)
	if err != nil {
		return err
	}
	credits, err := s.store.ItemCredits(r.Context(), d.ID)
	if err != nil {
		return err
	}
	extras, err := s.store.ItemExtras(r.Context(), d.ID)
	if err != nil {
		return err
	}
	d.Extras = make([]extraDTO, 0, len(extras))
	for _, e := range extras {
		d.Extras = append(d.Extras, extraDTO{Extra: e, Stream: "/api/extras/" + e.ID + "/stream"})
	}
	d.Genres = genres
	d.Collections = s.itemCollections(r, &d.Item)
	d.Credits = make([]creditDTO, 0, len(credits))
	for _, c := range credits {
		cd := creditDTO{PersonID: c.PersonID, Name: c.Name, Role: c.Role, Character: c.Character}
		if c.ProfilePath != "" {
			cd.Image = "/api/people/" + strconv.FormatInt(c.PersonID, 10) + "/image?v=" + artwork.Key(artwork.Source("person", "primary", c.ProfilePath))
		}
		d.Credits = append(d.Credits, cd)
	}
	return nil
}

type extraDTO struct {
	store.Extra
	Stream string `json:"stream_url"`
}

// streamExtra: GET /api/extras/{id}/stream plays a trailer or other extra (direct play), if the
// profile may see the title it belongs to.
func (s *Server) streamExtra(w http.ResponseWriter, r *http.Request) {
	e, err := s.store.GetExtra(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return
	}
	if e == nil {
		writeError(w, http.StatusNotFound, "extra not found")
		return
	}
	owner, err := s.store.GetItem(r.Context(), e.ItemID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if ok, err := s.store.CanSee(r.Context(), owner, access(r)); err != nil || !ok {
		writeError(w, http.StatusNotFound, "extra not found")
		return
	}
	stop, done, refused := s.playing.openStream(r, owner.ID, e.ID)
	if refused != "" {
		writeError(w, http.StatusForbidden, refused)
		return
	}
	defer done()
	s.serveFile(w, r, e.Path, stop)
}

// libraryFacets: GET /api/libraries/{id}/facets lists the genres, decades and resolutions a
// library's titles can be filtered by, with counts.
func (s *Server) libraryFacets(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	kind := "movie"
	if lib.Kind == "shows" {
		kind = "series"
	}
	f, err := s.store.Facets(r.Context(), lib.ID, kind, access(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// personImage: GET /api/people/{id}/image serves a person's photo from the artwork cache.
func (s *Server) personImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid person id")
		return
	}
	path, err := s.store.PersonImage(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	if path == "" {
		writeError(w, http.StatusNotFound, "no image")
		return
	}
	s.serveArtwork(w, r, artwork.Source("person", "primary", path))
}
