package scanner

import "context"

// WithLibraryChange shares scan/refresh admission without creating a scan status, history
// record or completion notification for a settings save. Pending watcher work is retained.
func (s *Scanner) WithLibraryChange(ctx context.Context, id int64, change func(context.Context) error) error {
	if s == nil {
		return change(ctx)
	}
	s.mu.Lock()
	if s.configuring[id] {
		s.mu.Unlock()
		return ErrLibraryBusy
	}
	if state := s.status[id]; state != nil && state.Running {
		s.mu.Unlock()
		return ErrLibraryBusy
	}
	if err := s.ctx.Err(); err != nil {
		s.mu.Unlock()
		return err
	}
	if s.configuring == nil {
		s.configuring = map[int64]bool{}
	}
	s.configuring[id] = true
	s.wg.Add(1)
	s.mu.Unlock()
	defer s.wg.Done()
	defer func() { s.mu.Lock(); delete(s.configuring, id); s.startPendingLibraryWorkLocked(id); s.mu.Unlock() }()
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	return change(work)
}
