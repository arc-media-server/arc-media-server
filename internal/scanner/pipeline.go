package scanner

import (
	"context"
	"sync"
)

type movieQueueKey struct{}

// A bounded discovery/matching pipeline. Always drain/join before returning from a
// scan, including on errors, so no late metadata writes outlive recovery/sweeping.
type matchPipeline struct {
	ctx  context.Context
	jobs chan func() error
	wg   sync.WaitGroup
	once sync.Once
	mu   sync.Mutex
	err  error
}

func newMatchPipeline(ctx context.Context) *matchPipeline {
	p := &matchPipeline{ctx: ctx, jobs: make(chan func() error, matchWorkers*2)}
	for range matchWorkers {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for fn := range p.jobs {
				if ctx.Err() != nil {
					continue
				}
				if err := fn(); err != nil {
					p.mu.Lock()
					if p.err == nil {
						p.err = err
					}
					p.mu.Unlock()
				}
			}
		}()
	}
	return p
}
func (p *matchPipeline) offer(fn func() error) error {
	select {
	case p.jobs <- fn:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}
func (p *matchPipeline) finish() error {
	p.once.Do(func() { close(p.jobs); p.wg.Wait() })
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ctx.Err() != nil {
		return p.ctx.Err()
	}
	return p.err
}
