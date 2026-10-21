// Package watcher notices changes in library folders and asks for those libraries to be scanned,
// so new files appear without a manual scan. Some folders (network shares in particular) don't
// report changes, so every library is also rescanned periodically; scanning unchanged files is cheap.
package watcher

import (
	"context"
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

	mu    sync.Mutex
	dirty map[int64]map[string]time.Time // settle each changed path independently
}

// New creates a watcher that calls scan(libraryID) when a library has changed and settled, and
// for every library once per periodic interval.
func New(scan func(libraryID int64), log *slog.Logger) *Watcher {
	return &Watcher{scan: scan, log: log, updates: make(chan map[int64][]string, 1),
		roots: map[string]int64{}, dirty: map[int64]map[string]time.Time{}}
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
	hourly := time.NewTicker(periodic)
	defer hourly.Stop()
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
		case ev := <-events:
			w.handle(ev)
		case err := <-errs:
			w.log.Warn("folder watching error", "err", err)
			for id := range w.libs {
				w.scan(id)
			}
		case <-tick.C:
			for id, paths := range w.settled(time.Now()) {
				w.log.Info("library folders changed; scanning", "library", id, "paths", len(paths))
				if w.ScanPaths != nil {
					w.ScanPaths(id, paths)
				} else {
					w.scan(id)
				}
			}
		case <-hourly.C:
			// Folders that were unavailable before (a drive that's back) get watched from now on.
			if w.fs != nil {
				for root := range w.roots {
					w.addTree(root)
				}
			}
			for id := range w.libs {
				w.scan(id)
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
	w.libs, w.roots, w.watched = libs, map[string]int64{}, 0
	for id, paths := range libs {
		for _, p := range paths {
			w.roots[filepath.Clean(p)] = id
		}
	}
	w.mu.Lock()
	for id := range w.dirty {
		if _, ok := libs[id]; !ok {
			delete(w.dirty, id)
		}
	}
	w.mu.Unlock()
	if len(w.roots) == 0 {
		return
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		w.log.Warn("can't watch library folders; new files appear at the next hourly scan", "err", err)
		return
	}
	w.fs = fw
	for root := range w.roots {
		w.addTree(root)
	}
	w.log.Info("watching library folders for changes", "folders", w.watched)
}

// addTree watches a folder and every folder below it.
func (w *Watcher) addTree(root string) {
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if p != root && skipDir(d.Name()) {
			return fs.SkipDir
		}
		if err := w.fs.Add(p); err != nil {
			w.log.Warn("can't watch folder; its changes appear at the next hourly scan", "path", p, "err", err)
			return fs.SkipDir
		}
		w.watched++
		return nil
	})
}

func skipDir(name string) bool {
	return strings.HasPrefix(name, ".") || ignoredDirs[strings.ToLower(name)]
}

func (w *Watcher) handle(ev fsnotify.Event) {
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
