package scanner

import (
	"ams/internal/store"
	"context"
	"sort"
	"sync"
)

type secondaryKey struct{}
type secondaryShows struct {
	mu    sync.Mutex
	files map[string][]episodeFile
}

func (q *secondaryShows) add(id string, files []episodeFile) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.files[id] = append(q.files[id], files...)
}
func (s *Scanner) finishSecondary(ctx context.Context, lib *store.Library, q *secondaryShows) error {
	var ids []string
	for id := range q.files {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil
	}
	s.phase(lib.ID, "Loading alternate episode orders", len(ids))
	return each(ctx, len(ids), matchWorkers, func(i int) error {
		defer s.step(lib.ID)
		id := ids[i]
		if err := s.SyncSeriesMetadata(ctx, lib, id, false); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.log.Warn("alternate episode orders deferred", "series", id, "err", err)
			s.retryLater(lib.ID)
			return nil // The essential, playable ordering remains intact.
		}
		_, err := s.linkEpisodeFiles(ctx, lib, id, q.files[id], true)
		return err
	})
}
