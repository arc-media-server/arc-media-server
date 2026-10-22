package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWritesQueueBeyondBusyTimeoutWhileReadsContinue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	st := openTest(t)
	if err := st.SetSetting(ctx, "contention", "before"); err != nil {
		t.Fatal(err)
	}
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE settings SET value = 'held' WHERE key = 'contention'`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- st.SetSetting(ctx, "contention", "after") }()
	// A pending writer must not prevent a browser read of committed data.
	readCtx, readCancel := context.WithTimeout(ctx, time.Second)
	defer readCancel()
	if got, err := st.Setting(readCtx, "contention"); err != nil || got != "before" {
		t.Fatalf("read during write: %q, %v", got, err)
	}
	// Exceed the real production busy timeout: queueing must replace lock contention,
	// rather than merely increasing SQLite's timeout or serializing reads as well.
	select {
	case err := <-done:
		t.Fatalf("write failed instead of waiting for the active transaction: %v", err)
	case <-time.After(11 * time.Second):
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got, err := st.Setting(ctx, "contention"); err != nil || got != "after" {
		t.Fatalf("queued write was lost: %q, %v", got, err)
	}
}

func TestQueuedWriteCancellationDoesNotApplyLater(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	if err := st.SetSetting(ctx, "cancelled-write", "before"); err != nil {
		t.Fatal(err)
	}
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	writeCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	if err := st.SetSetting(writeCtx, "cancelled-write", "must not be saved"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting write did not respect cancellation: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if got, err := st.Setting(ctx, "cancelled-write"); err != nil || got != "before" {
		t.Fatalf("cancelled write applied: %q, %v", got, err)
	}
	if err := st.SetSetting(ctx, "cancelled-write", "recovered"); err != nil {
		t.Fatal(err)
	}
}
