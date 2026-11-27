package artwork

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestKeeperCloseJoinsSourceListingBeforeStoreClose(t *testing.T) {
	cache := New(t.TempDir(), slog.New(slog.DiscardHandler))
	entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	keeper := cache.Keep(t.Context(), func(ctx context.Context) (Images, error) {
		calls.Add(1)
		close(entered)
		<-ctx.Done()
		close(canceled)
		<-release
		return Images{}, ctx.Err()
	})
	var once sync.Once
	defer func() { once.Do(func() { close(release) }); keeper.Close() }()
	keeper.Trigger()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("list did not start")
	}
	done := make(chan struct{})
	go func() { keeper.Close(); close(done) }()
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("listing did not inherit cancellation")
	}
	select {
	case <-done:
		t.Fatal("Close returned while listing still held store work")
	case <-time.After(20 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("keeper did not stop")
	}
	keeper.Trigger()
	if calls.Load() != 1 {
		t.Fatal("closed keeper performed another pass")
	}
}
