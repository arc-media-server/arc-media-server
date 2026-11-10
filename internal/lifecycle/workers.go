// Package lifecycle owns process work until its dependencies may safely close.
package lifecycle

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"
)

type Workers struct {
	ctx          context.Context
	cancel       context.CancelFunc
	log          *slog.Logger
	mu           sync.Mutex
	closed       bool
	active       map[string]int
	wg           sync.WaitGroup
	once         sync.Once
	drains       []drain
	drainWarning time.Duration
}

type drain struct {
	name string
	wait func()
}

func New(ctx context.Context, log *slog.Logger) *Workers {
	ctx, cancel := context.WithCancel(ctx)
	if log == nil {
		log = slog.Default()
	}
	return &Workers{ctx: ctx, cancel: cancel, log: log, active: map[string]int{}, drainWarning: 5 * time.Second}
}
func (w *Workers) Context() context.Context { return w.ctx }

// Begin reserves synchronous work (such as an HTTP request). Names are static
// operation labels, never request data. Release before closing its dependencies.
func (w *Workers) Begin(name string) (func(), bool) {
	w.mu.Lock()
	if w.closed || w.ctx.Err() != nil {
		w.mu.Unlock()
		return nil, false
	}
	w.wg.Add(1)
	w.active[name]++
	w.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			w.mu.Lock()
			w.active[name]--
			if w.active[name] == 0 {
				delete(w.active, name)
			}
			w.mu.Unlock()
			w.wg.Done()
		})
	}, true
}
func (w *Workers) Go(name string, run func()) bool {
	release, ok := w.Begin(name)
	if !ok {
		return false
	}
	go func() { defer release(); run() }()
	return true
}

// OnStop registers a dependency-specific join, run after tracked requests and loops.
func (w *Workers) OnStop(name string, wait func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		panic("register shutdown drain before closing workers")
	}
	w.drains = append(w.drains, drain{name: name, wait: wait})
}

// Close stops admission/cancels work and waits for every reservation. A slow
// cancellation gets one bounded diagnostic; dependencies stay open until it exits.
func (w *Workers) Close() {
	w.once.Do(func() {
		w.mu.Lock()
		w.closed = true
		w.cancel()
		w.mu.Unlock()
		done := make(chan struct{})
		go func() {
			w.wg.Wait()
			for _, d := range w.drains {
				w.mu.Lock()
				w.active[d.name]++
				w.mu.Unlock()
				d.wait()
				w.mu.Lock()
				delete(w.active, d.name)
				w.mu.Unlock()
			}
			close(done)
		}()
		timer := time.NewTimer(w.drainWarning)
		defer timer.Stop()
		select {
		case <-done:
			return
		case <-timer.C:
			w.mu.Lock()
			names := make([]string, 0, len(w.active))
			count := 0
			for name, n := range w.active {
				names = append(names, name)
				count += n
			}
			w.mu.Unlock()
			slices.Sort(names)
			if count > 0 {
				w.log.Warn("shutdown waiting for workers", "active", count, "operations", names)
			}
			<-done
		}
	})
}
