package scanner

import "time"

// One background retry per library, with capped exponential delay. Pending/unsynced
// state remains in SQLite, so a restart or hourly reconciliation also retries safely.
func (s *Scanner) retryLater(id int64) {
	s.mu.Lock()
	if st := s.status[id]; st != nil && st.Stopping {
		s.mu.Unlock()
		return
	}
	if s.retryPending[id] || s.ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	s.retryPending[id] = true
	wake := make(chan struct{})
	s.retryWake[id] = wake
	epoch := s.stopEpoch[id]
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
		case <-wake:
		case <-s.ctx.Done():
		case <-timer.C:
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if epoch != s.stopEpoch[id] {
			return
		}
		delete(s.retryPending, id)
		delete(s.retryWake, id)
		if s.ctx.Err() == nil {
			if st := s.status[id]; st != nil && st.Stopping {
				return
			}
			if !s.startLocked(id) {
				s.again[id] = true
				delete(s.pendingPaths, id)
			}
		}
	}()
}
