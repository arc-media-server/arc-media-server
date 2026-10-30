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

// artworkField is the locked field for an artwork type ("poster", "backdrop" or "logo").
func artworkField(typ string) (string, bool) {
	switch typ {
	case "poster", "primary":
		return "poster", true
	case "backdrop":
		return "backdrop", true
	case "logo":
		return "logo", true
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
	var posters, backdrops, logos []metadata.Image
	var err error
	if it.Kind == "episode" {
		// An episode's "poster" is its thumbnail: TMDB's stills for it.
		posters, err = s.episodeStills(r, it)
		backdrops, logos = []metadata.Image{}, []metadata.Image{}
	} else {
		posters, backdrops, logos, err = s.scanner.Images(r.Context(), it)
	}
	if errors.Is(err, scanner.ErrNoProvider) {
		posters, backdrops, logos = []metadata.Image{}, []metadata.Image{}, []metadata.Image{}
	} else if err != nil {
		s.log.Warn("listing artwork failed", "item", it.Title, "err", err)
		writeError(w, http.StatusBadGateway, "couldn't get artwork from TMDB: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"posters":   posters,
		"backdrops": backdrops,
		"logos":     logos,
		"current":   map[string]string{"poster": it.PosterPath, "backdrop": it.BackdropPath, "logo": it.LogoPath},
		"chosen": map[string]bool{"poster": slices.Contains(it.LockedFields, "poster"),
			"backdrop": slices.Contains(it.LockedFields, "backdrop"), "logo": slices.Contains(it.LockedFields, "logo")},
		"local": map[string]bool{"poster": it.LocalPoster != "", "backdrop": it.LocalBackdrop != "", "logo": it.LocalLogo != ""},
	})
}

// episodeStills lists TMDB's stills for an episode, found by its position in TMDB's aired order.
func (s *Server) episodeStills(r *http.Request, ep *store.Item) ([]metadata.Image, error) {
	ctx := r.Context()
	series, err := s.store.GetItem(ctx, ep.ParentID)
	if err != nil || series == nil {
		return []metadata.Image{}, err
	}
	orderings, err := s.store.ListOrderings(ctx, series.ID)
	if err != nil {
		return nil, err
	}
	for _, o := range orderings {
		if o.Source != "tmdb_aired" {
			continue
		}
		entries, err := s.store.OrderingEntries(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.EpisodeID == ep.ID {
				return s.scanner.EpisodeStills(ctx, series, e.Season, e.Episode)
			}
		}
	}
	return []metadata.Image{}, nil
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
		Path *string `json:"path" api:"nullable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	field, ok := artworkField(body.Type)
	if !ok {
		writeError(w, http.StatusBadRequest, `type is "poster", "backdrop" or "logo"`)
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
		writeError(w, http.StatusBadRequest, `type is "poster", "backdrop" or "logo"`)
		return
	}
	path, ok := s.saveUpload(w, r, it.ID+"-"+field)
	if !ok {
		return
	}
	old := imageField(it, field)
	if err := s.setArtwork(r, it, field, path); err != nil {
		s.removeUpload(path)
		s.fail(w, err)
		return
	}
	s.removeUpload(old)
	s.getItem(w, r)
}

// saveUpload keeps an uploaded image (JPEG, PNG or WebP, up to 20 MB) in the data folder and
// returns its path ("upload:<file>"), or writes the error and returns false.
func (s *Server) saveUpload(w http.ResponseWriter, r *http.Request, prefix string) (string, bool) {
	data, err := io.ReadAll(io.LimitReader(r.Body, maxUploadBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "couldn't read the upload")
		return "", false
	}
	if len(data) > maxUploadBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "images can be up to 20 MB")
		return "", false
	}
	ext, ok := uploadTypes[http.DetectContentType(data)]
	if !ok {
		writeError(w, http.StatusUnsupportedMediaType, "upload a JPEG, PNG or WebP image")
		return "", false
	}
	var rnd [6]byte
	rand.Read(rnd[:])
	name := prefix + "-" + hex.EncodeToString(rnd[:]) + ext
	if err := os.MkdirAll(s.uploadsDir, 0o755); err != nil {
		s.fail(w, err)
		return "", false
	}
	if err := os.WriteFile(filepath.Join(s.uploadsDir, name), data, 0o644); err != nil {
		s.fail(w, err)
		return "", false
	}
	return metadata.UploadPrefix + name, true
}

// setArtwork uses an image (provider path or upload) as an item's poster or backdrop, locked.
func (s *Server) setArtwork(r *http.Request, it *store.Item, field, path string) error {
	e := store.ItemEdit{}
	switch field {
	case "poster":
		e.Poster = &path
	case "logo":
		e.Logo = &path
	default:
		e.Backdrop = &path
	}
	return s.store.EditItem(r.Context(), it.ID, e)
}

func imageField(it *store.Item, field string) string {
	switch field {
	case "poster":
		return it.PosterPath
	case "logo":
		return it.LogoPath
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
