package settings

import (
	"context"
	"path/filepath"
	"testing"

	"ams/internal/store"
)

func TestScanDefaultsAndExplicitOffPersist(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "test.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s, err := Load(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if !s.ScanAutomatic || !s.ScanPartial || !s.ScanOnStart || s.ScanIntervalHours != 1 {
		t.Fatalf("defaults: %+v", s)
	}
	s.ScanAutomatic, s.ScanPartial, s.ScanOnStart, s.ScanIntervalHours = false, false, false, 0
	if err := Save(ctx, st, s); err != nil {
		t.Fatal(err)
	}
	s, err = Load(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if s.ScanAutomatic || s.ScanPartial || s.ScanOnStart || s.ScanIntervalHours != 0 {
		t.Fatalf("explicit Off lost: %+v", s)
	}
	for _, hours := range []int{1, 6, 12, 24} {
		s.ScanIntervalHours = hours
		if err := Save(ctx, st, s); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(ctx, st)
		if err != nil || loaded.ScanIntervalHours != hours {
			t.Fatalf("interval %d: %+v %v", hours, loaded, err)
		}
	}
}
