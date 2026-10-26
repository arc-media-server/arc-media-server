package markers

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"ams/internal/activity"
	"ams/internal/store"
)

// A pass with nothing to look up or detect is routine and isn't listed as finished work.
func TestIdlePassLeavesNoActivity(t *testing.T) {
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	m := New(st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.Activity = activity.New()
	if err := m.pass(context.Background()); err != nil {
		t.Fatal(err)
	}
	running, recent := m.Activity.Snapshot()
	if len(running) != 0 || len(recent) != 0 {
		t.Fatalf("idle pass listed running=%+v recent=%+v", running, recent)
	}
}
