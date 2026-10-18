package scanner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestEach(t *testing.T) {
	var done, running, most atomic.Int32
	err := each(context.Background(), 50, 4, func(i int) error {
		n := running.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		running.Add(-1)
		done.Add(1)
		return nil
	})
	if err != nil || done.Load() != 50 || most.Load() > 4 || most.Load() < 2 {
		t.Fatalf("err %v, done %d, at once %d", err, done.Load(), most.Load())
	}

	boom := errors.New("boom")
	var after atomic.Int32
	err = each(context.Background(), 1000, 3, func(i int) error {
		if i == 10 {
			return boom
		}
		if i > 100 {
			after.Add(1)
		}
		return nil
	})
	if !errors.Is(err, boom) || after.Load() > 50 {
		t.Fatalf("err %v, kept going for %d", err, after.Load())
	}
}
