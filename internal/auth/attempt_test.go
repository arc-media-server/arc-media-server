package auth

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestLimiterAttemptConcurrent(t *testing.T) {
	l := NewLimiter()
	var admitted atomic.Int32
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if l.Attempt("same-key") {
				admitted.Add(1)
			}
		})
	}
	wg.Wait()
	if admitted.Load() != 5 {
		t.Fatal("non-atomic admission", admitted.Load())
	}
	l.Succeed("same-key")
	if !l.Attempt("same-key") {
		t.Fatal("success did not reset")
	}
}
