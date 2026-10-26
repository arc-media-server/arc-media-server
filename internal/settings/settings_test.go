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

func TestSchedulingModesLegacyMigrationAndMinuteIntervals(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "schedule.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for k, v := range map[string]string{"scan_interval_hours": "6", "previews_at_night": "1", "detect_intros": "0"} {
		if err := st.SetSetting(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Load(ctx, st)
	if err != nil {
		t.Fatal(err)
	}
	if s.ScanMinutes() != 360 || s.PreviewMode != "scheduled" || s.DetectionMode != "off" || !s.AnalysisEnabled || !s.MetadataRefreshEnabled || !s.AutoBackupsEnabled {
		t.Fatalf("legacy defaults: %+v", s)
	}
	for _, minutes := range []int{0, 15, 30, 60, 120, 360, 720, 1440} {
		s.ScanIntervalMinutes = minutes
		s.ScanIntervalHours = minutes / 60
		s.PreviewMode = "off"
		s.DetectionMode = "added"
		s.AnalysisEnabled = false
		s.MetadataRefreshEnabled = false
		s.AutoBackupsEnabled = false
		if err := Save(ctx, st, s); err != nil {
			t.Fatal(err)
		}
		s, err = Load(ctx, st)
		if err != nil {
			t.Fatal(err)
		}
		if s.ScanMinutes() != minutes || s.PreviewMode != "off" || s.DetectionMode != "added" || s.AnalysisEnabled || s.MetadataRefreshEnabled || s.AutoBackupsEnabled {
			t.Fatalf("lost explicit settings: %+v", s)
		}
	}
}

func TestEmptyTrashDefaultsOffAndPersists(t *testing.T) {
	st, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "trash.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg, err := Load(t.Context(), st)
	if err != nil || cfg.EmptyTrashEnabled {
		t.Fatalf("unsafe default: %+v %v", cfg, err)
	}
	for _, v := range []bool{true, false} {
		cfg.EmptyTrashEnabled = v
		if err := Save(t.Context(), st, cfg); err != nil {
			t.Fatal(err)
		}
		loaded, err := Load(t.Context(), st)
		if err != nil || loaded.EmptyTrashEnabled != v {
			t.Fatalf("persist=%v %v", loaded.EmptyTrashEnabled, err)
		}
	}
}

func TestUpkeepDefaultsOnAndOffPersists(t *testing.T) {
	st, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "upkeep.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg, err := Load(t.Context(), st)
	if err != nil || !cfg.DatabaseUpkeepEnabled || !cfg.CacheCleanupEnabled {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	cfg.DatabaseUpkeepEnabled = false
	cfg.CacheCleanupEnabled = false
	if err := Save(t.Context(), st, cfg); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(t.Context(), st)
	if err != nil || cfg.DatabaseUpkeepEnabled || cfg.CacheCleanupEnabled {
		t.Fatalf("off lost: %+v %v", cfg, err)
	}
}
