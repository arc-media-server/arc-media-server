package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSessionUseQueueCoalescesBoundsAndStops(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	if err := st.CreateSession(ctx, "used", "", "password", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE sessions SET last_seen_at=1`); err != nil {
		t.Fatal(err)
	}
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < 1000; i++ {
		st.RecordSessionUse("used")
	}
	q := st.sessionUses
	q.mu.Lock()
	n := len(q.pending)
	q.mu.Unlock()
	if n != 1 {
		t.Fatalf("duplicate updates queued: %d", n)
	}
	for i := 0; i < 1000; i++ {
		st.RecordSessionUse(fmt.Sprintf("fixture-%d", i))
	}
	q.mu.Lock()
	n = len(q.pending)
	q.mu.Unlock()
	if n != sessionUseCapacity {
		t.Fatalf("queue size %d", n)
	}
	// Shutdown must cancel a worker waiting for Arc's writer, even while it is held.
	done := make(chan struct{})
	go func() { q.stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop while writer held")
	}
	st.RecordSessionUse("after-stop")
	q.mu.Lock()
	added := q.pending["after-stop"]
	q.mu.Unlock()
	if added {
		t.Fatal("accepted work after shutdown")
	}
}

func TestQueuedSessionUseCannotRecreateRevokedSession(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	if err := st.CreateSession(ctx, "revoked", "", "password", ""); err != nil {
		t.Fatal(err)
	}
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	st.RecordSessionUse("revoked")
	if _, err := tx.Exec(`DELETE FROM sessions WHERE token_hash='revoked'`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		q := st.sessionUses
		q.mu.Lock()
		pending := q.pending["revoked"]
		q.mu.Unlock()
		if !pending {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("update did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	if se, err := st.GetSession(ctx, "revoked"); err != nil || se != nil {
		t.Fatalf("session resurrected: %+v %v", se, err)
	}
}
