package scanner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEach(t *testing.T) {
	const total, workers = 50, 4
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	started := make(chan struct{}, workers)
	result := make(chan error, 1)
	var calls [total]atomic.Int32
	var running atomic.Int32
	var exceeded atomic.Bool
	go func() {
		result <- each(ctx, total, workers, func(i int) error {
			if running.Add(1) > workers {
				exceeded.Store(true)
			}
			defer running.Add(-1)
			calls[i].Add(1)
			if i < workers {
				started <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		})
	}()
	// Hold the first wave so reaching the worker limit does not depend on sleeps or CPU count.
	for range workers {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("workers did not start:", ctx.Err())
		}
	}
	if got := running.Load(); got != workers {
		t.Fatalf("running %d, want %d", got, workers)
	}
	unblock()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("workers did not finish:", ctx.Err())
	}
	if exceeded.Load() || running.Load() != 0 {
		t.Fatalf("worker limit exceeded %v, still running %d", exceeded.Load(), running.Load())
	}
	for i := range calls {
		if got := calls[i].Load(); got != 1 {
			t.Fatalf("index %d called %d times, want once", i, got)
		}
	}
}

func TestEachStopsOnError(t *testing.T) {
	boom := errors.New("boom")
	var calls int
	// One worker cannot accept another index after returning the error. With several workers,
	// later indices can finish before the failing callback is even scheduled, so counting them
	// is not evidence that dispatch continued after an error.
	err := each(t.Context(), 1000, 1, func(i int) error {
		calls++
		if i == 10 {
			return boom
		}
		return nil
	})
	if !errors.Is(err, boom) || calls != 11 {
		t.Fatalf("err %v, calls %d, want boom and 11 calls", err, calls)
	}
}

func TestEachDelayedError(t *testing.T) {
	const total = 40
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	boom := errors.New("boom")
	release := make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	othersDone := make(chan struct{})
	result := make(chan error, 1)
	var completed atomic.Int32
	go func() {
		result <- each(ctx, total, 3, func(i int) error {
			if i == 10 {
				select {
				case <-release:
					return boom
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			if completed.Add(1) == total-1 {
				close(othersDone)
			}
			return nil
		})
	}()
	// All other work can legitimately finish while the error callback is held. Its eventual
	// error must still be returned, even when every index has already been dispatched.
	select {
	case <-othersDone:
	case <-ctx.Done():
		t.Fatal("other workers did not finish:", ctx.Err())
	}
	select {
	case err := <-result:
		t.Fatalf("returned before the held callback completed: %v", err)
	default:
	}
	unblock()
	select {
	case err := <-result:
		if !errors.Is(err, boom) {
			t.Fatalf("err %v, want boom", err)
		}
	case <-ctx.Done():
		t.Fatal("error was not returned:", ctx.Err())
	}
}
