package scanner

// ResumeQueued resumes only current library ids after a destructive catalog reservation ends.
func (s *Scanner) ResumeQueued() {
	if s == nil || s.store == nil {
		return
	}
	libraries, err := s.store.ListLibraries(s.ctx)
	if err != nil {
		if s.ctx.Err() == nil {
			s.log.Warn("resuming queued scans failed", "err", err)
		}
		return
	}
	valid := map[int64]bool{}
	for _, library := range libraries {
		valid[library.ID] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.trashReady)
	ids := map[int64]bool{}
	for id := range s.again {
		ids[id] = true
	}
	for id := range s.pendingPaths {
		ids[id] = true
	}
	for id := range ids {
		if !valid[id] {
			delete(s.again, id)
			delete(s.pendingPaths, id)
			delete(s.status, id)
			continue
		}
		if state := s.status[id]; state != nil && state.Running {
			continue
		}
		if s.again[id] {
			if s.startLocked(id) {
				delete(s.again, id)
				delete(s.pendingPaths, id)
			}
		} else if paths := s.pendingPaths[id]; len(paths) > 0 {
			if s.startLocked(id, paths...) {
				delete(s.pendingPaths, id)
			}
		}
	}
}

// InvalidateTrashProof drops pre-mutation discovery proofs before an exclusive catalog change.
func (s *Scanner) InvalidateTrashProof() {
	if s == nil {
		return
	}
	s.mu.Lock()
	clear(s.trashReady)
	s.mu.Unlock()
}
