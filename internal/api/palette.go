package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ams/internal/artwork"
	"ams/internal/metadata"
	"ams/internal/palette"
	"ams/internal/store"
)

type paletteResponse struct {
	Available      bool            `json:"available"`
	Algorithm      string          `json:"algorithm"`
	ColorSpace     string          `json:"color_space"`
	ArtworkType    string          `json:"artwork_type"`
	ArtworkURL     string          `json:"artwork_url"`
	ArtworkVersion string          `json:"artwork_version"`
	Colors         *palette.Colors `json:"colors,omitempty"`
	Reason         string          `json:"reason,omitempty"`
}

// artworkPalette: GET /api/items/{id}/artwork/palette returns cached regional sRGB hex colours for
// a title's selected artwork (profile). ?type=backdrop (default) prefers a backdrop, falling back
// to primary only if no backdrop is configured; ?type=primary uses its poster/still. Missing or
// undecodable artwork answers 200 with available=false and a reason, never a theme-colour guess.
func (s *Server) artworkPalette(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	typ := r.URL.Query().Get("type")
	if typ == "" {
		typ = "backdrop"
	}
	if typ != "backdrop" && typ != "primary" {
		writeError(w, 400, "type must be backdrop or primary")
		return
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Add("Vary", "Authorization")
	w.Header().Add("Vary", "Cookie")
	out := paletteResponse{Algorithm: palette.Algorithm, ColorSpace: "srgb", ArtworkType: typ}
	path, local := imagePaths(it, typ)
	if typ == "backdrop" && path == "" && local == "" {
		typ = "primary"
		out.ArtworkType = typ
		path, local = imagePaths(it, typ)
	}
	if path == "" && local == "" {
		out.Reason = "no_artwork"
		writeJSON(w, 200, out)
		return
	}
	if s.palettes == nil {
		out.Reason = "image_unavailable"
		writeJSON(w, 200, out)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	f, imageURL, err := s.paletteImage(ctx, it, typ, path, local)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		out.Reason = "image_unavailable"
		s.paletteWarning(it.ID, typ, out.Reason)
		writeJSON(w, 200, out)
		return
	}
	defer f.Close()
	requestStage(r, "extracting artwork colours", 0)
	result, err := s.palettes.Get(ctx, f)
	if err != nil {
		if r.Context().Err() != nil {
			return
		}
		out.Reason = "image_unavailable"
		switch {
		case errors.Is(err, palette.ErrTooLarge):
			out.Reason = "image_too_large"
		case errors.Is(err, palette.ErrUnsupported):
			out.Reason = "unsupported_image"
		case errors.Is(err, palette.ErrNoColors):
			out.Reason = "no_visible_colors"
		}
		s.paletteWarning(it.ID, typ, out.Reason)
		writeJSON(w, 200, out)
		return
	}
	out.Available = true
	out.Colors = &result.Colors
	out.ArtworkVersion = result.Version
	out.ArtworkURL = imageURL + "&palette_version=" + result.Version
	raw, _ := json.Marshal(out)
	digest := sha256.Sum256(raw)
	etag := `"` + hex.EncodeToString(digest[:]) + `"`
	w.Header().Set("ETag", etag)
	if paletteETagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// Only paths already selected by the item's existing artwork rules are read. No client URL/path
// is accepted. Uploaded filenames remain confined to the uploads directory.
func (s *Server) paletteImage(ctx context.Context, it *store.Item, typ, path, local string) (*os.File, string, error) {
	uploaded := strings.HasPrefix(path, metadata.UploadPrefix)
	if uploaded {
		local = filepath.Join(s.uploadsDir, filepath.Base(strings.TrimPrefix(path, metadata.UploadPrefix)))
	}
	base := "/api/v1/items/" + url.PathEscape(it.ID) + "/images/" + typ
	if local != "" {
		if f, err := os.Open(local); err == nil {
			return f, base + "?local=" + artwork.Key(local), nil
		}
	}
	if uploaded || path == "" || s.artwork == nil {
		return nil, "", os.ErrNotExist
	}
	src := artwork.Source(it.Kind, typ, path)
	f, err := s.artwork.Open(ctx, src)
	return f, base + "?v=" + artwork.Key(src), err
}

// Optional decoration failures are bounded to one warning/minute, not one log per page/poll.
func (s *Server) paletteWarning(id, typ, reason string) {
	s.paletteLogMu.Lock()
	defer s.paletteLogMu.Unlock()
	if time.Since(s.paletteWarned) < time.Minute {
		return
	}
	s.paletteWarned = time.Now()
	s.log.Warn("artwork palette unavailable", "item", id, "type", typ, "reason", reason)
}

func paletteETagMatches(header, etag string) bool {
	for _, tag := range strings.Split(header, ",") {
		tag = strings.TrimSpace(tag)
		if tag == "*" || strings.TrimPrefix(tag, "W/") == etag {
			return true
		}
	}
	return false
}
