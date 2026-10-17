package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ams/internal/metadata"
	"ams/internal/scanner"
	"ams/internal/store"
)

// Uploaded images may be at most this big.
const maxUploadBytes = 20 << 20

var uploadTypes = map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}

// artworkField is the locked field for an artwork type ("poster" or "backdrop").
func artworkField(typ string) (string, bool) {
	switch typ {
	case "poster", "primary":
		return "poster", true
	case "backdrop":
		return "backdrop", true
	}
	return "", false
}

// listArtwork: GET /api/items/{id}/artwork lists the posters and backgrounds TMDB has for a movie
// or show, with the ones in use and whether they were chosen by hand.
func (s *Server) listArtwork(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if it.Kind != "movie" && it.Kind != "series" {
		writeError(w, http.StatusBadRequest, "only movies and shows have artwork to choose")
		return
	}
	posters, backdrops, err := s.scanner.Images(r.Context(), it)
	if errors.Is(err, scanner.ErrNoProvider) {
		posters, backdrops = []metadata.Image{}, []metadata.Image{}
	} else if err != nil {
		s.log.Warn("listing artwork failed", "item", it.Title, "err", err)
		writeError(w, http.StatusBadGateway, "couldn't get artwork from TMDB: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"posters":   posters,
		"backdrops": backdrops,
		"current":   map[string]string{"poster": it.PosterPath, "backdrop": it.BackdropPath},
		"chosen": map[string]bool{"poster": slices.Contains(it.LockedFields, "poster"),
			"backdrop": slices.Contains(it.LockedFields, "backdrop")},
		"local": map[string]bool{"poster": it.LocalPoster != "", "backdrop": it.LocalBackdrop != ""},
	})
}

// chooseArtwork: PUT /api/items/{id}/artwork {"type": "poster", "path": "/abc.jpg"} uses a TMDB
// image and locks it. A null path goes back to automatic artwork.
func (s *Server) chooseArtwork(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Type string  `json:"type"`
		Path *string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	field, ok := artworkField(body.Type)
	if !ok {
		writeError(w, http.StatusBadRequest, `type is "poster" or "backdrop"`)
		return
	}
	if body.Path != nil && !strings.HasPrefix(*body.Path, "/") {
		writeError(w, http.StatusBadRequest, "path is a TMDB image path such as /abc.jpg")
		return
	}
	old := imageField(it, field)
	if body.Path == nil {
		// Back to automatic: unlock, then fetch the provider's choice again.
		locked := slices.DeleteFunc(slices.Clone(it.LockedFields), func(f string) bool { return f == field })
		if err := s.store.EditItem(r.Context(), it.ID, store.ItemEdit{Locked: locked}); err != nil {
			s.fail(w, err)
			return
		}
		s.refetch(r, it)
	} else if err := s.setArtwork(r, it, field, *body.Path); err != nil {
		s.fail(w, err)
		return
	}
	s.removeUpload(old)
	s.getItem(w, r)
}

// uploadArtwork: POST /api/items/{id}/artwork/{type} with an image as the body (JPEG, PNG or
// WebP, up to 20 MB) uses it as the poster or background. It's kept in AMS's data folder.
func (s *Server) uploadArtwork(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	field, ok := artworkField(r.PathValue("type"))
	if !ok {
		writeError(w, http.StatusBadRequest, `type is "poster" or "backdrop"`)
		return
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "couldn't read the upload")
		return
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "images can be up to 20 MB")
		return
	}
	ext, ok := uploadTypes[http.DetectContentType(data)]
	if !ok {
		writeError(w, http.StatusUnsupportedMediaType, "upload a JPEG, PNG or WebP image")
		return
	}
	var rnd [6]byte
	rand.Read(rnd[:])
	name := it.ID + "-" + field + "-" + hex.EncodeToString(rnd[:]) + ext
	if err := os.MkdirAll(s.uploadsDir, 0o755); err != nil {
		s.fail(w, err)
		return
	}
	if err := os.WriteFile(filepath.Join(s.uploadsDir, name), data, 0o644); err != nil {
		s.fail(w, err)
		return
	}
	old := imageField(it, field)
	if err := s.setArtwork(r, it, field, metadata.UploadPrefix+name); err != nil {
		os.Remove(filepath.Join(s.uploadsDir, name))
		s.fail(w, err)
		return
	}
	s.removeUpload(old)
	s.getItem(w, r)
}

// setArtwork uses an image (provider path or upload) as an item's poster or backdrop, locked.
func (s *Server) setArtwork(r *http.Request, it *store.Item, field, path string) error {
	e := store.ItemEdit{}
	if field == "poster" {
		e.Poster = &path
	} else {
		e.Backdrop = &path
	}
	return s.store.EditItem(r.Context(), it.ID, e)
}

func imageField(it *store.Item, field string) string {
	if field == "poster" {
		return it.PosterPath
	}
	return it.BackdropPath
}

// removeUpload deletes an uploaded image that's no longer used.
func (s *Server) removeUpload(path string) {
	if name, ok := strings.CutPrefix(path, metadata.UploadPrefix); ok {
		os.Remove(filepath.Join(s.uploadsDir, filepath.Base(name)))
	}
}

// refetch fetches a movie or show's metadata again in the background-friendly way refreshItem does.
func (s *Server) refetch(r *http.Request, it *store.Item) {
	var err error
	if it.Kind == "movie" {
		_, err = s.scanner.RefreshMovie(r.Context(), it.ID)
	} else if lib, lerr := s.store.GetLibrary(r.Context(), it.LibraryID); lerr == nil && lib != nil {
		err = s.scanner.SyncSeriesMetadata(r.Context(), lib, it.ID, false)
	}
	if err != nil {
		s.log.Warn("refetching metadata failed", "item", it.Title, "err", err)
	}
}
