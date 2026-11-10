package store

import (
	"context"
	"errors"
	"testing"
)

func TestStaleSessionCleanupCountsAndKeepsBoundary(t *testing.T) {
	s := openTest(t)
	for token, seen := range map[string]int64{"expired-a": 998, "expired-b": 999, "boundary": 1000, "active": 1001} {
		if err := s.CreateSession(t.Context(), token, "", "password", ""); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(t.Context(), `UPDATE sessions SET last_seen_at=? WHERE token_hash=?`, seen, token); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := s.DeleteStaleSessions(t.Context(), 1000)
	if err != nil || removed != 2 {
		t.Fatal("cleanup outcome", removed, err)
	}
	for _, token := range []string{"boundary", "active"} {
		if session, err := s.GetSession(t.Context(), token); err != nil || session == nil {
			t.Fatal("live session removed", token, err)
		}
	}
	if removed, err = s.DeleteStaleSessions(t.Context(), 1000); err != nil || removed != 0 {
		t.Fatal("empty cleanup outcome", removed, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if removed, err = s.DeleteStaleSessions(ctx, 2000); !errors.Is(err, context.Canceled) || removed != 0 {
		t.Fatal("cancelled cleanup", removed, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if removed, err = s.DeleteStaleSessions(t.Context(), 2000); err == nil || removed != 0 {
		t.Fatal("database failure swallowed", removed, err)
	}
}
