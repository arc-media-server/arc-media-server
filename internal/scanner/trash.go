package scanner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"ams/internal/store"
)

var ErrTrashUnsafe = errors.New("automatic empty trash needs a fresh complete scan and a readable known file in every unchanged library folder")
var ErrTrashBackup = errors.New("couldn't back up first, so nothing was removed")

// EmptyTrash shares discovery's exclusive library slot. before must take the safety backup;
// its failure aborts deletion. Automatic callers additionally supply their live scheduling gate.
func (s *Scanner) EmptyTrash(ctx context.Context, id int64, automatic bool, before func(context.Context) error, allowed func() bool) (removed int64, err error) {
	releaseCatalog, ok := s.store.BeginCatalogWork()
	if !ok {
		return 0, ErrLibraryBusy
	}
	defer releaseCatalog()
	s.mu.Lock()
	if st := s.status[id]; st != nil && st.Running {
		s.mu.Unlock()
		return 0, ErrLibraryBusy
	}
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return 0, err
	}
	proof := append([]string(nil), s.trashReady[id]...)
	work, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	s.nextJob++
	s.jobs[id] = &jobControl{id: s.nextJob, ctx: work, cancel: cancel}
	s.status[id] = &Status{JobID: s.nextJob, Operation: "empty_trash", Running: true, StartedAt: time.Now(), Phase: "Emptying trash"}
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	defer func() {
		stop()
		cancel()
		s.update(id, func(st *Status) {
			st.FinishedAt = time.Now()
			if st.Stopping || errors.Is(err, context.Canceled) {
				st.Stopped, st.Stopping, st.Phase = true, false, "Stopped"
				delete(s.trashReady, id)
			} else if errors.Is(err, ErrTrashUnsafe) {
				st.Phase = "Automatic cleanup skipped"
			} else if err != nil {
				st.Error = err.Error()
			}
		})
		s.finishLibraryWork(id)
	}()
	lib, err := s.store.GetLibrary(work, id)
	if err != nil {
		return 0, err
	}
	if lib == nil {
		return 0, fmt.Errorf("library %d not found", id)
	}
	check := func() error {
		if err := work.Err(); err != nil {
			return err
		}
		if !automatic {
			return nil
		}
		if allowed == nil || !allowed() || len(proof) == 0 || !slices.Equal(proof, lib.Paths) {
			return ErrTrashUnsafe
		}
		current, err := s.store.GetLibrary(work, id)
		if err != nil {
			return err
		}
		if current == nil || !slices.Equal(proof, current.Paths) {
			return ErrTrashUnsafe
		}
		missing, err := s.store.MissingFiles(work, id)
		if err != nil {
			return err
		}
		for _, f := range missing {
			if _, err := os.Stat(f.Path); !os.IsNotExist(err) {
				// Files returning since the scan must be reconciled, not forgotten.
				return ErrTrashUnsafe
			}
		}
		return s.checkTrashRoots(work, lib)
	}
	if err = check(); err != nil {
		return 0, err
	}
	missing, err := s.store.MissingFiles(work, id)
	if err != nil {
		return 0, err
	}
	if len(missing) == 0 {
		s.mu.Lock()
		delete(s.trashReady, id)
		s.mu.Unlock()
		return 0, nil
	}
	task := s.Activity.Start("empty_trash", "Emptying trash in "+lib.Name)
	s.registerTask(id, task)
	defer func() { s.mu.Lock(); delete(s.tasks, id); s.mu.Unlock(); task.Finish(err) }()
	task.Detail("Taking a safety backup before removing missing records")
	if before == nil {
		return 0, errors.New("a safety backup is required")
	}
	if err = before(work); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrTrashBackup, err)
	}
	// A root can disappear while the backup runs. Recheck immediately before the transaction.
	if err = check(); err != nil {
		return 0, err
	}
	titles, err := s.store.TrashTitleSample(work, id)
	if err != nil {
		return 0, err
	}
	removed, err = s.store.EmptyTrash(work, id)
	if err == nil {
		s.mu.Lock()
		delete(s.trashReady, id)
		s.mu.Unlock()
		detail := fmt.Sprintf("Removed %d missing titles and %d missing file records; safety backup saved; media files untouched", removed, len(missing))
		if len(titles) > 0 {
			detail += " · " + strings.Join(titles, ", ")
			if removed > int64(len(titles)) {
				detail += fmt.Sprintf(" and %d more", removed-int64(len(titles)))
			}
		}
		task.Detail(detail)
		s.log.Info("emptied library trash", "library", lib.Name, "automatic", automatic, "titles", removed, "files", len(missing))
	}
	return removed, err
}

// A readable directory alone may be an unmounted drive's empty mount point. Require an
// existing saved file in each root too. Legitimately emptied roots need manual confirmation.
func (s *Scanner) checkTrashRoots(ctx context.Context, lib *store.Library) error {
	files, err := s.store.PresentLibraryFiles(ctx, lib.ID)
	if err != nil {
		return err
	}
	for _, root := range lib.Paths {
		d, err := os.Open(root)
		if err != nil {
			return ErrTrashUnsafe
		}
		_, readErr := d.ReadDir(1)
		d.Close()
		if readErr != nil {
			return ErrTrashUnsafe
		}
		anchor := false
		for _, f := range files {
			rel, err := filepath.Rel(root, f.Path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				continue
			}
			h, err := os.Open(f.Path)
			if err != nil {
				continue
			}
			info, err := h.Stat()
			h.Close()
			if err == nil && info.Mode().IsRegular() && info.Size() == f.Size {
				anchor = true
				break
			}
		}
		if !anchor {
			return ErrTrashUnsafe
		}
	}
	return ctx.Err()
}

// TrashCandidates contains only proofs produced in this server lifetime.
func (s *Scanner) TrashCandidates() []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]int64, 0, len(s.trashReady))
	for id := range s.trashReady {
		ids = append(ids, id)
	}
	return ids
}
