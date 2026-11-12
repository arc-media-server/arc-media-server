package api

import (
	"ams/internal/store"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func (s *Server) downloadAccess(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "private, no-store")
	u := currentUser(r)
	if u == nil || (!u.IsAdmin && !u.AllowDownloads) {
		writeError(w, 403, "downloads are not allowed for this profile")
		return false
	}
	if s.downloads == nil {
		writeError(w, 503, "download preparation is unavailable")
		return false
	}
	return true
}
func (s *Server) downloadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrDownloadNotFound):
		writeError(w, 404, "download unavailable")
	case errors.Is(err, store.ErrDownloadConflict):
		writeError(w, 409, err.Error())
	case errors.Is(err, store.ErrDownloadLimit):
		w.Header().Set("Retry-After", "60")
		writeError(w, 429, err.Error())
	default:
		s.fail(w, err)
	}
}

// createDownload: POST /api/downloads prepares one original movie/episode descriptor. No
// bytes, playback, conversion or offline sync are provided by this preparation-only API.
func (s *Server) createDownload(w http.ResponseWriter, r *http.Request) {
	if !s.downloadAccess(w, r) {
		return
	}
	var b struct {
		ItemID         string `json:"item_id"`
		FileID         int64  `json:"file_id"`
		Mode           string `json:"mode"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&b); err != nil {
		var large *http.MaxBytesError
		if errors.As(err, &large) {
			writeError(w, 413, "download request is too large")
		} else {
			writeError(w, 400, "invalid download request")
		}
		return
	}
	tailErr := decoder.Decode(&struct{}{})
	var tooLarge *http.MaxBytesError
	if errors.As(tailErr, &tooLarge) {
		writeError(w, 413, "download request is too large")
		return
	}
	if tailErr != io.EOF || b.ItemID == "" || len(b.ItemID) > 200 || b.FileID <= 0 || len(b.IdempotencyKey) == 0 || len(b.IdempotencyKey) > 80 || strings.TrimSpace(b.IdempotencyKey) != b.IdempotencyKey {
		writeError(w, 400, "invalid download selection or idempotency key")
		return
	}
	if b.Mode != "original" {
		writeError(w, 422, "only original preparation is supported")
		return
	}
	// Reuse the existing item visibility boundary without trusting a separate caller-supplied owner.
	r.SetPathValue("id", b.ItemID)
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	if it.Kind != "movie" && it.Kind != "episode" {
		writeError(w, 422, "select one movie or episode")
		return
	}
	files, err := s.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	var selected *store.MediaFile
	multipart := false
	for i := range files {
		if files[i].Part > 1 {
			multipart = true
		}
		if files[i].ID == b.FileID && !files[i].Missing {
			selected = &files[i]
		}
	}
	if selected == nil {
		writeError(w, 404, "file unavailable")
		return
	}
	links, err := s.store.FileLinks(r.Context(), []int64{selected.ID})
	if err != nil {
		s.fail(w, err)
		return
	}
	if multipart || len(links[selected.ID]) != 1 {
		writeError(w, 422, "multipart and shared episode files are not supported for downloads yet")
		return
	}
	requestStage(r, "registering download preparation", it.LibraryID)
	d, created, err := s.store.CreateDownload(r.Context(), currentUserID(r), currentSession(r), it.ID, b.IdempotencyKey, *selected)
	if err != nil {
		s.downloadError(w, err)
		return
	}
	s.downloads.Wake()
	if created {
		writeJSON(w, http.StatusCreated, d)
	} else {
		writeJSON(w, http.StatusOK, d)
	}
}

// listDownloads: GET /api/downloads lists this profile/device's visible preparation records.
// ?limit=50&offset=0, limit 1-100. It does not probe or hash files.
func (s *Server) listDownloads(w http.ResponseWriter, r *http.Request) {
	if !s.downloadAccess(w, r) {
		return
	}
	limit, offset := 50, 0
	for key, p := range map[string]*int{"limit": &limit, "offset": &offset} {
		if raw, ok := r.URL.Query()[key]; ok {
			if len(raw) != 1 {
				writeError(w, 400, "invalid pagination")
				return
			}
			v, err := strconv.Atoi(raw[0])
			if err != nil {
				writeError(w, 400, "invalid pagination")
				return
			}
			*p = v
		}
	}
	if limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		writeError(w, 400, "invalid pagination")
		return
	}
	requestStage(r, "listing download preparations", 0)
	visible, err := s.store.VisibleDownloadRecords(r.Context(), currentUserID(r), currentSession(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	total := len(visible)
	start := min(offset, total)
	end := min(start+limit, total)
	writeJSON(w, 200, struct {
		Items  []store.Download `json:"items"`
		Total  int              `json:"total"`
		Offset int              `json:"offset"`
	}{visible[start:end], total, offset})
}

// getDownload: GET /api/downloads/{id} returns the preparation descriptor, not a media URL.
func (s *Server) getDownload(w http.ResponseWriter, r *http.Request) {
	if !s.downloadAccess(w, r) {
		return
	}
	requestStage(r, "loading download preparation", 0)
	d, err := s.store.Download(r.Context(), currentUserID(r), currentSession(r), r.PathValue("id"))
	if err != nil {
		s.downloadError(w, err)
		return
	}
	if _, err = s.store.DownloadSource(r.Context(), d); err != nil {
		s.downloadError(w, err)
		return
	}
	if d.State == "ready" {
		info, err := os.Stat(d.SourcePath)
		if err != nil || !info.Mode().IsRegular() || info.Size() != d.Size || info.ModTime().UnixNano() != d.SourceMTimeNS {
			writeError(w, 409, "source changed; release and prepare again")
			return
		}
	}
	writeJSON(w, 200, d)
}

// deleteDownload: DELETE /api/downloads/{id} releases this profile/device's preparation record.
// Cancels preparation at its next check; no source or device-local files are deleted.
func (s *Server) deleteDownload(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	requestStage(r, "releasing download preparation", 0)
	// Releasing owned bookkeeping is allowed after a grant is withdrawn and when workers are off.
	if err := s.store.DeleteDownload(r.Context(), currentUserID(r), currentSession(r), r.PathValue("id")); err != nil {
		s.downloadError(w, err)
		return
	}
	w.WriteHeader(204)
}
