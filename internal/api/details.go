package api

import (
	"net/http"
	"strconv"

	"ams/internal/artwork"
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
	d.Genres = genres
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
