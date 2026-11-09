package api

import (
	"context"
	"errors"
	"sync"
)

const maxBulkRefreshTitles = 5000

var errBulkRefreshFull = errors.New("bulk metadata refresh queue is full")

type bulkRefreshQueue struct {
	mu      sync.Mutex
	server  *Server
	ctx     context.Context
	workers *sync.WaitGroup
	owners  map[string]*bulkRefreshTicket
	pending []string
	running bool
}
type bulkRefreshTicket struct {
	queue              *bulkRefreshQueue
	ids                []string
	remaining          int
	committed, aborted bool
	release            func()
}

func (s *Server) bulkRefreshWork() *bulkRefreshQueue {
	s.bulkMu.Lock()
	defer s.bulkMu.Unlock()
	if s.bulkQueue == nil {
		ctx := s.workContext
		if ctx == nil {
			ctx = context.Background()
		}
		workers := s.workers
		if workers == nil {
			workers = &sync.WaitGroup{}
		}
		s.bulkQueue = &bulkRefreshQueue{server: s, ctx: ctx, workers: workers, owners: map[string]*bulkRefreshTicket{}}
	}
	return s.bulkQueue
}
func (q *bulkRefreshQueue) reserve(ids []string) (*bulkRefreshTicket, error) {
	if err := q.ctx.Err(); err != nil {
		return nil, err
	}
	release, ok := q.server.store.BeginCatalogWork()
	if !ok {
		return nil, errors.New("catalog maintenance in progress")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	ticket := &bulkRefreshTicket{queue: q, release: release}
	seen := map[string]bool{}
	for _, id := range ids {
		if q.owners[id] == nil && !seen[id] {
			ticket.ids = append(ticket.ids, id)
			seen[id] = true
		}
	}
	if len(q.owners)+len(ticket.ids) > maxBulkRefreshTitles {
		release()
		return nil, errBulkRefreshFull
	}
	ticket.remaining = len(ticket.ids)
	for _, id := range ticket.ids {
		q.owners[id] = ticket
	}
	return ticket, nil
}
func (t *bulkRefreshTicket) abort() {
	q := t.queue
	q.mu.Lock()
	defer q.mu.Unlock()
	if t.committed || t.aborted {
		return
	}
	t.aborted = true
	for _, id := range t.ids {
		if q.owners[id] == t {
			delete(q.owners, id)
		}
	}
	t.remaining = 0
	t.release()
}
func (t *bulkRefreshTicket) commit() {
	q := t.queue
	q.mu.Lock()
	defer q.mu.Unlock()
	if t.committed || t.aborted {
		return
	}
	t.committed = true
	if q.ctx.Err() != nil {
		for _, id := range t.ids {
			if q.owners[id] == t {
				delete(q.owners, id)
			}
		}
		t.remaining = 0
		t.release()
		return
	}
	if t.remaining == 0 {
		t.release()
		return
	}
	q.pending = append(q.pending, t.ids...)
	if !q.running {
		q.running = true
		q.workers.Add(1)
		go q.run()
	}
}
func (q *bulkRefreshQueue) complete(id string, expected *bulkRefreshTicket) {
	q.mu.Lock()
	defer q.mu.Unlock()
	owner := q.owners[id]
	if owner == nil || owner != expected {
		return
	}
	delete(q.owners, id)
	owner.remaining--
	if owner.remaining == 0 {
		owner.release()
	}
}
func (q *bulkRefreshQueue) run() {
	defer q.workers.Done()
	for {
		q.mu.Lock()
		if len(q.pending) == 0 {
			q.running = false
			q.mu.Unlock()
			return
		}
		batch := q.pending
		expected := map[string]*bulkRefreshTicket{}
		for _, id := range batch {
			expected[id] = q.owners[id]
		}
		q.pending = nil
		q.mu.Unlock()
		q.server.refreshItems(q.ctx, batch, func(id string) { q.complete(id, expected[id]) })
	}
}
