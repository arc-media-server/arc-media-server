package store

import (
	"context"
	"sync"
	"time"
)

// Last-used timestamps are bookkeeping, not authorization. Keep their writes off
// authenticated requests so a scanner holding the writer cannot stall browsing.
// One worker and a bounded, deduplicated queue avoid a goroutine per request.
const sessionUseCapacity = 128

type sessionUse struct {
	hash string
	at   int64
}

type sessionUseQueue struct {
	mu      sync.Mutex
	pending map[string]bool
	closed  bool
	queue   chan sessionUse
	cancel  context.CancelFunc
	done    chan struct{}
}

func newSessionUseQueue(s *Store) *sessionUseQueue {
	ctx, cancel := context.WithCancel(context.Background())
	q := &sessionUseQueue{pending: make(map[string]bool), queue: make(chan sessionUse, sessionUseCapacity), cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(q.done)
		for {
			select {
			case <-ctx.Done():
				return
			case use := <-q.queue:
				writeCtx, stop := context.WithTimeout(ctx, 2*time.Second)
				// A deleted session stays deleted. Capture actual request time, not
				// time spent queued, and never move a newer timestamp backwards.
				s.db.ExecContext(writeCtx, `UPDATE sessions SET last_seen_at = MAX(last_seen_at, ?) WHERE token_hash = ?`, use.at, use.hash)
				stop()
				q.mu.Lock()
				delete(q.pending, use.hash)
				q.mu.Unlock()
			}
		}
	}()
	return q
}

// RecordSessionUse queues best-effort activity for an already validated session.
// Busy/full/failed queues are retried by a later request while its stored time is
// still old. Request cancellation must not cancel an accepted activity update.
func (s *Store) RecordSessionUse(hash string) {
	q := s.sessionUses
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed || q.pending[hash] || len(q.pending) >= sessionUseCapacity {
		return
	}
	q.pending[hash] = true
	select {
	case q.queue <- sessionUse{hash: hash, at: time.Now().Unix()}:
	default:
		delete(q.pending, hash)
	}
}

func (q *sessionUseQueue) stop() {
	q.mu.Lock()
	q.closed = true
	q.cancel()
	q.mu.Unlock()
	<-q.done
}
