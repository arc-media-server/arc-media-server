package scanner

import (
	"context"
	"time"

	"ams/internal/metadata"
)

// maintain deliberately reads known database titles, never the media directory tree.
// The caller reserves the same per-library slot as discovery, preventing overlap.
func (s *Scanner) maintain(ctx context.Context, libraryID int64) (err error) {
	lib, err := s.store.GetLibrary(ctx, libraryID)
	if err != nil || lib == nil {
		return err
	}
	s.update(libraryID, func(st *Status) { st.libraryName, st.Phase = lib.Name, "Updating known metadata" })
	ctx = metadata.WithLanguage(ctx, lib.MetadataLanguage)
	if lib.Kind == "videos" {
		return nil
	}
	task := s.Activity.Start("refresh", "Updating known titles in "+lib.Name)
	s.mu.Lock()
	s.tasks[libraryID] = task
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.tasks, libraryID)
		s.mu.Unlock()
		task.Finish(err)
	}()
	if lib.Kind == "movies" {
		if !s.tmdb.Configured() {
			return nil
		}
		return s.refreshMovies(ctx, libraryID, false)
	}
	if !s.hasProviders() {
		return nil
	}
	items, err := s.store.ItemsWithMetadataState(ctx, libraryID, "series", "matched")
	if err != nil {
		return err
	}
	s.phase(libraryID, "Updating known show details", len(items))
	return each(ctx, len(items), matchWorkers, func(i int) error {
		defer s.step(libraryID)
		it := items[i]
		synced, err := s.store.SyncedAt(ctx, it.ID)
		if err != nil {
			return err
		}
		next, err := s.store.NextAired(ctx, it.ID)
		if err != nil {
			return err
		}
		age := time.Since(time.Unix(synced, 0))
		if synced != 0 && age <= resyncAfter && !(age > 24*time.Hour && next != "" && next <= time.Now().Format("2006-01-02")) {
			return nil
		}
		// This updates metadata/order mappings using existing identity, not a file scan.
		if err := s.SyncSeriesMetadata(ctx, lib, it.ID, false); err != nil {
			s.log.Warn("known show refresh failed", "title", it.Title, "err", err)
			return err
		}
		return nil
	})
}
