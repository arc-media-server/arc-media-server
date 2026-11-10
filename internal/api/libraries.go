package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"ams/internal/backup"
	"ams/internal/config"
	"ams/internal/metadata"
	"ams/internal/scanner"
	"ams/internal/store"
)

type libraryBody struct {
	DetailsProvider string   `json:"details_provider"`
	Name            string   `json:"name"`
	Kind            string   `json:"kind"`
	SourceOrdering  string   `json:"source_ordering"`
	Paths           []string `json:"paths"`
	// Make timeline thumbnails for scrubbing (off by default).
	Trickplay bool `json:"trickplay"`
	// The fewest films a TMDB collection needs to be shown (default 2; 0 for none).
	CollectionMin *int `json:"collection_min"`
	// The language titles and descriptions are fetched in; "" for English.
	MetadataLanguage string `json:"metadata_language"`
	// The country whose age ratings are used (e.g. "GB"); "" for the server's region.
	RatingRegion string `json:"rating_region"`
	// Offer Skip Intro and Skip Credits (on by default where they apply).
	SkipIntros  *bool `json:"skip_intros"`
	SkipCredits *bool `json:"skip_credits"`
	// How collections show in the grid by default: "" their titles, "collection" or "both".
	CollectionGrid string `json:"collection_grid"`
}

// validate cleans up a library description and checks it. exceptID is the library being edited
// (its own folders don't conflict with themselves).
func (s *Server) validate(ctx context.Context, b *libraryBody, exceptID int64) error {
	b.Name = strings.TrimSpace(b.Name)
	if b.CollectionMin != nil && (*b.CollectionMin < 0 || *b.CollectionMin > 10) {
		return errMsg("collection_min is 0 (off) to 10")
	}
	if !languageRe.MatchString(b.MetadataLanguage) {
		return errMsg(`metadata_language is a language like "fr-FR" or "ja", or "" for English`)
	}
	if !slices.Contains([]string{"", "collection", "both"}, b.CollectionGrid) {
		return errMsg(`collection_grid is "", "collection" or "both"`)
	}
	if b.RatingRegion != "" && !regionRe.MatchString(b.RatingRegion) {
		return fmt.Errorf("rating_region is a two-letter country code such as GB")
	}
	if b.DetailsProvider != "" && (b.DetailsProvider != "tvdb" || b.Kind != "shows") {
		return errors.New("TVDB metadata provider is only available for TV libraries")
	}
	if b.Kind != "movies" && b.Kind != "shows" && b.Kind != "videos" {
		return errors.New("kind must be movies, shows or videos")
	}
	if b.SourceOrdering == "" {
		b.SourceOrdering = "tmdb_aired" // TMDB by default, as Plex does
	}
	if !config.ValidSourceOrdering(b.SourceOrdering) {
		return fmt.Errorf("unknown source_ordering %q", b.SourceOrdering)
	}
	if len(b.Paths) == 0 {
		return errors.New("add at least one folder")
	}
	seen := map[string]bool{}
	var paths []string
	for _, p := range b.Paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("can't open %s", abs)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s isn't a folder", abs)
		}
		// Check directory access without walking the media or doing metadata work.
		folder, err := os.Open(abs)
		if err != nil {
			return fmt.Errorf("can't read %s", abs)
		}
		_, readErr := folder.Readdirnames(1)
		folder.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return fmt.Errorf("can't read %s", abs)
		}
		if key := strings.ToLower(abs); !seen[key] {
			seen[key] = true
			paths = append(paths, abs)
		}
	}
	if len(paths) == 0 {
		return errors.New("add at least one folder")
	}
	for i, a := range paths {
		for _, c := range paths[i+1:] {
			if within(a, c) || within(c, a) {
				return fmt.Errorf("%s and %s overlap; choose just one of them", a, c)
			}
		}
	}
	libs, err := s.store.ListLibraries(ctx)
	if err != nil {
		return err
	}
	for _, l := range libs {
		if l.ID == exceptID {
			continue
		}
		for _, lp := range l.Paths {
			for _, p := range paths {
				if within(p, lp) || within(lp, p) {
					return fmt.Errorf("%s is already part of the %q library", p, l.Name)
				}
			}
		}
	}
	b.Paths = paths
	return nil
}

// within reports whether path is folder or inside it (comparing case-insensitively, as Windows does).
func within(path, folder string) bool {
	path, folder = strings.ToLower(filepath.Clean(path)), strings.ToLower(filepath.Clean(folder))
	return path == folder || strings.HasPrefix(path, strings.TrimSuffix(folder, string(filepath.Separator))+string(filepath.Separator))
}

// createLibrary: POST /api/libraries {"name", "kind", "paths", "source_ordering"} adds a library
// and starts scanning it.
func (s *Server) createLibrary(w http.ResponseWriter, r *http.Request) {
	var b libraryBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := s.validate(r.Context(), &b, 0); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if b.Name == "" {
		writeError(w, http.StatusBadRequest, "give the library a name")
		return
	}
	// Compute every option before the one store transaction; no worker sees a partial library.
	lib := store.Library{Name: b.Name, Kind: b.Kind, SourceOrdering: b.SourceOrdering, Paths: b.Paths, DetailsProvider: b.DetailsProvider, Trickplay: b.Trickplay, CollectionMin: 2, MetadataLanguage: b.MetadataLanguage, RatingRegion: b.RatingRegion, SkipIntros: b.Kind == "shows", SkipCredits: b.Kind != "videos", CollectionGrid: b.CollectionGrid}
	if b.CollectionMin != nil {
		lib.CollectionMin = *b.CollectionMin
	}
	if lib.RatingRegion == "" {
		lib.RatingRegion = s.settings().Region
	}
	if b.SkipIntros != nil {
		lib.SkipIntros = *b.SkipIntros && b.Kind == "shows"
	}
	if b.SkipCredits != nil {
		lib.SkipCredits = *b.SkipCredits && b.Kind != "videos"
	}
	id, err := s.store.CreateConfiguredLibrary(r.Context(), lib)
	if errors.Is(err, store.ErrDuplicateName) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, store.ErrLibraryPathConflict) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	lib.ID = id
	s.log.Info("library added", "library", id, "kind", lib.Kind, "folders", len(lib.Paths))
	if lib.Trickplay && s.trickplay != nil {
		s.trickplay.Wake()
	}
	if s.librariesChanged != nil {
		s.librariesChanged()
	}
	s.scanner.Start(id)
	// Match GetLibrary's stable folder ordering without another post-commit DB read.
	sort.Strings(lib.Paths)
	writeJSON(w, http.StatusCreated, lib)
}

// setTrickplay turns a library's timeline thumbnails on (they're made in the background) or off
// (those already made are deleted to free the space).
func (s *Server) setTrickplay(ctx context.Context, libraryID int64, on bool) error {
	if err := s.store.SetLibraryTrickplay(ctx, libraryID, on); err != nil {
		return err
	}
	if s.trickplay == nil {
		return nil
	}
	if on {
		s.trickplay.Wake()
		return nil
	}
	ids, err := s.store.LibraryFileIDs(ctx, libraryID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.trickplay.Forget(id)
	}
	return nil
}

// updateLibrary: PATCH /api/libraries/{id} {"name", "paths", "source_ordering", "trickplay"} changes a library
// (not its kind) and rescans it.
func (s *Server) updateLibrary(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	b := libraryBody{Name: lib.Name, Kind: lib.Kind, SourceOrdering: lib.SourceOrdering, Paths: lib.Paths, DetailsProvider: lib.DetailsProvider}
	var patch struct {
		DetailsProvider *string  `json:"details_provider"`
		Name            *string  `json:"name"`
		SourceOrdering  *string  `json:"source_ordering"`
		Paths           []string `json:"paths"`
		Trickplay       *bool    `json:"trickplay"`
		CollectionMin   *int     `json:"collection_min"`
		Language        *string  `json:"metadata_language"`
		RatingRegion    *string  `json:"rating_region"`
		SkipIntros      *bool    `json:"skip_intros"`
		SkipCredits     *bool    `json:"skip_credits"`
		CollectionGrid  *string  `json:"collection_grid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if patch.DetailsProvider != nil {
		if *patch.DetailsProvider != "" && (*patch.DetailsProvider != "tvdb" || lib.Kind != "shows") {
			writeError(w, 400, "TVDB metadata provider is only available for TV libraries")
			return
		}

	}
	if patch.Trickplay != nil && *patch.Trickplay != lib.Trickplay {
		if err := s.setTrickplay(r.Context(), lib.ID, *patch.Trickplay); err != nil {
			s.fail(w, err)
			return
		}
	}
	if patch.CollectionGrid != nil {
		if *patch.CollectionGrid != "" && *patch.CollectionGrid != "collection" && *patch.CollectionGrid != "both" {
			writeError(w, http.StatusBadRequest, `collection_grid is "" (titles), "collection" or "both"`)
			return
		}
		if err := s.store.SetLibraryCollectionGrid(r.Context(), lib.ID, *patch.CollectionGrid); err != nil {
			s.fail(w, err)
			return
		}
	}
	if patch.SkipIntros != nil || patch.SkipCredits != nil {
		intros, credits := lib.SkipIntros, lib.SkipCredits
		if patch.SkipIntros != nil {
			intros = *patch.SkipIntros && lib.Kind == "shows"
		}
		if patch.SkipCredits != nil {
			credits = *patch.SkipCredits && lib.Kind != "videos"
		}
		if err := s.store.SetLibrarySkips(r.Context(), lib.ID, intros, credits); err != nil {
			s.fail(w, err)
			return
		}
		if s.markers != nil && (intros && !lib.SkipIntros || credits && !lib.SkipCredits) {
			s.markers.Wake() // look for the newly wanted ones
		}
	}
	if patch.CollectionMin != nil {
		if *patch.CollectionMin < 0 || *patch.CollectionMin > 10 {
			writeError(w, http.StatusBadRequest, "collection_min is 0 (off) to 10")
			return
		}
		if err := s.store.SetLibraryCollectionMin(r.Context(), lib.ID, *patch.CollectionMin); err != nil {
			s.fail(w, err)
			return
		}
	}
	if patch.Language != nil && *patch.Language != lib.MetadataLanguage {
		if !languageRe.MatchString(*patch.Language) {
			writeError(w, http.StatusBadRequest, `metadata_language is a language like "fr-FR" or "ja", or "" for English`)
			return
		}
		if err := s.store.SetLibraryLanguage(r.Context(), lib.ID, *patch.Language); err != nil {
			s.fail(w, err)
			return
		}
		// Fetch everything again in the new language.
		s.scanner.Request(lib.ID)
	}
	// A new age-rating country refetches the library's titles (after the rest of the save, which
	// refuses a busy library); if it is busy anyway, the maintenance refresh picks them up.
	regionChanged := patch.RatingRegion != nil && *patch.RatingRegion != lib.RatingRegion && lib.Kind != "videos"
	if patch.RatingRegion != nil && *patch.RatingRegion != lib.RatingRegion {
		if !regionRe.MatchString(*patch.RatingRegion) {
			writeError(w, http.StatusBadRequest, "rating_region is a two-letter country code such as GB")
			return
		}
		if err := s.store.SetLibraryRatingRegion(r.Context(), lib.ID, *patch.RatingRegion); err != nil {
			s.fail(w, err)
			return
		}
	}
	if patch.Name == nil && patch.SourceOrdering == nil && patch.Paths == nil {
		if patch.DetailsProvider != nil {
			if s.scanner.Status(lib.ID).Running {
				writeError(w, 409, "the library is busy; retry when its current work finishes")
				return
			}
			if err := s.store.SetLibraryDetailsProvider(r.Context(), lib.ID, *patch.DetailsProvider); err != nil {
				s.fail(w, err)
				return
			}
		}
		// Provider-only changes refresh known metadata, never discover media.
		if patch.DetailsProvider != nil && *patch.DetailsProvider != lib.DetailsProvider || regionChanged {
			s.scanner.StartOperation(lib.ID, "refresh")
		}
		// Nothing that needs a rescan.
		updated, err := s.store.GetLibrary(r.Context(), lib.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}
	if patch.Name != nil {
		b.Name = *patch.Name
	}
	if patch.SourceOrdering != nil {
		b.SourceOrdering = *patch.SourceOrdering
	}
	if patch.Paths != nil {
		b.Paths = patch.Paths
	}
	if err := s.validate(r.Context(), &b, lib.ID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if b.Name == "" {
		writeError(w, http.StatusBadRequest, "give the library a name")
		return
	}
	if s.scanner.Status(lib.ID).Running {
		writeError(w, http.StatusConflict, "the library is being scanned; try again when it finishes")
		return
	}
	var providers []string
	if patch.DetailsProvider != nil {
		providers = []string{*patch.DetailsProvider}
	}
	err := s.store.UpdateLibrary(r.Context(), lib.ID, b.Name, b.SourceOrdering, b.Paths, providers...)
	if errors.Is(err, store.ErrDuplicateName) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	if s.librariesChanged != nil {
		s.librariesChanged()
	}
	if !slices.Equal(lib.Paths, b.Paths) {
		s.scanner.Request(lib.ID)
	} else if b.SourceOrdering != lib.SourceOrdering || patch.DetailsProvider != nil && *patch.DetailsProvider != lib.DetailsProvider || regionChanged {
		s.scanner.StartOperation(lib.ID, "refresh")
	}
	updated, err := s.store.GetLibrary(r.Context(), lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

// deleteLibrary: DELETE /api/libraries/{id} removes a library and everything AMS knows about it,
// after backing up the database. Media files aren't touched.
func (s *Server) deleteLibrary(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	if s.scanner.Status(lib.ID).Running {
		writeError(w, http.StatusConflict, "the library is being scanned; try again when it finishes")
		return
	}
	if _, err := s.backups.Create(r.Context(), backup.ReasonBeforeDelete); err != nil {
		s.log.Warn("backup before deleting library failed", "err", err)
		writeError(w, http.StatusInternalServerError, "couldn't back up first, so nothing was removed: "+err.Error())
		return
	}
	if err := s.store.DeleteLibrary(r.Context(), lib.ID); err != nil {
		s.fail(w, err)
		return
	}
	s.log.Info("library removed", "library", lib.Name)
	if s.librariesChanged != nil {
		s.librariesChanged()
	}
	w.WriteHeader(http.StatusNoContent)
}

// previews keeps the one library preview being looked at. Starting another replaces it.
type previews struct {
	mu     sync.Mutex
	id     string
	state  *scanner.PreviewState
	cancel context.CancelFunc
}

// startPreview: POST /api/library-previews {"kind", "paths", "source_ordering"} starts working out
// what a library with these folders would contain. Poll GET /api/library-previews/{id}.
func (s *Server) startPreview(w http.ResponseWriter, r *http.Request) {
	var b libraryBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// Previewing a library's own folders while editing it is fine; pass its id to allow that.
	var except int64
	fmt.Sscan(r.URL.Query().Get("library"), &except)
	if err := s.validate(r.Context(), &b, except); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	region := b.RatingRegion
	if region == "" {
		region = s.settings().Region
	}
	ctx = metadata.WithRegion(metadata.WithLanguage(ctx, b.MetadataLanguage), region)
	ps := &scanner.PreviewState{}
	p := s.previews
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	p.id, p.state, p.cancel = store.NewID(), ps, cancel
	id := p.id
	p.mu.Unlock()

	go func() {
		defer cancel()
		s.scanner.Preview(ctx, b.Kind, b.SourceOrdering, b.Paths, ps)
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"id": id, "preview": ps.Snapshot()})
}

// getPreview: GET /api/library-previews/{id}. The preview's progress and results.
func (s *Server) getPreview(w http.ResponseWriter, r *http.Request) {
	p := s.previews
	p.mu.Lock()
	ok := p.id == r.PathValue("id") && p.state != nil
	state := p.state
	p.mu.Unlock()
	if !ok {
		writeError(w, http.StatusNotFound, "this preview has finished or was replaced; start another")
		return
	}
	writeJSON(w, http.StatusOK, state.Snapshot())
}

// listFolders: GET /api/folders?path=C:\Media lists the folders inside a folder, to choose library
// folders from. With no path it lists the drives (Windows) or a few starting points.
func (s *Server) listFolders(w http.ResponseWriter, r *http.Request) {
	type folder struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		var roots []folder
		if runtime.GOOS == "windows" {
			for c := 'A'; c <= 'Z'; c++ {
				d := string(c) + `:\`
				if _, err := os.Stat(d); err == nil {
					roots = append(roots, folder{Name: string(c) + ":", Path: d})
				}
			}
		} else {
			roots = append(roots, folder{Name: "/", Path: "/"})
			for _, p := range []string{"/mnt", "/media", "/srv", "/Volumes"} {
				if info, err := os.Stat(p); err == nil && info.IsDir() {
					roots = append(roots, folder{Name: p, Path: p})
				}
			}
		}
		if home, err := os.UserHomeDir(); err == nil {
			roots = append(roots, folder{Name: "Home", Path: home})
		}
		writeJSON(w, http.StatusOK, map[string]any{"path": "", "parent": nil, "folders": roots})
		return
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		writeError(w, http.StatusNotFound, "can't open "+abs)
		return
	}
	out := []folder{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") || strings.EqualFold(name, "System Volume Information") {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(filepath.Join(abs, name)); err == nil {
				isDir = info.IsDir()
			}
		}
		if isDir {
			out = append(out, folder{Name: name, Path: filepath.Join(abs, name)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	var parent any
	if p := filepath.Dir(abs); p != abs {
		parent = p
	} else {
		parent = "" // the top of a drive: go back to the list of drives
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": abs, "parent": parent, "folders": out})
}

// languageRe is a TMDB language: "fr", "fr-FR" or "pt-BR", or "" for English.
var languageRe = regexp.MustCompile(`^([a-z]{2}(-[A-Z]{2})?)?$`)
