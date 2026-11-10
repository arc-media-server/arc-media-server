// Package scanner walks library folders, records what it finds and matches it to metadata.
// It never writes to, renames or moves media files.
package scanner

import (
	"ams/internal/activity"
	"ams/internal/events"
	"ams/internal/schedule"
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
	"ams/internal/subtitles"
)

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".mov": true, ".wmv": true,
	".ts": true, ".m2ts": true, ".webm": true, ".mpg": true, ".mpeg": true, ".flv": true,
}

// Folders holding extras rather than the main feature (Plex/Jellyfin naming), and what kind.
var extraDirs = map[string]string{
	"extras": "other", "featurettes": "featurette", "behind the scenes": "behind_the_scenes",
	"deleted scenes": "deleted_scene", "interviews": "interview", "scenes": "scene", "shorts": "short",
	"trailers": "trailer", "other": "other",
}

// Folders that are never looked in.
var ignoredDirs = map[string]bool{"samples": true, "sample": true, "@eadir": true, "$recycle.bin": true}

// File-name endings that mark an extra ("Alien (1979)-trailer.mkv"), and what kind.
var extraSuffixes = []struct{ suffix, kind string }{
	{"-trailer", "trailer"}, {".trailer", "trailer"}, {"_trailer", "trailer"},
	{"-featurette", "featurette"}, {"-behindthescenes", "behind_the_scenes"}, {"-deleted", "deleted_scene"},
	{"-interview", "interview"}, {"-scene", "scene"}, {"-short", "short"}, {"-other", "other"},
}

// extraFile is an extra found while walking a library folder.
type extraFile struct {
	fileID int64
	path   string
	kind   string
	// The folder its movie or show is in, and for "<movie file>-trailer.mkv" the movie file's name.
	ownerDir, ownerStem string
}

type jobControl struct {
	id     int64
	ctx    context.Context
	cancel context.CancelFunc
}

type Status struct {
	JobID          int64     `json:"job_id,omitempty"`
	Operation      string    `json:"operation,omitempty"`
	Stopping       bool      `json:"stopping,omitempty"`
	Stopped        bool      `json:"stopped,omitempty"`
	Running        bool      `json:"running"`
	StartedAt      time.Time `json:"started_at,omitzero"`
	FinishedAt     time.Time `json:"finished_at,omitzero"`
	FilesSeen      int       `json:"files_seen"`
	FilesProbed    int       `json:"files_probed"`
	FilesReused    int       `json:"files_reused"`
	ProbeErrors    int       `json:"probe_errors"`
	MetadataErrors int       `json:"metadata_errors"`
	NewItems       int       `json:"new_items"`
	Missing        int64     `json:"files_missing"`
	Matched        int       `json:"matched"`
	Unmatched      int       `json:"unmatched"`
	// Video files in a TV library that couldn't be read as an episode.
	Unrecognised int    `json:"not_recognised"`
	Error        string `json:"error,omitempty"`
	// What the scan is doing after looking through the folders (e.g. "Matching movies"), and how
	// far through it is.
	Phase string `json:"phase,omitempty"`
	Done  int    `json:"done,omitempty"`
	Total int    `json:"total,omitempty"`
	// Retain error-log context without another database read after a failure.
	libraryName string
}

// summary separates files checked from details actually read and titles identified.
func (st Status) summary() string {
	parts := []string{fmt.Sprintf("%d files checked", st.FilesSeen), fmt.Sprintf("%d file details read", max(0, st.FilesProbed-st.ProbeErrors))}
	if st.FilesReused > 0 {
		parts = append(parts, fmt.Sprintf("%d reused saved details", st.FilesReused))
	}
	if st.ProbeErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d inspection error%s", st.ProbeErrors, pluralSuffix(st.ProbeErrors)))
	}
	if st.MetadataErrors > 0 {
		parts = append(parts, fmt.Sprintf("%d metadata update%s deferred", st.MetadataErrors, pluralSuffix(st.MetadataErrors)))
	}
	if st.Unrecognised > 0 {
		parts = append(parts, fmt.Sprintf("%d unrecognised file%s", st.Unrecognised, pluralSuffix(st.Unrecognised)))
	}
	if st.Matched > 0 {
		parts = append(parts, fmt.Sprintf("%d identified", st.Matched))
	}
	if st.NewItems > 0 {
		parts = append(parts, fmt.Sprintf("%d new", st.NewItems))
	}
	if st.Missing > 0 {
		parts = append(parts, fmt.Sprintf("%d missing", st.Missing))
	}
	if st.Unmatched > 0 {
		parts = append(parts, fmt.Sprintf("%d not matched", st.Unmatched))
	}
	return strings.Join(parts, " · ")
}

func pluralSuffix(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// describe says what a running scan is doing, for the Activity menu.
func (st Status) describe() string {
	var parts []string
	switch {
	case st.Phase == "":
		parts = append(parts, fmt.Sprintf("Looking through folders · %d files", st.FilesSeen))
	case st.Phase == "Checking files and titles":
		parts = append(parts, fmt.Sprintf("Checking files and titles · %d files checked", st.FilesSeen))
	case st.Total > 0:
		parts = append(parts, fmt.Sprintf("%s · %d of %d", st.Phase, max(min(st.Done, st.Total), 0), st.Total))
	case st.Done > 0:
		parts = append(parts, fmt.Sprintf("%s · %d checked", st.Phase, st.Done))
	default:
		parts = append(parts, st.Phase)
	}
	if st.Matched > 0 {
		parts = append(parts, fmt.Sprintf("%d identified", st.Matched))
	}
	return strings.Join(parts, " · ")
}

// How many titles are looked up at once, and how many files read at once. The provider clients
// still pace their requests; reading a few files at a time keeps spinning disks from thrashing.
const (
	matchWorkers = 6
	probeWorkers = 3
)

type Scanner struct {
	// Events is configured before workers start; invalidations contain no item metadata.
	Events *events.Hub
	ctx    context.Context
	store  *store.Store
	prober *probe.Prober
	tmdb   *metadata.TMDB // check Configured() before use: there may be no API key
	tvdb   *metadata.TVDB
	log    *slog.Logger

	// OnFinish, if set, is called after each scan (successful or not). Set it before starting scans.
	OnFinish func(libraryID int64)
	// MaintenanceAllowed is set before workers start; nil retains standalone behavior.
	MaintenanceAllowed func() bool
	// Activity, if set, is told about each scan.
	Activity *activity.Tracker
	// Window, if set, is when regular metadata refreshes happen (always, if nil).
	Window *schedule.Window
	// For Other videos libraries: FFmpeg (for a frame of each video) and where the frames are kept.
	FFmpeg   func() string
	ThumbDir string

	wg     sync.WaitGroup
	mu     sync.Mutex
	status map[int64]*Status
	// Full discovery proofs are deliberately memory-only: a restart requires a fresh scan.
	trashReady map[int64][]string
	jobs       map[int64]*jobControl
	nextJob    int64
	stopEpoch  map[int64]int64
	tasks      map[int64]*activity.Handle
	// Libraries that changed during their scan and need scanning again when it ends.
	again         map[int64]bool
	pendingPaths  map[int64][]string
	retryPending  map[int64]bool
	retryWake     map[int64]chan struct{}
	retryAttempts map[int64]int
	retryBase     time.Duration // zero uses the automatic one-minute base
	// Titles a library preview identified, so adding the library doesn't search for them again.
	remembered remembered
	// The latest Fix Match of each show still being fetched in the background, by item id.
	manualMatch sync.Mutex // serializes explicit Match/Unmatch admission, not background scanning
	matching    map[string]int64
	matchGen    int64
}

// New creates a scanner. Background scans stop when ctx is cancelled.
func New(ctx context.Context, st *store.Store, prober *probe.Prober, tmdb *metadata.TMDB, tvdb *metadata.TVDB, log *slog.Logger) *Scanner {
	return &Scanner{ctx: ctx, store: st, prober: prober, tmdb: tmdb, tvdb: tvdb, log: log,
		nextJob: activity.NewRunID(), status: map[int64]*Status{}, jobs: map[int64]*jobControl{}, stopEpoch: map[int64]int64{}, again: map[int64]bool{}, pendingPaths: map[int64][]string{}, retryPending: map[int64]bool{}, retryWake: map[int64]chan struct{}{}, retryAttempts: map[int64]int{}, tasks: map[int64]*activity.Handle{}}
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
	if st := s.status[libraryID]; st != nil && st.Stopping {
		return
	}
	if !s.startLocked(libraryID) {
		s.again[libraryID] = true
		delete(s.pendingPaths, libraryID)
	}
}

// RequestPaths scans affected top-level movie/show folders, retaining queued changes.
func (s *Scanner) RequestPaths(libraryID int64, paths []string) {
	if len(paths) == 0 {
		s.Request(libraryID)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if st := s.status[libraryID]; st != nil && st.Stopping {
		return
	}
	if !s.startLocked(libraryID, paths...) && !s.again[libraryID] {
		s.pendingPaths[libraryID] = append(s.pendingPaths[libraryID], paths...)
		if len(s.pendingPaths[libraryID]) > 4096 {
			s.again[libraryID] = true
			delete(s.pendingPaths, libraryID)
		}
	}
}

// Maintain refreshes known metadata without discovering files, serialized with scans.
func (s *Scanner) Maintain(libraryID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startWorkLocked(libraryID, true)
}

func (s *Scanner) startLocked(libraryID int64, paths ...string) bool {
	return s.startWorkLocked(libraryID, false, paths...)
}

func (s *Scanner) startWorkLocked(libraryID int64, maintenance bool, paths ...string) bool {
	kind := "scan"
	if maintenance {
		kind = "maintenance"
	}
	return s.startKindLocked(libraryID, kind, paths...)
}

// StartOperation reserves the same library lease as discovery and scheduled refresh.
func (s *Scanner) StartOperation(libraryID int64, operation string) bool {
	if operation != "refresh" && operation != "analyze" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startKindLocked(libraryID, operation)
}

// StartItemAnalysis rereads one movie's or episode's files, or every episode of a show, under
// the library's lease, so it never overlaps a scan or other library work and can be stopped.
func (s *Scanner) StartItemAnalysis(libraryID int64, itemID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startKindLocked(libraryID, "analyze_item", itemID)
}

func (s *Scanner) startKindLocked(libraryID int64, operation string, paths ...string) bool {
	if st := s.status[libraryID]; st != nil && st.Running {
		return false
	}
	if s.ctx.Err() != nil {
		return false
	}
	releaseCatalog, ok := s.store.BeginCatalogWork()
	if !ok {
		return false
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.nextJob++
	job := &jobControl{id: s.nextJob, ctx: ctx, cancel: cancel}
	s.jobs[libraryID] = job
	s.status[libraryID] = &Status{JobID: job.id, Operation: operation, Running: true, StartedAt: time.Now(), Phase: "Opening library"}
	paths = append([]string(nil), paths...)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer releaseCatalog()
		defer cancel()
		var err error
		switch operation {
		case "maintenance":
			err = s.maintain(ctx, libraryID)
		case "refresh", "analyze":
			err = s.manualWork(ctx, libraryID, operation)
		case "analyze_item":
			err = s.analyzeItem(ctx, libraryID, paths[0]) // StartItemAnalysis passes the item ID
		default:
			err = s.scanPaths(ctx, libraryID, paths)
		}
		s.update(libraryID, func(st *Status) {
			st.FinishedAt = time.Now()
			if st.Stopping || errors.Is(err, context.Canceled) {
				st.Stopped, st.Stopping, st.Phase = true, false, "Stopped"
			} else if err != nil {
				st.Error = err.Error()
			}
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			st := s.Status(libraryID)
			name := st.libraryName
			if name == "" {
				name = strconv.FormatInt(libraryID, 10)
			}
			level := slog.LevelError
			if errors.Is(err, ErrScanRootsUnsafe) {
				level = slog.LevelWarn
			}
			s.log.Log(ctx, level, "scan failed", "library", name, "library_id", libraryID,
				"phase", st.Phase, "files_seen", st.FilesSeen, "err", err)
		}
		if s.OnFinish != nil {
			s.OnFinish(libraryID)
		}
		s.finishLibraryWork(libraryID)
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
	st := s.status[libraryID]
	fn(st)
	if h := s.tasks[libraryID]; h != nil {
		h.Detail(st.describe())
		if st.Phase == "Checking files and titles" {
			h.Progress(0, 0)
		} else {
			h.Progress(st.Done, st.Total)
		}
	}
	s.mu.Unlock()
}

// phase starts a stage of a scan that goes through total things.
func (s *Scanner) phase(libraryID int64, name string, total int) {
	s.update(libraryID, func(st *Status) { st.Phase, st.Done, st.Total = name, 0, total })
}

// step marks one more thing done in the current stage.
func (s *Scanner) step(libraryID int64) {
	s.update(libraryID, func(st *Status) { st.Done++ })
}

func (s *Scanner) scan(ctx context.Context, libraryID int64) (err error) {
	return s.scanPaths(ctx, libraryID, nil)
}

func (s *Scanner) scanPaths(ctx context.Context, libraryID int64, paths []string) (err error) {
	s.mu.Lock()
	delete(s.trashReady, libraryID)
	s.mu.Unlock()
	lib, err := s.store.GetLibrary(ctx, libraryID)
	if err != nil {
		return err
	}
	if lib == nil {
		return fmt.Errorf("library %d not found", libraryID)
	}
	priorFiles, err := s.store.PresentLibraryFiles(ctx, libraryID)
	if err != nil {
		return err
	}

	s.update(libraryID, func(st *Status) { st.libraryName, st.Phase = lib.Name, "Checking library folders" })
	log := s.log.With("library", lib.Name)
	log.Info("scan started", "paths", lib.Paths)
	ctx = metadata.WithPreviewReuse(metadata.WithRegion(metadata.WithLanguage(ctx, lib.MetadataLanguage), lib.RatingRegion))
	visited, err := scanFolders(lib, paths)
	if err != nil {
		return err
	}
	task := s.Activity.Start("scan", "Scanning "+lib.Name)
	s.registerTask(lib.ID, task)
	task.Detail(s.Status(lib.ID).describe())
	defer func() {
		s.mu.Lock()
		delete(s.tasks, lib.ID)
		s.mu.Unlock()
		task.Detail(s.Status(lib.ID).summary())
		task.Finish(err)
	}()
	scanStart := time.Now().UnixMilli()
	seen := &seenFiles{st: s.store, at: scanStart}
	ctx = context.WithValue(ctx, seenKey{}, seen)
	secondary := &secondaryShows{files: map[string][]episodeFile{}}
	ctx = context.WithValue(ctx, secondaryKey{}, secondary)
	series := map[string]*seriesGroup{}
	pipeline := newMatchPipeline(ctx)
	defer pipeline.finish()
	queuedMovies := map[string]bool{}
	enqueueMovie := func(id string) error {
		if !s.tmdb.Configured() || queuedMovies[id] {
			return nil
		}
		it, err := s.store.GetItem(ctx, id)
		if err != nil {
			return err
		}
		if it == nil || it.MetadataState != "pending" {
			return nil
		}
		queuedMovies[id] = true
		s.update(lib.ID, func(st *Status) { st.Total++ })
		return pipeline.offer(func() error { defer s.step(lib.ID); return s.matchMovie(ctx, lib.ID, id) })
	}
	ctx = context.WithValue(ctx, movieQueueKey{}, enqueueMovie)
	s.phase(lib.ID, "Checking files and titles", 0)
	showLocks := map[string]*sync.Mutex{}
	// Folders merged into another show lock as that show (store.MergeSeries).
	aliases, err := s.store.SeriesAliases(ctx, lib.ID)
	if err != nil {
		return err
	}
	enqueueShows := func(all bool) error {
		for key, group := range series {
			if !all && group.folder == "" {
				continue
			}
			delete(series, key)
			// Directory spellings differ even when they create the same series row.
			// Serialize using the same identity as EnsureItem, not the directory key.
			identity := seriesMatchKey(group.name)
			if kept, ok := aliases[identity]; ok {
				identity = kept
			}
			lock := showLocks[identity]
			if lock == nil {
				lock = &sync.Mutex{}
				showLocks[identity] = lock
			}
			s.update(lib.ID, func(st *Status) { st.Total++ })
			if err := pipeline.offer(func() error {
				defer s.step(lib.ID)
				// The same show can have files in several library roots.
				lock.Lock()
				defer lock.Unlock()
				return s.syncSeriesGroup(ctx, lib, group, log)
			}); err != nil {
				return err
			}
		}
		return nil
	}
	allRootsOK := true

	var addFile func(root, path string, d fs.DirEntry) error
	folders := map[string]*videoFolder{}
	switch lib.Kind {
	case "videos":
		addFile = func(root, path string, d fs.DirEntry) error {
			return s.scanVideoFile(ctx, lib, root, path, d, scanStart, folders)
		}
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

	var extras []extraFile
	for _, folder := range visited {
		root := folder.root
		chunks := []string{folder.path}
		if lib.Kind == "shows" && folder.path == root {
			entries, readErr := os.ReadDir(root)
			if readErr != nil {
				allRootsOK = false
				continue
			}
			chunks = nil
			for _, entry := range entries {
				chunks = append(chunks, filepath.Join(root, entry.Name()))
			}
		}
		for _, chunk := range chunks {
			complete, err := walkVideos(ctx, root, log, func(path string, d fs.DirEntry) {
				if err := addFile(root, path, d); err != nil {
					allRootsOK = false
					log.Warn("failed to add file", "path", path, "err", err)
				}
			}, func(path string, d fs.DirEntry, ex extraFile) {
				// Home videos have no extras: a "Behind the scenes" folder is just more videos.
				if lib.Kind == "videos" {
					if err := addFile(root, path, d); err != nil {
						allRootsOK = false
						log.Warn("failed to add file", "path", path, "err", err)
					}
					return
				}
				id, err := s.recordFile(ctx, lib, path, d, scanStart, false)
				if err != nil {
					allRootsOK = false
					log.Warn("failed to add file", "path", path, "err", err)
					return
				}
				ex.fileID = id
				extras = append(extras, ex)
			}, chunk)
			if err != nil {
				return err
			}
			// Don't mark anything missing if a drive is offline; that would hide the whole library.
			allRootsOK = allRootsOK && complete
			if lib.Kind == "shows" {
				if err := enqueueShows(false); err != nil {
					return err
				}
			}
		}
	}

	if err := seen.flush(ctx); err != nil {
		return err
	}
	if lib.Kind == "shows" {
		if err := enqueueShows(true); err != nil {
			return err
		}
	}
	if lib.Kind == "movies" && s.tmdb.Configured() {
		pending, err := s.store.ItemsWithMetadataState(ctx, lib.ID, "movie", "pending")
		if err != nil {
			return err
		}
		for _, it := range pending {
			if len(paths) > 0 {
				files, err := s.store.ItemFiles(ctx, it.ID)
				if err != nil {
					return err
				}
				inside := false
				for _, f := range files {
					if inScanFolders(f.Path, visited) {
						inside = true
						break
					}
				}
				if !inside {
					continue
				}
			}
			if err := enqueueMovie(it.ID); err != nil {
				return err
			}
		}
	}
	if err := pipeline.finish(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if allRootsOK {
		if err := s.checkDiscoveryAnchors(ctx, lib.ID, scanStart, visited, priorFiles); err != nil {
			return err
		}
		var folders []string
		completeLibrary := len(visited) == len(lib.Paths)
		for _, f := range visited {
			folders = append(folders, f.path)
			completeLibrary = completeLibrary && f.path == f.root
		}
		var n int64
		var err error
		if completeLibrary {
			// Also reconcile files whose old root was removed from the library.
			n, err = s.store.MarkMissing(ctx, lib.ID, scanStart, time.Now().Unix())
		} else {
			n, err = s.store.MarkMissingFolders(ctx, lib.ID, scanStart, time.Now().Unix(), folders)
		}
		if err != nil {
			return err
		}
		s.update(lib.ID, func(st *Status) { st.Missing = n })
	}

	if lib.Kind == "videos" {
		if err := s.syncVideoFolders(ctx, lib, folders, allRootsOK && len(paths) == 0); err != nil {
			return err
		}
	}
	// Extras last, once their movies and shows are linked to files.
	s.phase(lib.ID, "Adding extras", 0)
	if err := s.linkExtras(ctx, lib, extras); err != nil {
		return err
	}
	// Then what's inside the new files (length, video, audio and subtitle tracks), now that the
	// titles are showing.
	if err := s.probePending(ctx, lib, scanStart, visited...); err != nil {
		return err
	}
	if err := s.finishSecondary(ctx, lib, secondary); err != nil {
		return err
	}
	if allRootsOK && len(paths) == 0 && ctx.Err() == nil {
		s.mu.Lock()
		if s.trashReady == nil {
			s.trashReady = map[int64][]string{}
		}
		if st := s.status[lib.ID]; st == nil || !st.Stopping {
			s.trashReady[lib.ID] = append([]string(nil), lib.Paths...)
		}
		s.mu.Unlock()
	}

	st := s.Status(lib.ID)
	log.Info("scan finished", "files", st.FilesSeen, "probed", st.FilesProbed, "new", st.NewItems,
		"missing", st.Missing, "matched", st.Matched, "unmatched", st.Unmatched, "not_recognised", st.Unrecognised)
	return nil
}

// walkVideos calls visit for every main video file below root and extra (if set) for every
// trailer, featurette and other extra, skipping hidden folders and samples. complete is false when
// root or part of it couldn't be read, e.g. a drive is offline.
func walkVideos(ctx context.Context, root string, log *slog.Logger, visit func(path string, d fs.DirEntry),
	extra func(path string, d fs.DirEntry, ex extraFile), from ...string) (complete bool, err error) {
	info, err := os.Stat(root)
	if err != nil {
		log.Warn("library folder unavailable", "path", root, "err", err)
		return false, nil
	}
	if !info.IsDir() {
		log.Warn("library folder unavailable", "path", root, "err", "path is not a directory")
		return false, nil
	}
	complete = true
	start := root
	if len(from) > 0 {
		start = from[0]
	}
	err = filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			complete = false
			log.Warn("cannot read", "path", path, "err", err)
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if info, statErr := os.Stat(path); statErr != nil || info.IsDir() {
				complete = false // A directory not traversed cannot establish missing files.
				return nil
			}
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && strings.HasPrefix(name, ".") {
				return fs.SkipDir
			}
			if path != root && ignoredDirs[strings.ToLower(name)] {
				return fs.SkipDir
			}
			return nil
		}
		if !videoExts[strings.ToLower(filepath.Ext(name))] || isSample(name) {
			return nil
		}
		if ex, ok := classifyExtra(root, path); ok {
			if extra != nil {
				extra(path, d, ex)
			}
			return nil
		}
		visit(path, d)
		return nil
	})
	return complete, err
}

// classifyExtra reports whether a file is an extra: inside an extras folder (the owner is the
// folder holding that one), or named "<movie file>-trailer.mkv" and so on.
func classifyExtra(root, path string) (extraFile, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return extraFile{}, false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, dir := range parts[:len(parts)-1] {
		if kind, ok := extraDirs[strings.ToLower(dir)]; ok {
			return extraFile{path: path, kind: kind, ownerDir: filepath.Join(append([]string{root}, parts[:i]...)...)}, true
		}
	}
	name := filepath.Base(path)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	for _, s := range extraSuffixes {
		if strings.HasSuffix(strings.ToLower(stem), s.suffix) {
			return extraFile{path: path, kind: s.kind, ownerDir: filepath.Dir(path), ownerStem: stem[:len(stem)-len(s.suffix)]}, true
		}
	}
	return extraFile{}, false
}

// extraTitle makes a readable title from an extra's file name.
func extraTitle(ex extraFile) string {
	name := filepath.Base(ex.path)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if ex.ownerStem != "" {
		if t := cleanTitle(sepRe.ReplaceAllString(stem[len(ex.ownerStem):], " ")); t != "" && !isKindWord(t) {
			return t
		}
		return extraKindNames[ex.kind]
	}
	return cleanTitle(sepRe.ReplaceAllString(stem, " "))
}

var extraKindNames = map[string]string{"trailer": "Trailer", "featurette": "Featurette", "behind_the_scenes": "Behind the Scenes",
	"deleted_scene": "Deleted Scene", "interview": "Interview", "scene": "Scene", "short": "Short", "other": "Extra"}

func isKindWord(s string) bool {
	s = strings.ToLower(strings.Trim(s, " -._"))
	for _, x := range extraSuffixes {
		if strings.Trim(x.suffix, "-._") == s {
			return true
		}
	}
	return false
}

// linkExtras attaches the extras found in a scan to their movie or show.
func (s *Scanner) linkExtras(ctx context.Context, lib *store.Library, extras []extraFile) error {
	for _, ex := range extras {
		owner := ""
		var err error
		if ex.ownerStem != "" && lib.Kind == "movies" {
			if owner, err = s.store.MovieByFileStem(ctx, lib.ID, ex.ownerDir, ex.ownerStem); err != nil {
				return err
			}
		}
		if owner == "" {
			if owner, err = s.store.OwnerInFolder(ctx, lib.ID, lib.Kind, ex.ownerDir); err != nil {
				return err
			}
		}
		if owner == "" {
			s.log.Debug("extra without a movie or show", "path", ex.path)
			continue
		}
		if err := s.store.SaveExtra(ctx, owner, ex.fileID, ex.kind, extraTitle(ex)); err != nil {
			return err
		}
	}
	return nil
}

// recordFile stores a file and marks it seen in this scan. A new or changed file is read with
// ffprobe now if probeNow, otherwise later by probePending, so titles can be matched first.
func (s *Scanner) recordFile(ctx context.Context, lib *store.Library, path string, d fs.DirEntry, scanStart int64, probeNow bool) (int64, error) {
	ctx, finish := s.working(ctx, lib.ID, "Checking file", filepath.Base(path), "Checking for changes")
	defer finish()
	info, err := d.Info()
	if err != nil {
		return 0, err
	}
	s.update(lib.ID, func(st *Status) { st.FilesSeen++ })

	existing, err := s.store.GetFileByPath(ctx, path)
	if err != nil {
		return 0, err
	}
	if existing == nil {
		existing, err = s.reuseRenamedFile(ctx, lib, path, info)
		if err != nil {
			return 0, err
		}
	}

	unchanged := existing != nil && existing.Size == info.Size() && existing.ModTime == info.ModTime().Unix()
	if unchanged && (existing.Probed || !probeNow) {
		if existing.Probed {
			s.update(lib.ID, func(st *Status) { st.FilesReused++ })
		}
		if seen, _ := ctx.Value(seenKey{}).(*seenFiles); seen != nil {
			seen.ids = append(seen.ids, existing.ID)
			if len(seen.ids) >= 256 {
				return existing.ID, seen.flush(ctx)
			}
			return existing.ID, nil
		}
		return existing.ID, s.store.TouchFile(ctx, existing.ID, scanStart)
	}
	f := &store.MediaFile{LibraryID: lib.ID, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}
	if !probeNow {
		return s.store.SaveFile(ctx, f, 0, scanStart)
	}
	fileID, err := s.saveProbed(ctx, f, scanStart)
	if err != nil {
		return 0, err
	}
	s.update(lib.ID, func(st *Status) {
		st.FilesProbed++
		if f.ProbeError != "" {
			st.ProbeErrors++
		}
	})
	return fileID, nil
}

// saveProbed reads a file with ffprobe and stores what's in it (or why it couldn't be read).
func (s *Scanner) saveProbed(ctx context.Context, f *store.MediaFile, seenAt int64) (int64, error) {
	currentWork(ctx).Update("Inspecting file", "Reading video, audio and HDR details · "+filepath.Base(f.Path))
	var probedAt int64
	if res, err := s.prober.Probe(ctx, f.Path); err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		f.ProbeError = err.Error()
	} else {
		probedAt = time.Now().Unix()
		applyProbe(f, res)
	}
	id, err := s.store.SaveFile(ctx, f, probedAt, seenAt)
	if err == nil {
		s.Events.Publish(events.Event{Topic: "catalog", LibraryID: f.LibraryID})
	}
	return id, err
}

// probePending reads a library's files that haven't been read yet (new or changed since), a few
// at a time, in path order so each worker stays in one folder (and disk) at a time.
func (s *Scanner) probePending(ctx context.Context, lib *store.Library, scanStart int64, folders ...scanFolder) error {
	files, err := s.store.UnprobedFiles(ctx, lib.ID)
	if len(folders) > 0 {
		filtered := files[:0]
		for _, f := range files {
			if inScanFolders(f.Path, folders) {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}
	if err != nil || len(files) == 0 {
		return err
	}
	s.phase(lib.ID, "Reading file details", len(files))
	return each(ctx, len(files), probeWorkers, func(i int) error {
		f := files[i]
		title, _ := s.store.FileActivityTitle(ctx, f.ID)
		if title == "" {
			title = filepath.Base(f.Path)
		}
		ctx, finish := s.working(ctx, lib.ID, "Inspecting file", title, filepath.Base(f.Path))
		defer finish()
		probe := store.MediaFile{LibraryID: lib.ID, Path: f.Path, Size: f.Size, ModTime: f.ModTime}
		if _, err := s.saveProbed(ctx, &probe, max(f.LastSeen, scanStart)); err != nil {
			return err
		}
		s.update(lib.ID, func(st *Status) {
			st.FilesProbed++
			st.Done++
			if probe.ProbeError != "" {
				st.ProbeErrors++
			}
		})
		return nil
	})
}

// EnsureProbed reads any of these files that haven't been read yet (e.g. a title played before
// its scan got to it), reporting whether it read any.
func (s *Scanner) EnsureProbed(ctx context.Context, files []store.MediaFile) (bool, error) {
	read := false
	for i := range files {
		f := files[i]
		if f.Probed || f.Missing || f.ProbeError != "" {
			continue
		}
		probe := store.MediaFile{LibraryID: f.LibraryID, Path: f.Path, Size: f.Size, ModTime: f.ModTime}
		if _, err := s.saveProbed(ctx, &probe, time.Now().UnixMilli()); err != nil {
			return read, err
		}
		read = true
	}
	return read, nil
}

// each runs fn for 0..n-1 on up to workers goroutines, stopping at the first error. fn must be
// safe to run concurrently (the store is; SQLite takes one write at a time).
func each(ctx context.Context, n, workers int, fn func(i int) error) error {
	next := make(chan int)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range min(workers, n) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				if err := fn(i); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	var err error
feed:
	for i := range n {
		select {
		case next <- i:
		case err = <-errs:
			break feed
		case <-ctx.Done():
			err = ctx.Err()
			break feed
		}
	}
	close(next)
	wg.Wait()
	if err == nil {
		select {
		case err = <-errs:
		default:
		}
	}
	return err
}

func (s *Scanner) scanMovieFile(ctx context.Context, lib *store.Library, path string, d fs.DirEntry, scanStart int64) error {
	fileID, err := s.recordFile(ctx, lib, path, d, scanStart, false)
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
	// A file merged or split by hand stays with the title it was put under.
	itemID, err := s.store.FileItemOverride(ctx, fileID)
	if err != nil {
		return err
	}
	created := false
	if itemID == "" {
		itemID, created, err = s.ensureNamedItem(ctx, &store.Item{
			LibraryID: lib.ID, Kind: "movie", Title: name.Title, Year: name.Year,
			TMDBID: name.TMDBID, IMDBID: name.IMDBID,
		}, matchKey, []int64{fileID})
		if err != nil {
			return err
		}
	}
	if created {
		s.update(lib.ID, func(st *Status) { st.NewItems++ })
		// The library's preview may have identified it already: no need to search again.
		if c := s.remembered.get("movies", name); c != nil && name.TMDBID == 0 && name.IMDBID == "" {
			if err := s.store.AdoptIDs(ctx, itemID, c.ID, 0, ""); err != nil {
				return err
			}
		}
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
	if err := s.store.SetLocalArt(ctx, itemID, art.Poster, art.Backdrop, art.Logo); err != nil {
		return err
	}
	if err := s.store.SetLocalTheme(ctx, itemID, art.Theme); err != nil {
		return err
	}
	if nfo := movieNFO(path, ownFolder); !nfo.empty() {
		if err := s.store.AdoptIDs(ctx, itemID, nfo.TMDBID, 0, nfo.IMDBID); err != nil {
			return err
		}
	}
	if queue, _ := ctx.Value(movieQueueKey{}).(func(string) error); queue != nil {
		return queue(itemID)
	}
	return nil
}

func (s *Scanner) matchMovies(ctx context.Context, libraryID int64) error {
	items, err := s.store.ItemsWithMetadataState(ctx, libraryID, "movie", "pending")
	if err != nil {
		return err
	}
	s.phase(libraryID, "Matching movies", len(items))
	return each(ctx, len(items), matchWorkers, func(i int) error {
		defer s.step(libraryID)
		return s.matchMovie(ctx, libraryID, items[i].ID)
	})
}

func (s *Scanner) matchMovie(ctx context.Context, libraryID int64, id string) error {
	it, err := s.store.GetItem(ctx, id)
	if err != nil {
		return err
	}
	if it == nil || it.MetadataState != "pending" {
		return nil
	}
	ctx, finish := s.working(ctx, libraryID, "Matching movie", activityTitle(it.Title, it.Year), "Looking up metadata on TMDB")
	defer finish()
	tctx, save := s.traced(ctx, it.ID)
	m, err := s.tmdb.MatchMovie(tctx, it.Title, it.Year, it.TMDBID, it.IMDBID)
	save()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Leave the item pending so the next scan retries it.
		s.log.Warn("tmdb lookup failed", "title", it.Title, "year", it.Year, "err", err)
		s.update(libraryID, func(st *Status) { st.MetadataErrors++ })
		if !isMetadataMaintenance(ctx) {
			s.retryLater(libraryID)
		}
		return nil
	}
	if m == nil {
		s.log.Info("no match found for movie", "title", it.Title, "year", it.Year)
		s.update(libraryID, func(st *Status) { st.Unmatched++ })
		return s.store.SetMetadataState(ctx, it.ID, "unmatched")
	}
	currentWork(ctx).Update("Saving movie details", "Saving metadata and artwork references")
	if err := s.saveMovie(ctx, *it, m); err != nil {
		return err
	}
	s.log.Info("matched movie", "file_title", activityTitle(it.Title, it.Year), "title", activityTitle(m.Title, m.Year), "tmdb", m.TMDBID)
	s.update(libraryID, func(st *Status) { st.Matched++ })
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
	it.LogoPath = d.LogoPath
	it.Studio = strings.Join(d.Studios, ", ")
	it.OriginalLanguage = subtitles.LanguageCode2(d.OriginalLanguage)
	it.ShowStatus, it.LastAired, it.NextAired = d.Status, d.LastAired, d.NextAired
}

// saveMovie stores a movie's metadata, genres and credits, and notes when it was fetched.
func (s *Scanner) saveMovie(ctx context.Context, it store.Item, m *metadata.Movie) error {
	if err := s.store.UpdateMetadata(ctx, movieMetadata(it, m)); err != nil {
		return err
	}
	defer s.Events.Publish(events.Event{Topic: "catalog", LibraryID: it.LibraryID})
	var coll *store.CollectionRef
	if c := m.Collection; c != nil {
		coll = &store.CollectionRef{TMDBID: c.TMDBID, Name: c.Name, PosterPath: c.PosterPath, BackdropPath: c.BackdropPath}
	}
	if err := s.store.SetMovieCollection(ctx, it.ID, it.LibraryID, coll); err != nil {
		return err
	}
	if err := s.saveDetails(ctx, it.ID, m.Details); err != nil {
		return err
	}
	return s.store.MarkSynced(ctx, it.ID)
}

func storedCredits(people []metadata.Credit) []store.Credit {
	credits := make([]store.Credit, 0, len(people))
	for _, p := range people {
		credits = append(credits, store.Credit{TMDBID: p.TMDBID, Name: p.Name, ProfilePath: p.ProfilePath,
			Role: p.Role, Character: p.Character, Order: p.Order, EpisodeCount: p.EpisodeCount})
	}
	return credits
}

// saveDetails stores genres and credits.
func (s *Scanner) saveDetails(ctx context.Context, itemID string, d metadata.Details) error {
	if err := s.store.SetItemDetails(ctx, itemID, d.Genres, storedCredits(d.People)); err != nil {
		return err
	}
	if err := s.store.SetRecommendations(ctx, itemID, d.Recommendations); err != nil {
		return err
	}
	return s.store.SetAltTitlesKeywords(ctx, itemID, d.AltTitles, d.Keywords)
}

// refreshMovies is known-title maintenance, independent of discovery and its clock window.
func (s *Scanner) refreshMovies(ctx context.Context, libraryID int64) error {
	before := time.Now().Add(-movieResyncAfter).Unix()
	after := ""
	s.phase(libraryID, "Updating movie details", 0)
	for {
		items, err := s.store.MetadataRefreshPage(ctx, libraryID, before, after, 200)
		if err != nil || len(items) == 0 {
			return err
		}
		after = items[len(items)-1].ID
		if err := each(ctx, len(items), matchWorkers, func(i int) error {
			defer s.step(libraryID)
			if _, err := s.RefreshMovie(ctx, items[i].ID); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				s.update(libraryID, func(st *Status) { st.MetadataErrors++ })
				s.log.Warn("refreshing movie failed", "title", items[i].Title, "err", err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
}

// inLanguage is ctx asking for a library's metadata language.
func (s *Scanner) inLanguage(ctx context.Context, libraryID int64) context.Context {
	if lib, err := s.store.GetLibrary(ctx, libraryID); err == nil && lib != nil {
		return metadata.WithRegion(metadata.WithLanguage(ctx, lib.MetadataLanguage), lib.RatingRegion)
	}
	return ctx
}

// A movie's details are fetched again this often (in the maintenance window).
const movieResyncAfter = 30 * 24 * time.Hour

// ErrNoProvider means the metadata provider needed isn't configured (no API key).
var ErrNoProvider = errors.New("no metadata provider is configured")

// RefreshMovie fetches a movie's metadata again now. Fields the user locked are kept. It reports
// whether a match was found.
func (s *Scanner) RefreshMovie(ctx context.Context, itemID string) (bool, error) {
	release, ok := s.store.BeginCatalogWork()
	if !ok {
		return false, ErrLibraryBusy
	}
	defer release()
	if !s.tmdb.Configured() {
		return false, ErrNoProvider
	}
	it, err := s.store.GetItem(ctx, itemID)
	if err != nil || it == nil {
		return false, err
	}
	if it.MatchSuppressed {
		return false, nil
	}
	if it.MetadataState == "locked" {
		return true, nil // a home video: nothing to look up
	}
	ctx, finish := s.working(ctx, it.LibraryID, "Refreshing movie", activityTitle(it.Title, it.Year), "Looking up metadata on TMDB")
	defer finish()
	ctx = s.inLanguage(ctx, it.LibraryID)
	tctx, save := s.traced(ctx, it.ID)
	m, err := s.tmdb.MatchMovie(tctx, it.Title, it.Year, it.TMDBID, it.IMDBID)
	save()
	if err != nil {
		return false, err
	}
	if m == nil {
		return false, s.store.SetMetadataState(ctx, it.ID, "unmatched")
	}
	currentWork(ctx).Update("Saving movie details", "Saving metadata and artwork references")
	return true, s.saveMovie(ctx, *it, m)
}

func applyProbe(f *store.MediaFile, res *probe.Result) {
	f.Container, f.DurationMS, f.Bitrate = res.Container, res.DurationMS, res.Bitrate
	f.FormatProbeVersion = store.FormatVersion
	for _, c := range res.Chapters {
		f.Chapters = append(f.Chapters, store.Chapter{StartMS: c.StartMS, EndMS: c.EndMS, Title: c.Title})
	}
	for _, ps := range res.Streams {
		f.Streams = append(f.Streams, store.Stream{
			Index: ps.Index, Kind: ps.Kind, Codec: ps.Codec, Profile: ps.Profile, Language: ps.Language,
			Title: ps.Title, Width: ps.Width, Height: ps.Height, Channels: ps.Channels,
			IsDefault: ps.IsDefault, IsForced: ps.IsForced, PixFmt: ps.PixFmt, BitDepth: ps.BitDepth,
			ColorTransfer: ps.ColorTransfer, ColorPrimaries: ps.ColorPrimaries, FrameRate: ps.FrameRate,
			Bitrate: ps.Bitrate, ChannelLayout: ps.ChannelLayout, DolbyVision: ps.DolbyVision,
			DolbyVisionProfile: ps.DolbyVisionProfile, DVBLCompatibilityID: ps.DVBLCompatibilityID, HDR10Plus: ps.HDR10Plus, HDRProbeStatus: ps.HDRProbeStatus,
			AspectRatio: ps.AspectRatio,
		})
	}
}

// isSample reports whether a file is a release sample, which is never shown.
func isSample(name string) bool {
	stem := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	return stem == "sample" || strings.HasSuffix(stem, "-sample") || strings.HasSuffix(stem, ".sample")
}

// CollectionInfo fetches a TMDB collection's description and artwork, or nil without TMDB.
func (s *Scanner) CollectionInfo(ctx context.Context, tmdbID int64) (*metadata.CollectionInfo, error) {
	if !s.tmdb.Configured() {
		return nil, nil
	}
	return s.tmdb.Collection(ctx, tmdbID)
}

// Stop targets a particular run, so a stale button cannot stop a later replacement.
func (s *Scanner) Stop(libraryID, jobID int64) bool {
	s.mu.Lock()
	st := s.status[libraryID]
	if st == nil || st.JobID != jobID {
		s.mu.Unlock()
		return false
	}
	if st.Stopped || st.Stopping {
		s.mu.Unlock()
		return true
	}
	job := s.jobs[libraryID]
	if job == nil || !st.Running {
		s.mu.Unlock()
		return false
	}
	st.Stopping = true
	delete(s.trashReady, libraryID)
	s.stopEpoch[libraryID]++
	delete(s.retryPending, libraryID)
	if wake := s.retryWake[libraryID]; wake != nil {
		close(wake)
		delete(s.retryWake, libraryID)
	}
	delete(s.again, libraryID)
	delete(s.pendingPaths, libraryID)
	if task := s.tasks[libraryID]; task != nil {
		task.Stopping()
	}
	s.mu.Unlock()
	job.cancel()
	return true
}

func (s *Scanner) registerTask(libraryID int64, task *activity.Handle) {
	s.mu.Lock()
	s.tasks[libraryID] = task
	if job := s.jobs[libraryID]; job != nil {
		task.Cancellable(func() { s.Stop(libraryID, job.id) })
		if s.status[libraryID].Stopping {
			task.Stopping()
		}
	}
	s.mu.Unlock()
}
