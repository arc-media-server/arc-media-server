package scanner

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"ams/internal/activity"
	"ams/internal/store"
)

var ErrLibraryBusy = errors.New("the library is busy; retry when its current work finishes")

// RefreshSeries reserves the existing library slot, optionally saves a numbering change,
// then refreshes/relinks this show's known files only. It never discovers media or probes files.
func (s *Scanner) RefreshSeries(ctx context.Context, lib *store.Library, id string, deep bool, before func(context.Context) error) error {
	return s.refreshSeries(ctx, lib, id, deep, before, nil)
}

// refreshSeries is RefreshSeries reporting to an Activity task the caller already started (nil
// starts "Refreshing <title>"). It finishes the task, unless the library is busy: then it returns
// ErrLibraryBusy and leaves the task untouched for the caller to try again with.
func (s *Scanner) refreshSeries(ctx context.Context, lib *store.Library, id string, deep bool, before func(context.Context) error,
	task *activity.Handle) (err error) {
	s.mu.Lock()
	if st := s.status[lib.ID]; st != nil && st.Running {
		s.mu.Unlock()
		return ErrLibraryBusy
	}
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		task.Finish(err)
		return err
	}
	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	s.nextJob++
	s.jobs[lib.ID] = &jobControl{id: s.nextJob, ctx: work, cancel: cancel}
	s.status[lib.ID] = &Status{JobID: s.nextJob, Operation: "refresh", Running: true, StartedAt: time.Now(), Phase: "Refreshing selected show"}
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	defer func() {
		stop()
		cancel()
		s.update(lib.ID, func(st *Status) {
			st.FinishedAt = time.Now()
			if errors.Is(err, context.Canceled) || st.Stopping {
				st.Stopped = true
				st.Stopping = false
				st.Phase = "Stopped"
			} else if err != nil {
				st.Error = err.Error()
			}
		})
		s.finishLibraryWork(lib.ID)
		if s.OnFinish != nil {
			s.OnFinish(lib.ID)
		}
	}()

	item, e := s.store.GetItem(work, id)
	if e != nil {
		task.Finish(e)
		return e
	}
	if task == nil {
		title := id
		if item != nil {
			title = item.Title
		}
		task = s.Activity.Start("refresh", "Refreshing "+title)
	}
	s.registerTask(lib.ID, task)
	defer func() { s.mu.Lock(); delete(s.tasks, lib.ID); s.mu.Unlock(); task.Finish(err) }()
	if err = work.Err(); err != nil {
		return err
	}
	if before != nil {
		if err = before(work); err != nil {
			return err
		}
	}
	if err = s.SyncSeriesMetadata(work, lib, id, deep); err != nil {
		return err
	}
	return s.RelinkSeries(work, lib, id)
}

// RelinkSeries interprets only saved, present files attached to this show. Missing files stay
// missing, and new/unrelated files are left for explicit or automatic discovery.
func (s *Scanner) RelinkSeries(ctx context.Context, lib *store.Library, id string) error {
	episodes, err := s.store.Children(ctx, id)
	if err != nil {
		return err
	}
	files := []episodeFile{}
	seen := map[int64]bool{}
	for _, ep := range episodes {
		linked, err := s.store.ItemFiles(ctx, ep.ID)
		if err != nil {
			return err
		}
		for _, f := range linked {
			if seen[f.ID] || f.Missing {
				continue
			}
			seen[f.ID] = true
			for _, root := range lib.Paths {
				rel, err := filepath.Rel(root, f.Path)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					continue
				}
				_, _, name, ok := ParseEpisodePath(root, f.Path)
				if ok {
					files = append(files, episodeFile{fileID: f.ID, path: f.Path, ep: name})
				}
				break
			}
		}
	}
	if _, err := s.linkEpisodeFiles(ctx, lib, id, files, true); err != nil {
		return err
	}
	_, err = s.store.DeleteOrphanEpisodes(ctx, id)
	return err
}

// finishLibraryWork also releases requests queued by filesystem watching during scoped work.
func (s *Scanner) finishLibraryWork(libraryID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[libraryID].Running = false
	delete(s.jobs, libraryID)
	if s.status[libraryID].Stopped {
		delete(s.again, libraryID)
		delete(s.pendingPaths, libraryID)
	}
	if s.again[libraryID] {
		delete(s.again, libraryID)
		delete(s.pendingPaths, libraryID)
		s.startLocked(libraryID)
	} else if next := s.pendingPaths[libraryID]; len(next) > 0 {
		delete(s.pendingPaths, libraryID)
		s.startLocked(libraryID, next...)
	}
}
