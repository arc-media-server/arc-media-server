package api

import (
	"ams/internal/store"
	"context"
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
	ID        string `json:"id"`
	Role      string `json:"role"`
	URL       string `json:"url"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}
type downloadItemSnapshot struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
}
type downloadManifest struct {
	DownloadID string               `json:"download_id"`
	Revision   string               `json:"revision"`
	ServerID   string               `json:"server_id"`
	Mode       string               `json:"mode"`
	Item       downloadItemSnapshot `json:"item"`
	Assets     []downloadAssetDTO   `json:"assets"`
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
	item, err := s.store.GetItem(r.Context(), d.ItemID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if item == nil {
		writeError(w, 404, "download unavailable")
		return
	}
	server, err := s.store.Setting(r.Context(), "server_id")
	if err != nil {
		s.fail(w, err)
		return
	}
	manifest := downloadManifest{DownloadID: d.ID, Revision: d.Revision, ServerID: server, Mode: "original", Item: downloadItemSnapshot{ID: item.ID, Kind: item.Kind, Title: item.Title}, Assets: []downloadAssetDTO{{ID: d.Revision, Role: "media", URL: "/api/v1/downloads/" + url.PathEscape(d.ID) + "/assets/" + url.PathEscape(d.Revision), MediaType: downloadMediaType(d.SourcePath), Size: d.Size, SHA256: d.SHA256}}}
	writeJSON(w, http.StatusOK, manifest)
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
	if r.PathValue("asset") != d.Revision {
		writeError(w, 404, "download unavailable")
		return
	}
	f, opened, err := openDownloadSource(d)
	if err != nil {
		writeError(w, 409, "download source changed; prepare again")
		return
	}
	defer f.Close()
	rangeHeader := r.Header.Get("Range")
	if r.Method == "HEAD" || r.Header.Get("If-Range") != "" {
		rangeHeader = ""
	}
	selected, err := parseDownloadRange(rangeHeader, d.Size)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", d.Size))
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
	w.Header().Set("Content-Type", downloadMediaType(d.SourcePath))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(selected.length, 10))
	// A fixed safe name avoids leaking the source filename or trusting title/header characters.
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "arc-download" + safeDownloadExtension(d.SourcePath)}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	status := 200
	if rangeHeader != "" {
		status = 206
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", selected.start, selected.start+selected.length-1, d.Size))
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
		current, err := f.Stat()
		if err != nil || !downloadSourceInfo(d, current) || !os.SameFile(opened, current) {
			return false
		}
		path, err := os.Stat(d.SourcePath)
		return err == nil && downloadSourceInfo(d, path) && os.SameFile(opened, path)
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
