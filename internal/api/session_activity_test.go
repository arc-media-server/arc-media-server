package api

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/auth"
	"ams/internal/store"
)

func TestOldSessionActivityDoesNotWaitForWriter(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := store.Open(ctx, path, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	u, err := st.CreateUser(ctx, "Admin", true, "violet", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"stale", "revoked"} {
		if err := st.CreateSession(ctx, auth.HashToken(token), u.ID, "password", "fixture"); err != nil {
			t.Fatal(err)
		}
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := time.Now().Add(-6 * time.Minute).Unix()
	if _, err := db.Exec(`UPDATE sessions SET last_seen_at=?`, old); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSession(ctx, auth.HashToken("revoked")); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	s := &Server{store: st, activity: activity.New()}
	for _, token := range []string{"stale", "invalid", "revoked"} {
		requestCtx, cancel := context.WithCancel(ctx)
		r := httptest.NewRequest("GET", "http://arc/api/activity", nil).WithContext(requestCtx)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		done := make(chan *httptest.ResponseRecorder, 1)
		go func() { w := httptest.NewRecorder(); s.adminOnly(s.getActivity)(w, r); done <- w }()
		select {
		case w := <-done:
			cancel()
			want := http.StatusUnauthorized
			if token == "stale" {
				want = http.StatusOK
			}
			if w.Code != want {
				t.Fatalf("%s: %d, want %d", token, w.Code, want)
			}
		case <-time.After(time.Second):
			tx.Rollback()
			<-done
			cancel()
			t.Fatal("Activity waited for the database writer")
		}
	}
	// Releasing the writer lets the already accepted bookkeeping update persist.
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		se, err := st.GetSession(ctx, auth.HashToken("stale"))
		if err != nil {
			t.Fatal(err)
		}
		if se.LastSeen > old {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("activity was never persisted")
		}
		time.Sleep(time.Millisecond)
	}
}
