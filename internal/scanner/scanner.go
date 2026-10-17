// Package scanner walks library folders, records what it finds and matches it to metadata.
// It never writes to, renames or moves media files.
package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/store"
)

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".wmv": true,
	".ts": true, ".m2ts": true, ".webm": true, ".mpg": true, ".mpeg": true, ".flv": true,
}

// Folders holding bonus material rather than the main feature.
var skipDirs = map[string]bool{
	"extras": true, "featurettes": true, "behind the scenes": true, "deleted scenes": true,
	"interviews": true, "scenes": true, "shorts": true, "trailers": true, "samples": true,
	"sample": true, "other": true, "@eadir": true, "$recycle.bin": true,
}

type Status struct {
	Running     bool      `json:"running"`
	StartedAt   time.Time `json:"started_at,omitzero"`
	FinishedAt  time.Time `json:"finished_at,omitzero"`
	FilesSeen   int       `json:"files_seen"`
	FilesProbed int       `json:"files_probed"`
	NewItems    int       `json:"new_items"`
	Missing     int64     `json:"files_missing"`
	Matched     int       `json:"matched"`
	Unmatched   int       `json:"unmatched"`
	// Video files in a TV library that couldn't be read as an episode.
	Unrecognised int    `json:"not_recognised"`
	Error        string `json:"error,omitempty"`
}

type Scanner struct {
	ctx    context.Context
	store  *store.Store
	prober *probe.Prober
	tmdb   *metadata.TMDB // check Configured() before use: there may be no API key
	tvdb   *metadata.TVDB
	log    *slog.Logger

	// OnFinish, if set, is called after each scan (successful or not). Set it before starting scans.
	OnFinish func(libraryID int64)

	wg     sync.WaitGroup
	mu     sync.Mutex
	status map[int64]*Status
	// Libraries that changed during their scan and need scanning again when it ends.
	again map[int64]bool
}

// New creates a scanner. Background scans stop when ctx is cancelled.
func New(ctx context.Context, st *store.Store, prober *probe.Prober, tmdb *metadata.TMDB, tvdb *metadata.TVDB, log *slog.Logger) *Scanner {
	return &Scanner{ctx: ctx, store: st, prober: prober, tmdb: tmdb, tvdb: tvdb, log: log,
		status: map[int64]*Status{}, again: map[int64]bool{}}
}

// Start begins a background scan of a library. It returns false if one is already running.
func (s *Scanner) Start(libraryID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startLocked(libraryID)
}

// Request scans a library soon: now, or straight after the scan already running, so changes made
// during that scan aren't missed.
func (s *Scanner) Request(libraryID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.startLocked(libraryID) {
		s.again[libraryID] = true
	}
}

func (s *Scanner) startLocked(libraryID int64) bool {
	if st := s.status[libraryID]; st != nil && st.Running {
		return false
	}
	if s.ctx.Err() != nil {
		return false
	}
	s.status[libraryID] = &Status{Running: true, StartedAt: time.Now()}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := s.scan(s.ctx, libraryID)
		s.update(libraryID, func(st *Status) {
			st.Running = false
			st.FinishedAt = time.Now()
			if err != nil {
				st.Error = err.Error()
			}
		})
		if err != nil {
			s.log.Error("scan failed", "library", libraryID, "err", err)
		}
		if s.OnFinish != nil {
			s.OnFinish(libraryID)
		}
		s.mu.Lock()
		if s.again[libraryID] {
			delete(s.again, libraryID)
			s.startLocked(libraryID)
		}
		s.mu.Unlock()
	}()
	return true
}

// Running reports whether any library is being scanned.
func (s *Scanner) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.status {
		if st.Running {
			return true
		}
	}
	return false
}

// Wait blocks until all running scans have stopped.
func (s *Scanner) Wait() { s.wg.Wait() }

func (s *Scanner) Status(libraryID int64) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st := s.status[libraryID]; st != nil {
		return *st
	}
	return Status{}
}

func (s *Scanner) update(libraryID int64, fn func(*Status)) {
	s.mu.Lock()
	fn(s.status[libraryID])
	s.mu.Unlock()
}

func (s *Scanner) scan(ctx context.Context, libraryID int64) error {
	lib, err := s.store.GetLibrary(ctx, libraryID)
	if err != nil {
		return err
	}
	if lib == nil {
		return fmt.Errorf("library %d not found", libraryID)
	}
	log := s.log.With("library", lib.Name)
	log.Info("scan started", "paths", lib.Paths)
	scanStart := time.Now().UnixMilli()
	allRootsOK := true

	var addFile func(root, path string, d fs.DirEntry) error
	series := map[string]*seriesGroup{}
	switch lib.Kind {
	case "movies":
		addFile = func(root, path string, d fs.DirEntry) error {
			return s.scanMovieFile(ctx, lib, path, d, scanStart)
		}
	case "shows":
		addFile = func(root, path string, d fs.DirEntry) error {
			return s.collectEpisodeFile(ctx, lib, root, path, d, scanStart, series)
		}
	default:
		return fmt.Errorf("unknown library kind %q", lib.Kind)
	}

	for _, root := range lib.Paths {
		complete, err := walkVideos(ctx, root, log, func(path string, d fs.DirEntry) {
			if err := addFile(root, path, d); err != nil {
				log.Warn("failed to add file", "path", path, "err", err)
			}
		}, nil)
		if err != nil {
			return err
		}
		// Don't mark anything missing if a drive is offline; that would hide the whole library.
		allRootsOK = allRootsOK && complete
	}

	if allRootsOK {
		n, err := s.store.MarkMissing(ctx, lib.ID, scanStart, time.Now().Unix())
		if err != nil {
			return err
		}
		s.update(lib.ID, func(st *Status) { st.Missing = n })
	}

	switch lib.Kind {
	case "movies":
		if s.tmdb.Configured() {
			if err := s.matchMovies(ctx, lib.ID); err != nil {
				return fmt.Errorf("metadata: %w", err)
			}
			if err := s.refreshUnsyncedMovies(ctx, lib.ID); err != nil {
				return fmt.Errorf("metadata: %w", err)
			}
		}
	case "shows":
		if err := s.syncSeriesGroups(ctx, lib, series, log); err != nil {
			return err
		}
	}

	st := s.Status(lib.ID)
	log.Info("scan finished", "files", st.FilesSeen, "probed", st.FilesProbed, "new", st.NewItems,
		"missing", st.Missing, "matched", st.Matched, "unmatched", st.Unmatched, "not_recognised", st.Unrecognised)
	return nil
}

// walkVideos calls visit for every main video file below root, skipping hidden folders, extras,
// trailers and samples (bonus is called for each of those, if set). complete is false when root or
// part of it couldn't be read, e.g. a drive is offline.
func walkVideos(ctx context.Context, root string, log *slog.Logger, visit func(path string, d fs.DirEntry), bonus func(path string)) (complete bool, err error) {
	if _, err := os.Stat(root); err != nil {
		log.Warn("library folder unavailable", "path", root, "err", err)
		return false, nil
	}
	complete = true
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Warn("cannot read", "path", path, "err", err)
			if d != nil && d.IsDir() {
				complete = false
				return fs.SkipDir
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if path != root && skipDirs[strings.ToLower(name)] {
				if bonus != nil {
					bonus(path)
				}
				return fs.SkipDir
			}
			return nil
		}
		if !videoExts[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		if isBonusFile(name) {
			if bonus != nil {
				bonus(path)
			}
			return nil
		}
		visit(path, d)
		return nil
	})
	return complete, err
}

// recordFile stores a file (probing it if it is new or changed) and marks it seen in this scan.
func (s *Scanner) recordFile(ctx context.Context, lib *store.Library, path string, d fs.DirEntry, scanStart int64) (int64, error) {
	info, err := d.Info()
	if err != nil {
		return 0, err
	}
	s.update(lib.ID, func(st *Status) { st.FilesSeen++ })

	existing, err := s.store.GetFileByPath(ctx, path)
	if err != nil {
		return 0, err
	}
	if existing != nil && existing.Probed && existing.Size == info.Size() && existing.ModTime == info.ModTime().Unix() {
		return existing.ID, s.store.TouchFile(ctx, existing.ID, scanStart)
	}
	f := &store.MediaFile{LibraryID: lib.ID, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}
	var probedAt int64
	if res, err := s.prober.Probe(ctx, path); err != nil {
		f.ProbeError = err.Error()
	} else {
		probedAt = time.Now().Unix()
		applyProbe(f, res)
	}
	fileID, err := s.store.SaveFile(ctx, f, probedAt, scanStart)
	if err != nil {
		return 0, err
	}
	s.update(lib.ID, func(st *Status) { st.FilesProbed++ })
	return fileID, nil
}

func (s *Scanner) scanMovieFile(ctx context.Context, lib *store.Library, path string, d fs.DirEntry, scanStart int64) error {
	fileID, err := s.recordFile(ctx, lib, path, d, scanStart)
	if err != nil {
		return err
	}

	// Parse every time (it's cheap) so naming fixes and parser improvements take effect.
	name := ParseMovie(path)
	matchKey := "title:" + metadata.Normalize(name.Title) + ":" + strconv.Itoa(name.Year)
	if name.TMDBID != 0 {
		matchKey = "tmdb:" + strconv.FormatInt(name.TMDBID, 10)
	} else if name.IMDBID != "" {
		matchKey = "imdb:" + name.IMDBID
	}
	itemID, created, err := s.store.EnsureItem(ctx, &store.Item{
		LibraryID: lib.ID, Kind: "movie", Title: name.Title, Year: name.Year,
		TMDBID: name.TMDBID, IMDBID: name.IMDBID,
	}, matchKey)
	if err != nil {
		return err
	}
	if created {
		s.update(lib.ID, func(st *Status) { st.NewItems++ })
	}
	if err := s.store.SetFileItems(ctx, fileID, []string{itemID}, name.Part); err != nil {
		return err
	}

	// Local artwork and .nfo ids. Folder-wide files (poster.jpg, movie.nfo) only count when the
	// folder is the movie's own, not a shared folder of many movies.
	folder := parseMovieName(filepath.Base(filepath.Dir(path)))
	ownFolder := filepath.Dir(path) != filepath.Clean(lib.Paths[0]) &&
		metadata.Normalize(folder.Title) == metadata.Normalize(name.Title) && name.Title != ""
	if name.Part > 1 {
		return nil // the first part speaks for the movie
	}
	art := movieArt(path, ownFolder)
	if err := s.store.SetLocalArt(ctx, itemID, art.Poster, art.Backdrop); err != nil {
		return err
	}
	if nfo := movieNFO(path, ownFolder); !nfo.empty() {
		return s.store.AdoptIDs(ctx, itemID, nfo.TMDBID, 0, nfo.IMDBID)
	}
	return nil
}

func (s *Scanner) matchMovies(ctx context.Context, libraryID int64) error {
	items, err := s.store.ItemsWithMetadataState(ctx, libraryID, "movie", "pending")
	if err != nil {
		return err
	}
	for _, it := range items {
		tctx, save := s.traced(ctx, it.ID)
		m, err := s.tmdb.MatchMovie(tctx, it.Title, it.Year, it.TMDBID, it.IMDBID)
		save()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Leave the item pending so the next scan retries it.
			s.log.Warn("tmdb lookup failed", "title", it.Title, "year", it.Year, "err", err)
			continue
		}
		if m == nil {
			s.update(libraryID, func(st *Status) { st.Unmatched++ })
			if err := s.store.SetMetadataState(ctx, it.ID, "unmatched"); err != nil {
				return err
			}
			continue
		}
		if err := s.saveMovie(ctx, it, m); err != nil {
			return err
		}
		s.update(libraryID, func(st *Status) { st.Matched++ })
	}
	return nil
}

func movieMetadata(it store.Item, m *metadata.Movie) *store.Item {
	it.Title, it.OriginalTitle, it.Overview = m.Title, m.OriginalTitle, m.Overview
	it.Year, it.ReleaseDate, it.RuntimeMin = m.Year, m.ReleaseDate, m.RuntimeMin
	it.TMDBID, it.IMDBID = m.TMDBID, m.IMDBID
	it.PosterPath, it.BackdropPath = m.PosterPath, m.BackdropPath
	applyDetails(&it, m.Details)
	it.MetadataState = "matched"
	return &it
}

func applyDetails(it *store.Item, d metadata.Details) {
	it.Tagline, it.ContentRating, it.Rating = d.Tagline, d.ContentRating, d.Rating
	it.Studio = strings.Join(d.Studios, ", ")
}

// saveMovie stores a movie's metadata, genres and credits, and notes when it was fetched.
func (s *Scanner) saveMovie(ctx context.Context, it store.Item, m *metadata.Movie) error {
	if err := s.store.UpdateMetadata(ctx, movieMetadata(it, m)); err != nil {
		return err
	}
	if err := s.saveDetails(ctx, it.ID, m.Details); err != nil {
		return err
	}
	return s.store.MarkSynced(ctx, it.ID)
}

// saveDetails stores genres and credits.
func (s *Scanner) saveDetails(ctx context.Context, itemID string, d metadata.Details) error {
	credits := make([]store.Credit, 0, len(d.People))
	for _, p := range d.People {
		credits = append(credits, store.Credit{TMDBID: p.TMDBID, Name: p.Name, ProfilePath: p.ProfilePath,
			Role: p.Role, Character: p.Character, Order: p.Order})
	}
	return s.store.SetItemDetails(ctx, itemID, d.Genres, credits)
}

// refreshUnsyncedMovies fetches matched movies that have never been fully fetched (e.g. matched
// before AMS stored genres and cast), a batch per scan.
func (s *Scanner) refreshUnsyncedMovies(ctx context.Context, libraryID int64) error {
	items, err := s.store.UnsyncedItems(ctx, libraryID, "movie", 200)
	if err != nil {
		return err
	}
	for _, it := range items {
		if _, err := s.RefreshMovie(ctx, it.ID); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.log.Warn("refreshing movie failed", "title", it.Title, "err", err)
		}
	}
	return nil
}

// ErrNoProvider means the metadata provider needed isn't configured (no API key).
var ErrNoProvider = errors.New("no metadata provider is configured")

// RefreshMovie fetches a movie's metadata again now. Fields the user locked are kept. It reports
// whether a match was found.
func (s *Scanner) RefreshMovie(ctx context.Context, itemID string) (bool, error) {
	if !s.tmdb.Configured() {
		return false, ErrNoProvider
	}
	it, err := s.store.GetItem(ctx, itemID)
	if err != nil || it == nil {
		return false, err
	}
	tctx, save := s.traced(ctx, it.ID)
	m, err := s.tmdb.MatchMovie(tctx, it.Title, it.Year, it.TMDBID, it.IMDBID)
	save()
	if err != nil {
		return false, err
	}
	if m == nil {
		return false, s.store.SetMetadataState(ctx, it.ID, "unmatched")
	}
	return true, s.saveMovie(ctx, *it, m)
}

func applyProbe(f *store.MediaFile, res *probe.Result) {
	f.Container, f.DurationMS, f.Bitrate = res.Container, res.DurationMS, res.Bitrate
	for _, ps := range res.Streams {
		f.Streams = append(f.Streams, store.Stream{
			Index: ps.Index, Kind: ps.Kind, Codec: ps.Codec, Profile: ps.Profile, Language: ps.Language,
			Title: ps.Title, Width: ps.Width, Height: ps.Height, Channels: ps.Channels,
			IsDefault: ps.IsDefault, IsForced: ps.IsForced,
		})
	}
}

func isBonusFile(name string) bool {
	stem := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	if stem == "sample" {
		return true
	}
	for _, suffix := range []string{"-trailer", ".trailer", "_trailer", "-sample", ".sample", "-featurette",
		"-behindthescenes", "-deleted", "-interview", "-scene", "-short", "-other"} {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	return false
}
