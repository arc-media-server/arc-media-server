package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ams/internal/store"
)

// manualWork deliberately ignores age/backoff and never performs discovery or missing-file cleanup.
func (s *Scanner) manualWork(ctx context.Context, libraryID int64, operation string) (err error) {
	lib, err := s.store.GetLibrary(ctx, libraryID)
	if err != nil {
		return err
	}
	if lib == nil {
		return fmt.Errorf("library not found")
	}
	if operation == "refresh" && lib.Kind == "videos" {
		return fmt.Errorf("home videos have no online metadata to refresh")
	}
	title, kind := "Refreshing metadata in "+lib.Name, "refresh"
	if operation == "analyze" {
		title, kind = "Reanalyzing files in "+lib.Name, "analysis"
	}
	task := s.Activity.Start(kind, title)
	s.registerTask(libraryID, task)
	completed, failed := 0, 0
	defer func() {
		s.mu.Lock()
		delete(s.tasks, libraryID)
		s.mu.Unlock()
		task.Detail(fmt.Sprintf("%d completed · %d failed", completed, failed))
		task.Finish(err)
	}()
	s.update(libraryID, func(st *Status) { st.libraryName, st.Phase = lib.Name, title })
	if operation == "refresh" {
		var after string
		for {
			items, e := s.store.RefreshPage(ctx, libraryID, after)
			if e != nil {
				return e
			}
			if len(items) == 0 {
				break
			}
			for _, item := range items {
				if e := ctx.Err(); e != nil {
					return e
				}
				after = item.ID
				var e error
				if item.Kind == "movie" {
					_, e = s.RefreshMovie(ctx, item.ID)
				} else {
					e = s.SyncSeriesMetadata(ctx, lib, item.ID, false)
				}
				if e != nil && ctx.Err() != nil {
					return ctx.Err()
				}
				if e != nil {
					failed++
					s.log.Warn("manual metadata refresh failed", "item", item.Title, "err", e)
				} else {
					completed++
				}
				s.update(libraryID, func(st *Status) { st.Done, st.MetadataErrors = completed+failed, failed })
			}
		}
	} else {
		var after int64
		for {
			files, e := s.store.AnalysisPage(ctx, libraryID, after)
			if e != nil {
				return e
			}
			if len(files) == 0 {
				break
			}
			for _, file := range files {
				if e := ctx.Err(); e != nil {
					return e
				}
				after = file.ID
				work, finish := s.working(ctx, libraryID, "Reanalyzing file", filepath.Base(file.Path), "Reading video, audio and HDR details")
				e := s.reanalyzeFile(work, file)
				finish()
				if e != nil && ctx.Err() != nil {
					return ctx.Err()
				}
				if e != nil {
					failed++
					s.log.Warn("file reanalysis failed", "file", file.Path, "err", e)
				} else {
					completed++
				}
				s.update(libraryID, func(st *Status) { st.Done, st.ProbeErrors = completed+failed, failed })
			}
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d completed; %d failed (see the server log)", completed, failed)
	}
	return ctx.Err()
}

func (s *Scanner) reanalyzeFile(ctx context.Context, file store.MediaFile) error {
	before, err := os.Stat(file.Path)
	if err != nil {
		return err
	}
	if before.Size() != file.Size || before.ModTime().Unix() != file.ModTime {
		return fmt.Errorf("source changed; scan the library first")
	}
	result, err := s.prober.Probe(ctx, file.Path)
	if err != nil {
		return err
	} // retain existing details on failure/cancellation
	after, err := os.Stat(file.Path)
	if err != nil {
		return err
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return fmt.Errorf("source changed during inspection; scan the library first")
	}
	file.Streams, file.Chapters, file.ProbeError = nil, nil, ""
	applyProbe(&file, result)
	_, err = s.store.SaveAnalysis(ctx, &file, time.Now().Unix())
	return err
}
