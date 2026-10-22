// Package watcher notices changes in library folders and asks for those libraries to be scanned,
// so new files appear without a manual scan. Some folders (network shares in particular) don't
// report changes, so a configurable periodic scan provides a fallback.
package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// Wait until a library's folders have been quiet this long before scanning, so files that are
	// still being copied are picked up once, complete.
	settle = 10 * time.Second
	// Rescan every library this often regardless, for folders that don't report changes.
	periodic = time.Hour
)

// Folders never worth watching (they also hold no library content).
var ignoredDirs = map[string]bool{"@eadir": true, "$recycle.bin": true, "system volume information": true}

// Watcher watches library folders and asks for scans. Create one with New, start it with Run and
// tell it about the libraries (and later changes to them) with SetLibraries.
type Watcher struct {
	scan func(libraryID int64)
	// ScanPaths handles settled paths; hourly/error recovery still uses the full scan callback.
	ScanPaths func(libraryID int64, paths []string)
	log       *slog.Logger
	updates   chan map[int64][]string

	// Owned by the Run goroutine.
	fs      *fsnotify.Watcher
	libs    map[int64][]string
	roots   map[string]int64 // library folder -> library id
	watched int

	mu       sync.Mutex
	dirty    map[int64]map[string]time.Time // settle each changed path independently
	config   Config
	changed  chan struct{}
	catchup  bool
	issues   map[string]string
	rootLive map[string]bool
}

// Config controls server-wide discovery. The startup trigger belongs to the caller.
type Config struct {
	Automatic, Partial bool
	Interval           time.Duration
}

// Configure takes effect without cancelling work already submitted to the scanner.
func (w *Watcher) Configure(c Config) {
	w.mu.Lock()
	if c == w.config {
		w.mu.Unlock()
		return
	}
	if !w.config.Automatic && c.Automatic {
		w.catchup = true
	}
	w.config = c
	if !c.Automatic {
		w.dirty = map[int64]map[string]time.Time{}
		w.catchup = false
	}
	w.mu.Unlock()
	select {
	case w.changed <- struct{}{}:
	default:
	}
}

func (w *Watcher) configuration() Config { w.mu.Lock(); defer w.mu.Unlock(); return w.config }

// Problem is an actionable, current notification failure for Library Health.
func (w *Watcher) Problem(libraryID int64) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.config.Automatic {
		return ""
	}
	for path, reason := range w.issues {
		for root, id := range w.roots {
			if id == libraryID && (path == root || strings.HasPrefix(path, root+string(filepath.Separator))) {
				fallback := "Use Scan now or enable scheduled full scans to catch missed changes."
				if w.config.Interval > 0 {
					frequency := fmt.Sprintf("every %d hours", int(w.config.Interval.Hours()))
					if w.config.Interval == time.Hour {
						frequency = "every hour"
					}
					fallback = "Scheduled full scans check for missed changes " + frequency + "."
				}
				return "Folder change detection: " + reason + ". " + fallback
			}
		}
	}
	return ""
}

func (w *Watcher) issue(path string, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err == nil {
		delete(w.issues, path)
	} else {
		w.issues[path] = err.Error()
	}
}

// New creates a watcher that calls scan(libraryID) when a library has changed and settled, and
// for every library once per periodic interval.
func New(scan func(libraryID int64), log *slog.Logger) *Watcher {
	return &Watcher{scan: scan, log: log, updates: make(chan map[int64][]string, 1),
		roots: map[string]int64{}, dirty: map[int64]map[string]time.Time{},
		config: Config{Automatic: true, Partial: true, Interval: periodic}, changed: make(chan struct{}, 1),
		issues: map[string]string{}, rootLive: map[string]bool{}}
}

// SetLibraries replaces the set of watched libraries (library id -> folders).
func (w *Watcher) SetLibraries(libs map[int64][]string) {
	for {
		select {
		case w.updates <- libs:
			return
		default:
			// Replace an update that hasn't been picked up yet.
			select {
			case <-w.updates:
			default:
			}
		}
	}
}

// Run watches until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	defer func() {
		if w.fs != nil {
			w.fs.Close()
		}
	}()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var timer *time.Timer
	var periodicC <-chan time.Time
	interval := time.Duration(-1)
	resetTimer := func() {
		next := w.configuration().Interval
		if next == interval {
			return
		}
		if timer != nil {
			timer.Stop()
		}
		interval = next
		periodicC = nil
		if next > 0 {
			timer = time.NewTimer(next)
			periodicC = timer.C
		}
	}
	resetTimer()
	automatic := w.configuration().Automatic
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	availability := time.NewTicker(time.Minute)
	defer availability.Stop()
	for {
		var events <-chan fsnotify.Event
		var errs <-chan error
		if w.fs != nil {
			events, errs = w.fs.Events, w.fs.Errors
		}
		select {
		case <-ctx.Done():
			return
		case libs := <-w.updates:
			w.watch(libs)
			automatic = w.configuration().Automatic
		case <-w.changed:
			resetTimer()
			w.mu.Lock()
			catchup := w.catchup && w.config.Automatic
			w.catchup = false
			w.mu.Unlock()
			if next := w.configuration().Automatic; next != automatic {
				w.watch(w.libs)
				automatic = next
			}
			if catchup {
				for id := range w.libs {
					w.scan(id)
				}
			}
		case ev := <-events:
			w.handle(ev)
		case err := <-errs:
			if err != nil {
				w.watchError(err)
			}
		case <-tick.C:
			w.dispatchSettled(time.Now())
		case <-availability.C:
			w.checkRoots()
		case <-periodicC:
			// Check the latest value too: an Off update may race a ready timer.
			if w.configuration().Interval == interval && interval > 0 {
				for id := range w.libs {
					w.scan(id)
				}
			}
			if interval > 0 {
				timer.Reset(interval)
			}
		}
	}
}

// watch starts over watching the given libraries.
func (w *Watcher) watch(libs map[int64][]string) {
	if w.fs != nil {
		w.fs.Close()
		w.fs = nil
	}
	w.mu.Lock()
	w.libs, w.roots, w.watched = libs, map[string]int64{}, 0
	w.issues = map[string]string{}
	w.rootLive = map[string]bool{}
	for id, paths := range libs {
		for _, p := range paths {
			w.roots[filepath.Clean(p)] = id
		}
	}
	for id := range w.dirty {
		if _, ok := libs[id]; !ok {
			delete(w.dirty, id)
		}
	}
	w.mu.Unlock()
	if len(w.roots) == 0 || !w.configuration().Automatic {
		return
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		w.log.Warn("can't watch library folders", "err", err)
		for root := range w.roots {
			w.issue(root, err)
		}
		return
	}
	w.fs = fw
	for root := range w.roots {
		info, err := os.Stat(root)
		w.rootLive[root] = err == nil && info.IsDir()
		w.addTree(root)
	}
	w.log.Info("watching library folders for changes", "folders", w.watched)
}

// Only stat known roots on this timer. A returning root needs watch registration once;
// normal recursive media discovery still follows the user's chosen triggers.
func (w *Watcher) checkRoots() {
	if !w.configuration().Automatic {
		return
	}
	if w.fs == nil {
		w.watch(w.libs)
		if w.fs != nil {
			// Registration recovered: reconcile changes missed while no watcher existed.
			for id := range w.libs {
				w.scan(id)
			}
		}
		return
	}
	for root, id := range w.roots {
		info, err := os.Stat(root)
		live := err == nil && info.IsDir()
		if !live {
			if err == nil {
				err = fmt.Errorf("folder is not a directory")
			}
			w.issue(root, err)
		}
		if live && !w.rootLive[root] {
			w.addTree(root)
			w.scan(id)
		}
		w.rootLive[root] = live
	}
}

// addTree watches a folder and every folder below it.
func (w *Watcher) addTree(root string) {
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			w.issue(p, err)
			return nil
		}
		if !d.IsDir() {
			if p == root {
				w.issue(p, fmt.Errorf("folder is not a directory"))
			}
			return nil
		}
		if p != root && skipDir(d.Name()) {
			return fs.SkipDir
		}
		if err := w.fs.Add(p); err != nil {
			w.log.Warn("can't watch folder", "path", p, "err", err)
			w.issue(p, err)
			return fs.SkipDir
		}
		w.issue(p, nil)
		w.watched++
		return nil
	})
}

func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || ignoredDirs[strings.ToLower(name)]
}

func (w *Watcher) handle(ev fsnotify.Event) {
	if !w.configuration().Automatic {
		return
	}
	if ev.Has(fsnotify.Chmod) && !ev.Has(fsnotify.Write) {
		return // permission or timestamp changes don't affect what's in the library
	}
	name := filepath.Base(ev.Name)
	if skipDir(name) {
		return
	}
	lib, ok := w.library(ev.Name)
	if !ok {
		return
	}
	// A new folder (e.g. a season copied in): watch it too, and scan for what's already in it.
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			w.addTree(ev.Name)
		}
	}
	w.mu.Lock()
	if !w.config.Automatic {
		w.mu.Unlock()
		return
	}
	if w.dirty[lib] == nil {
		w.dirty[lib] = map[string]time.Time{}
	}
	w.dirty[lib][ev.Name] = time.Now()
	w.mu.Unlock()
}

// library finds the library a path is in: the one with the longest matching folder.
func (w *Watcher) library(path string) (int64, bool) {
	best, id := "", int64(0)
	for root, lib := range w.roots {
		if (path == root || strings.HasPrefix(path, root+string(filepath.Separator))) && len(root) > len(best) {
			best, id = root, lib
		}
	}
	return id, best != ""
}

// settled returns the libraries that changed and have been quiet for the settle time, and
// forgets them.
func (w *Watcher) settled(now time.Time) map[int64][]string {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := map[int64][]string{}
	for id, paths := range w.dirty {
		for path, last := range paths {
			if now.Sub(last) >= settle {
				out[id] = append(out[id], path)
				delete(paths, path)
			}
		}
		if len(paths) == 0 {
			delete(w.dirty, id)
		}
	}
	return out
}

func (w *Watcher) watchError(err error) {
	w.log.Warn("folder watching error", "err", err)
	for root := range w.roots {
		w.issue(root, err)
	}
	if w.configuration().Automatic {
		for id := range w.libs {
			w.scan(id)
		}
	}
}

func (w *Watcher) dispatchSettled(now time.Time) {
	for id, paths := range w.settled(now) {
		c := w.configuration()
		if !c.Automatic {
			continue
		}
		w.log.Info("library folders changed; scanning", "library", id, "paths", len(paths))
		if c.Partial && w.ScanPaths != nil {
			w.ScanPaths(id, paths)
		} else {
			w.scan(id)
		}
	}
}
