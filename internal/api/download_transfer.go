package api

import (
	"ams/internal/downloads"
	"ams/internal/store"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type downloadAssetDTO struct {
	ID              string `json:"id"`
	Role            string `json:"role"`
	URL             string `json:"url"`
	MediaType       string `json:"media_type"`
	Size            int64  `json:"size"`
	SHA256          string `json:"sha256"`
	Language        string `json:"language,omitempty"`
	Forced          bool   `json:"forced,omitempty"`
	HearingImpaired bool   `json:"hearing_impaired,omitempty"`
}
type downloadItemSnapshot = downloads.BundleItem

type downloadManifest struct {
	Unavailable []downloads.MissingAsset `json:"unavailable"`
	ProfileID   string                   `json:"profile_id"`
	DownloadID  string                   `json:"download_id"`
	Revision    string                   `json:"revision"`
	ServerID    string                   `json:"server_id"`
	Mode        string                   `json:"mode"`
	Item        downloadItemSnapshot     `json:"item"`
	Assets      []downloadAssetDTO       `json:"assets"`
}

func (s *Server) downloadReadLease(w http.ResponseWriter, r *http.Request) (func(), bool) {
	if !s.downloadAccess(w, r) {
		return nil, false
	}
	release, ok := s.store.BeginCatalogWork()
	if !ok {
		w.Header().Set("Retry-After", "3")
		writeError(w, 503, "catalog maintenance in progress; retry shortly")
		return nil, false
	}
	return release, true
}
func (s *Server) readyDownload(w http.ResponseWriter, r *http.Request) (*store.Download, bool) {
	d, err := s.store.Download(r.Context(), currentUserID(r), currentSession(r), r.PathValue("id"))
	if err != nil {
		s.downloadError(w, err)
		return nil, false
	}
	if _, err = s.store.DownloadSource(r.Context(), d); err != nil {
		s.downloadError(w, err)
		return nil, false
	}
	if d.State != "ready" || len(d.SHA256) != 64 {
		writeError(w, 409, "download is not ready; prepare again if it failed")
		return nil, false
	}
	return d, true
}
func downloadSourceInfo(d *store.Download, info os.FileInfo) bool {
	return info != nil && info.Mode().IsRegular() && info.Size() == d.Size && info.ModTime().UnixNano() == d.SourceMTimeNS
}
func openDownloadSource(d *store.Download) (*os.File, os.FileInfo, error) {
	before, err := os.Stat(d.SourcePath)
	if err != nil || !downloadSourceInfo(d, before) {
		return nil, nil, errors.New("download source changed")
	}
	f, err := os.Open(d.SourcePath)
	if err != nil {
		return nil, nil, errors.New("download source unavailable")
	}
	opened, err := f.Stat()
	if err != nil || !downloadSourceInfo(d, opened) || !os.SameFile(before, opened) {
		f.Close()
		return nil, nil, errors.New("download source changed")
	}
	return f, opened, nil
}
func downloadMediaType(path string) string {
	if v := videoMIME[strings.ToLower(filepath.Ext(path))]; v != "" {
		return v
	}
	return "application/octet-stream"
}

// getDownloadManifest: GET /api/downloads/{id}/manifest describes a ready original representation.
// Asset URLs require the same profile/device authentication; the digest is not a strong HTTP ETag.
func (s *Server) getDownloadManifest(w http.ResponseWriter, r *http.Request) {
	release, ok := s.downloadReadLease(w, r)
	if !ok {
		return
	}
	defer release()
	d, ok := s.readyDownload(w, r)
	if !ok {
		return
	}
	f, _, err := openDownloadSource(d)
	if err != nil {
		writeError(w, 409, "download source changed; prepare again")
		return
	}
	f.Close()
	lease := s.downloads.LeaseBundle(d)
	defer lease()
	var bundle downloads.Bundle
	if d.Snapshot == "" || json.Unmarshal([]byte(d.Snapshot), &bundle) != nil {
		writeError(w, 409, "offline snapshot unavailable; prepare again")
		return
	}
	manifest := downloadManifest{ProfileID: d.UserID, DownloadID: d.ID, Revision: d.Revision, ServerID: bundle.ServerID, Mode: "original", Item: bundle.Item, Assets: []downloadAssetDTO{{ID: d.Revision, Role: "media", URL: downloadAssetURL(d, d.Revision), MediaType: downloadMediaType(d.SourcePath), Size: d.Size, SHA256: d.SHA256}}, Unavailable: bundle.Unavailable}
	for _, asset := range bundle.Assets {
		f, err := s.downloads.OpenBundleAsset(d, asset)
		if err != nil {
			writeError(w, 409, "offline assets unavailable; prepare again")
			return
		}
		f.Close()
		manifest.Assets = append(manifest.Assets, downloadAssetDTO{ID: asset.ID, Role: asset.Role, URL: downloadAssetURL(d, asset.ID), MediaType: asset.MediaType, Size: asset.Size, SHA256: asset.SHA256, Language: asset.Language, Forced: asset.Forced, HearingImpaired: asset.HearingImpaired})
	}
	writeJSON(w, http.StatusOK, manifest)
}

func downloadAssetURL(d *store.Download, id string) string {
	return "/api/v1/downloads/" + url.PathEscape(d.ID) + "/assets/" + url.PathEscape(id)
}

type downloadRange struct{ start, length int64 }

func parseDownloadRange(header string, size int64) (downloadRange, error) {
	if header == "" {
		return downloadRange{0, size}, nil
	}
	bad := errors.New("invalid download range")
	if len(header) > 128 || !strings.HasPrefix(header, "bytes=") || strings.Contains(header, ",") {
		return downloadRange{}, bad
	}
	first, last, ok := strings.Cut(strings.TrimPrefix(header, "bytes="), "-")
	if !ok || size == 0 {
		return downloadRange{}, bad
	}
	number := func(s string) (int64, error) {
		if s == "" || strings.ContainsAny(s, "+ \t\r\n-") {
			return 0, bad
		}
		return strconv.ParseInt(s, 10, 64)
	}
	if first == "" {
		n, err := number(last)
		if err != nil || n <= 0 {
			return downloadRange{}, bad
		}
		n = min(n, size)
		return downloadRange{size - n, n}, nil
	}
	start, err := number(first)
	if err != nil || start < 0 || start >= size {
		return downloadRange{}, bad
	}
	end := size - 1
	if last != "" {
		end, err = number(last)
		if err != nil || end < start {
			return downloadRange{}, bad
		}
		end = min(end, size-1)
	}
	return downloadRange{start, end - start + 1}, nil
}

// downloadAsset: GET or HEAD /api/downloads/{id}/assets/{asset} transfers a ready original.
// Full 200, single-range 206, invalid/multiple ranges 416. No playback/history side effects.
func (s *Server) downloadAsset(w http.ResponseWriter, r *http.Request) {
	release, ok := s.downloadReadLease(w, r)
	if !ok {
		return
	}
	defer release()
	d, ok := s.readyDownload(w, r)
	if !ok {
		return
	}
	lease := s.downloads.LeaseBundle(d)
	defer lease()
	original := r.PathValue("asset") == d.Revision
	var asset downloads.BundleAsset
	var f *os.File
	var opened os.FileInfo
	var err error
	mediaType, size := downloadMediaType(d.SourcePath), d.Size
	if original {
		f, opened, err = openDownloadSource(d)
	} else {
		var bundle downloads.Bundle
		if d.Snapshot == "" || json.Unmarshal([]byte(d.Snapshot), &bundle) != nil {
			writeError(w, 409, "offline snapshot unavailable")
			return
		}
		found := false
		for _, a := range bundle.Assets {
			if a.ID == r.PathValue("asset") {
				asset = a
				found = true
				break
			}
		}
		if !found {
			writeError(w, 404, "download unavailable")
			return
		}
		f, err = s.downloads.OpenBundleAsset(d, asset)
		if err == nil {
			opened, err = f.Stat()
		}
		mediaType, size = asset.MediaType, asset.Size
	}
	if err != nil {
		if f != nil {
			f.Close()
		}
		writeError(w, 409, "download asset changed; prepare again")
		return
	}
	defer f.Close()
	rangeHeader := r.Header.Get("Range")
	if r.Method == "HEAD" || r.Header.Get("If-Range") != "" {
		rangeHeader = ""
	}
	selected, err := parseDownloadRange(rangeHeader, size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		writeError(w, 416, "invalid download range")
		return
	}
	if r.Method != "HEAD" {
		done, admitted := s.downloads.BeginTransfer(d.UserID, d.SessionHash)
		if !admitted {
			w.Header().Set("Retry-After", "3")
			writeError(w, 429, "download transfer limit reached")
			return
		}
		defer done()
	}
	if _, err = f.Seek(selected.start, io.SeekStart); err != nil {
		writeError(w, 409, "download source unavailable")
		return
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(selected.length, 10))
	// A fixed safe name avoids leaking the source filename or trusting title/header characters.
	extension := safeDownloadExtension(d.SourcePath)
	if !original {
		extension = map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp", "text/vtt": ".vtt", "application/x-subrip": ".srt"}[mediaType]
		if extension == "" {
			extension = ".bin"
		}
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "arc-download" + extension}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	status := 200
	if rangeHeader != "" {
		status = 206
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", selected.start, selected.start+selected.length-1, size))
	}
	if r.Method == "HEAD" {
		w.WriteHeader(status)
		return
	}
	started := time.Now()
	var sent int64
	reason := ""
	defer func() { s.downloads.TransferOutcome(d.ID, sent, time.Since(started), reason) }()
	var interrupted atomic.Bool
	monitorCtx, stop := context.WithCancel(r.Context())
	joined := make(chan struct{})
	validate := func() bool {
		if _, err := s.store.DownloadSource(monitorCtx, d); err != nil {
			return false
		}
		source, err := os.Stat(d.SourcePath)
		if err != nil || !downloadSourceInfo(d, source) {
			return false
		}
		current, err := f.Stat()
		if err != nil || current.Size() != size || !os.SameFile(opened, current) || !current.ModTime().Equal(opened.ModTime()) {
			return false
		}
		if original {
			return os.SameFile(opened, source)
		}
		verified, err := s.downloads.OpenBundleAsset(d, asset)
		if err != nil {
			return false
		}
		verified.Close()
		return true
	}

	abort := func() { interrupted.Store(true); _ = http.NewResponseController(w).SetWriteDeadline(time.Now()) }
	go func() {
		defer close(joined)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-monitorCtx.Done():
				if r.Context().Err() != nil {
					abort()
				}
				return
			case <-shuttingDown(r):
				abort()
				return
			case <-ticker.C:
				if !validate() {
					if monitorCtx.Err() == nil {
						abort()
					}
					return
				}
			}
		}
	}()
	defer func() { stop(); <-joined }()
	w.WriteHeader(status)
	buffer := make([]byte, 256*1024)
	remaining := selected.length
	for remaining > 0 {
		if r.Context().Err() != nil || interrupted.Load() {
			reason = "revoked"
			panic(http.ErrAbortHandler)
		}
		want := min(int64(len(buffer)), remaining)
		n, readErr := io.ReadFull(f, buffer[:want])
		if readErr != nil {
			reason = "source_changed"
			panic(http.ErrAbortHandler)
		}
		if int64(n) == remaining && !validate() {
			reason = "source_changed"
			panic(http.ErrAbortHandler)
		}
		wrote, writeErr := w.Write(buffer[:n])
		sent += int64(wrote)
		remaining -= int64(wrote)
		if writeErr != nil || wrote != n {
			reason = "cancelled"
			panic(http.ErrAbortHandler)
		}
	}
}
func safeDownloadExtension(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	if _, ok := videoMIME[extension]; ok {
		return extension
	}
	return ".bin"
}
