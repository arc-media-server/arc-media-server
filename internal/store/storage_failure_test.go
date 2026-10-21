package store

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestSQLiteFullWriteIsAtomicAndStorageCanRecover(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	// max_page_count is connection-local. Keep the test on one connection so its
	// simulated capacity limit cannot be bypassed by the connection pool.
	st.db.SetMaxOpenConns(1)
	if err := st.SetSetting(ctx, "fixture", "before"); err != nil {
		t.Fatal(err)
	}
	var pages int
	if err := st.db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(fmt.Sprintf("PRAGMA max_page_count=%d", pages)); err != nil {
		t.Fatal(err)
	}
	large := strings.Repeat("x", 4<<20)
	err := st.SetSetting(ctx, "fixture", large)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "full") {
		t.Fatalf("expected SQLite full error, got %v", err)
	}
	got, err := st.Setting(ctx, "fixture")
	if err != nil || got != "before" {
		t.Fatalf("failed write partially changed setting: %q %v", got, err)
	}
	var integrity string
	if err := st.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%q err=%v", integrity, err)
	}
	if _, err := st.db.Exec(`PRAGMA max_page_count=1073741823`); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSetting(ctx, "fixture", large); err != nil {
		t.Fatal(err)
	}
	got, err = st.Setting(ctx, "fixture")
	if err != nil || got != large {
		t.Fatal("write did not recover after capacity was restored", err)
	}
}
