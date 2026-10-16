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

// Run watches the libraries' folders until ctx is cancelled, calling scan(libraryID) when a
// library has changed and settled, and for every library once per periodic interval.
func Run(ctx context.Context, libraries map[int64][]string, scan func(libraryID int64), log *slog.Logger) {
	w := &watcher{roots: map[string]int64{}, dirty: map[int64]time.Time{}, log: log}
	for id, paths := range libraries {
		for _, p := range paths {
			w.roots[filepath.Clean(p)] = id
		}
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		log.Warn("can't watch library folders; new files appear at the next hourly scan", "err", err)
	} else {
		defer fw.Close()
		w.fs = fw
		for root := range w.roots {
			w.addTree(root)
		}
		log.Info("watching library folders for changes", "folders", w.watched)
	}

	var events <-chan fsnotify.Event
	var errs <-chan error
	if w.fs != nil {
		events, errs = w.fs.Events, w.fs.Errors
	}
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	hourly := time.NewTicker(periodic)
	defer hourly.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			w.handle(ev)
		case err := <-errs:
			log.Warn("folder watching error", "err", err)
		case <-tick.C:
			for _, id := range w.settled(time.Now()) {
				log.Info("library folders changed; scanning", "library", id)
				scan(id)
			}
		case <-hourly.C:
			// Folders that were unavailable before (a drive that's back) get watched from now on.
			if w.fs != nil {
				for root := range w.roots {
					w.addTree(root)
				}
			}
			for id := range libraries {
				scan(id)
			}
		}
	}
}

type watcher struct {
	fs      *fsnotify.Watcher
	roots   map[string]int64 // library folder -> library id
	log     *slog.Logger
	watched int

	mu    sync.Mutex
	dirty map[int64]time.Time // library id -> last change
}

// addTree watches a folder and every folder below it.
func (w *watcher) addTree(root string) {
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

func (w *watcher) handle(ev fsnotify.Event) {
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
	w.dirty[lib] = time.Now()
	w.mu.Unlock()
}

// library finds the library a path is in: the one with the longest matching folder.
func (w *watcher) library(path string) (int64, bool) {
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
func (w *watcher) settled(now time.Time) []int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []int64
	for id, last := range w.dirty {
		if now.Sub(last) >= settle {
			out = append(out, id)
			delete(w.dirty, id)
		}
	}
	return out
}
