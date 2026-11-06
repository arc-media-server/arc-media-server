package scanner

import (
	"ams/internal/events"
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
	s.log.Info("library work started", "work", title)
	completed, failed := 0, 0
	defer func() {
		s.mu.Lock()
		delete(s.tasks, libraryID)
		s.mu.Unlock()
		task.Detail(fmt.Sprintf("%d completed · %d failed", completed, failed))
		task.Finish(err)
		s.log.Info("library work finished", "work", title, "completed", completed, "failed", failed, "err", err)
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
					if e == nil {
						e = s.RelinkSeries(ctx, lib, item.ID)
					}
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

// analyzeItem is the More menu's Analyze: the library's reanalysis, for one title's files.
func (s *Scanner) analyzeItem(ctx context.Context, libraryID int64, itemID string) (err error) {
	it, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return err
	}
	if it == nil || it.LibraryID != libraryID {
		return fmt.Errorf("title not found")
	}
	files, err := s.store.ItemAnalysisFiles(ctx, itemID)
	if err != nil {
		return err
	}
	name := it.Title
	if it.Kind == "episode" && len(files) > 0 {
		if label, e := s.store.FileActivityLabel(ctx, files[0].ID); e == nil && label != "" {
			name = label
		}
	}
	title := "Analyzing " + name
	task := s.Activity.Start("analysis", title)
	s.registerTask(libraryID, task)
	s.log.Info("title analysis started", "title", name, "files", len(files))
	completed, failed := 0, 0
	defer func() {
		s.mu.Lock()
		delete(s.tasks, libraryID)
		s.mu.Unlock()
		task.Detail(fmt.Sprintf("%d analysed · %d failed", completed, failed))
		task.Finish(err)
		s.log.Info("title analysis finished", "title", name, "completed", completed, "failed", failed, "err", err)
	}()
	s.update(libraryID, func(st *Status) { st.Phase, st.Done, st.Total = title, 0, len(files) })
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		e := s.reanalyzeFile(ctx, file)
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
	if failed > 0 {
		return fmt.Errorf("%d analysed; %d failed (see the server log)", completed, failed)
	}
	return nil
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
	if err == nil {
		s.Events.Publish(events.Event{Topic: "catalog", LibraryID: file.LibraryID})
	}
	return err
}
