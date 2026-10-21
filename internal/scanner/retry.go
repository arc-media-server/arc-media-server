package scanner

import "time"

// One background retry per library, with capped exponential delay. Pending/unsynced
// state remains in SQLite, so a restart or hourly reconciliation also retries safely.
func (s *Scanner) retryLater(id int64) {
	s.mu.Lock()
	if s.retryPending[id] || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.retryPending[id] = true
	base := s.retryBase
	if base == 0 {
		base = time.Minute
	}
	delay := min(15*time.Minute, base*time.Duration(1<<min(s.retryAttempts[id], 4)))
	s.retryAttempts[id]++
	s.mu.Unlock()
	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-s.ctx.Done():
		case <-timer.C:
		}
		s.mu.Lock()
		delete(s.retryPending, id)
		s.mu.Unlock()
		if s.ctx.Err() == nil {
			s.Request(id)
		}
	}()
}
