// Package api serves AMS's native REST API.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ams/internal/artwork"
	"ams/internal/backup"
	"ams/internal/scanner"
	"ams/internal/store"
	"ams/internal/webui"
)

// Deps is what the API needs from the rest of AMS.
type Deps struct {
	Store   *store.Store
	Scanner *scanner.Scanner
	Artwork *artwork.Cache
	Backups *backup.Manager
	// AfterRestore runs once a backup has been restored, to bring libraries back in step.
	AfterRestore func()
	Log          *slog.Logger
	Version      string
}

type Server struct {
	store        *store.Store
	scanner      *scanner.Scanner
	artwork      *artwork.Cache
	backups      *backup.Manager
	afterRestore func()
	log          *slog.Logger
	version      string
}

func New(d Deps) http.Handler {
	s := &Server{store: d.Store, scanner: d.Scanner, artwork: d.Artwork, backups: d.Backups,
		afterRestore: d.AfterRestore, log: d.Log, version: d.Version}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/backups", s.listBackups)
	mux.HandleFunc("POST /api/backups", s.createBackup)
	mux.HandleFunc("POST /api/backups/{name}/restore", s.restoreBackup)
	mux.HandleFunc("DELETE /api/backups/{name}", s.deleteBackup)
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/libraries", s.listLibraries)
	mux.HandleFunc("GET /api/libraries/{id}/scan", s.scanStatus)
	mux.HandleFunc("POST /api/libraries/{id}/scan", s.startScan)
	mux.HandleFunc("GET /api/items", s.listItems)
	mux.HandleFunc("GET /api/items/{id}", s.getItem)
	mux.HandleFunc("PATCH /api/items/{id}", s.editItem)
	mux.HandleFunc("POST /api/items/{id}/refresh", s.refreshItem)
	mux.HandleFunc("GET /api/items/{id}/match", s.matchInfo)
	mux.HandleFunc("GET /api/items/{id}/match/search", s.searchMatches)
	mux.HandleFunc("PUT /api/items/{id}/match", s.setMatch)
	mux.HandleFunc("GET /api/items/{id}/stream", s.stream)
	mux.HandleFunc("GET /api/items/{id}/images/{type}", s.image)
	mux.HandleFunc("GET /api/libraries/{id}/unmatched", s.unmatched)
	mux.HandleFunc("GET /api/libraries/{id}/health", s.libraryHealth)
	mux.HandleFunc("GET /api/health/libraries", s.healthSummary)
	mux.HandleFunc("GET /api/series/{id}/seasons", s.seasons)
	mux.HandleFunc("GET /api/series/{id}/orderings", s.listOrderings)
	mux.HandleFunc("POST /api/series/{id}/orderings", s.createOrdering)
	mux.HandleFunc("DELETE /api/series/{id}/orderings/{oid}", s.deleteOrdering)
	mux.HandleFunc("PUT /api/series/{id}/orderings/{oid}/entries/{eid}", s.setEntry)
	mux.HandleFunc("DELETE /api/series/{id}/orderings/{oid}/entries/{eid}", s.clearEntry)
	mux.HandleFunc("PUT /api/series/{id}/display-ordering", s.setDisplayOrdering)
	mux.HandleFunc("POST /api/series/{id}/refresh", s.refreshSeries)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such endpoint")
	})
	mux.Handle("/", webui.Handler())
	return s.logRequests(mux)
}

// itemDTO adds links and files to a stored item.
type itemDTO struct {
	store.Item
	// For episodes, where the caller asks for it: the series' title.
	SeriesTitle string            `json:"series_title,omitempty"`
	Images      map[string]string `json:"images,omitempty"`
	Stream string            `json:"stream_url,omitempty"`
	Files  []store.MediaFile `json:"files,omitempty"`
}

func toDTO(it store.Item) itemDTO {
	d := itemDTO{Item: it, Images: map[string]string{}}
	base := "/api/items/" + it.ID
	// The version changes with the image, so each link can be cached by browsers forever.
	if it.PosterPath != "" {
		d.Images["primary"] = base + "/images/primary?v=" + artwork.Key(artwork.Source(it.Kind, "primary", it.PosterPath))
	}
	if it.BackdropPath != "" {
		d.Images["backdrop"] = base + "/images/backdrop?v=" + artwork.Key(artwork.Source(it.Kind, "backdrop", it.BackdropPath))
	}
	if it.Kind == "movie" || it.Kind == "episode" {
		d.Stream = base + "/stream"
	}
	return d
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"name": "Arc Media Server", "version": s.version, "status": "ok"})
}

func (s *Server) listLibraries(w http.ResponseWriter, r *http.Request) {
	libs, err := s.store.ListLibraries(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	type libDTO struct {
		store.Library
		Scan scanner.Status `json:"scan"`
	}
	out := make([]libDTO, 0, len(libs))
	for _, l := range libs {
		out = append(out, libDTO{Library: l, Scan: s.scanner.Status(l.ID)})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) libraryFromPath(w http.ResponseWriter, r *http.Request) (*store.Library, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid library id")
		return nil, false
	}
	lib, err := s.store.GetLibrary(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	if lib == nil {
		writeError(w, http.StatusNotFound, "library not found")
		return nil, false
	}
	return lib, true
}

func (s *Server) scanStatus(w http.ResponseWriter, r *http.Request) {
	if lib, ok := s.libraryFromPath(w, r); ok {
		writeJSON(w, http.StatusOK, s.scanner.Status(lib.ID))
	}
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	if !s.scanner.Start(lib.ID) {
		writeError(w, http.StatusConflict, "a scan is already running for this library")
		return
	}
	writeJSON(w, http.StatusAccepted, s.scanner.Status(lib.ID))
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := store.ItemQuery{
		Kind:   q.Get("kind"),
		Search: strings.TrimSpace(q.Get("q")),
		Sort:   q.Get("sort"),
	}
	query.LibraryID, _ = strconv.ParseInt(q.Get("library_id"), 10, 64)
	query.Limit, _ = strconv.Atoi(q.Get("limit"))
	query.Offset, _ = strconv.Atoi(q.Get("offset"))

	items, total, err := s.store.ListItems(r.Context(), query)
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]itemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, toDTO(it))
	}
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "items": out})
}

func (s *Server) itemFromPath(w http.ResponseWriter, r *http.Request) (*store.Item, bool) {
	it, err := s.store.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		s.fail(w, err)
		return nil, false
	}
	if it == nil {
		writeError(w, http.StatusNotFound, "item not found")
		return nil, false
	}
	return it, true
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	files, err := s.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	d := toDTO(*it)
	d.Files = files
	writeJSON(w, http.StatusOK, d)
}

var videoMIME = map[string]string{
	".mp4": "video/mp4", ".m4v": "video/x-m4v", ".mkv": "video/x-matroska", ".webm": "video/webm",
	".avi": "video/x-msvideo", ".mov": "video/quicktime", ".ts": "video/mp2t", ".m2ts": "video/mp2t",
	".wmv": "video/x-ms-wmv", ".mpg": "video/mpeg", ".mpeg": "video/mpeg", ".flv": "video/x-flv",
}

// stream serves the original file with HTTP range support (direct play). ?file=<id> selects a
// specific part or version; otherwise the first available file is used.
func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	files, err := s.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	wantID, _ := strconv.ParseInt(r.URL.Query().Get("file"), 10, 64)
	var chosen *store.MediaFile
	for i := range files {
		f := &files[i]
		if f.Missing || (wantID != 0 && f.ID != wantID) {
			continue
		}
		chosen = f
		break
	}
	if chosen == nil {
		writeError(w, http.StatusNotFound, "no playable file for this item")
		return
	}

	fh, err := os.Open(chosen.Path)
	if err != nil {
		s.log.Warn("cannot open media file", "path", chosen.Path, "err", err)
		writeError(w, http.StatusNotFound, "media file unavailable")
		return
	}
	defer fh.Close()
	info, err := fh.Stat()
	if err != nil {
		s.fail(w, err)
		return
	}
	if mime := videoMIME[strings.ToLower(filepath.Ext(chosen.Path))]; mime != "" {
		w.Header().Set("Content-Type", mime)
	}
	http.ServeContent(w, r, filepath.Base(chosen.Path), info.ModTime(), fh)
}

// image serves an item's artwork from the local cache, downloading it on first use. If it can't
// be downloaded (e.g. the provider is unreachable) the browser is sent to the provider instead.
func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	var imagePath string
	switch typ := r.PathValue("type"); typ {
	case "primary":
		imagePath = it.PosterPath
	case "backdrop":
		imagePath = it.BackdropPath
	default:
		writeError(w, http.StatusBadRequest, "unknown image type")
		return
	}
	if imagePath == "" {
		writeError(w, http.StatusNotFound, "no image")
		return
	}
	src := artwork.Source(it.Kind, r.PathValue("type"), imagePath)
	f, err := s.artwork.Open(r.Context(), src)
	if err != nil {
		s.log.Debug("artwork unavailable locally", "url", src, "err", err)
		http.Redirect(w, r, src, http.StatusFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		s.fail(w, err)
		return
	}
	key := artwork.Key(src)
	if r.URL.Query().Get("v") == key {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("ETag", `"`+key+`"`)
	http.ServeContent(w, r, artwork.ContentName(src), info.ModTime(), f)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("request failed", "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"took", time.Since(start).Round(time.Millisecond))
	})
}
