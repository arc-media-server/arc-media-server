package backup

import (
	"context"
	"log/slog"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"ams/internal/store"
)

func TestBackupAndRestore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	m := New(filepath.Join(dir, "backups"), st, slog.New(slog.DiscardHandler))

	if _, err := st.SyncLibrary(ctx, "Movies", "movies", "tvdb_aired", []string{"/movies"}); err != nil {
		t.Fatal(err)
	}
	b, err := m.Create(ctx, ReasonManual)
	if err != nil {
		t.Fatal(err)
	}
	if b.Reason != ReasonManual || b.Size == 0 {
		t.Fatalf("unexpected backup %+v", b)
	}

	// Change things after the backup, then restore it.
	if _, err := st.SyncLibrary(ctx, "TV", "shows", "tvdb_aired", []string{"/tv"}); err != nil {
		t.Fatal(err)
	}
	if err := m.Restore(ctx, b.Name); err != nil {
		t.Fatal(err)
	}
	libs, err := st.ListLibraries(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(libs) != 1 || libs[0].Name != "Movies" {
		t.Fatalf("after restore, libraries = %+v; want just Movies", libs)
	}

	// The state before the restore was kept, so the restore can be undone.
	list, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]int{}
	for _, b := range list {
		reasons[b.Reason]++
	}
	if reasons[ReasonManual] != 1 || reasons[ReasonBeforeRestore] != 1 {
		t.Fatalf("backups = %+v; want one manual and one before-restore", list)
	}

	if err := m.Restore(ctx, "../ams.db"); err == nil {
		t.Fatal("restoring an arbitrary path should be refused")
	}
}

func TestExpired(t *testing.T) {
	start := time.Date(2026, 1, 1, 3, 0, 0, 0, time.Local)
	var all []Backup
	// 60 daily automatic backups, a manual one and five safety copies.
	for d := range 60 {
		all = append(all, Backup{Name: "auto", Reason: ReasonAuto, CreatedAt: start.AddDate(0, 0, d)})
	}
	all = append(all, Backup{Name: "manual", Reason: ReasonManual, CreatedAt: start})
	for d := range 5 {
		all = append(all, Backup{Name: "safety", Reason: ReasonBeforeRestore, CreatedAt: start.AddDate(0, 0, d)})
	}
	// expired expects newest first, as List returns them.
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })

	gone := map[string]int{}
	for _, b := range expired(all) {
		gone[b.Name]++
	}
	if want := 60 - keepDaily - keepWeekly; gone["auto"] != want {
		t.Errorf("expired %d automatic backups, want %d", gone["auto"], want)
	}
	if gone["manual"] != 0 {
		t.Error("manual backups must never expire")
	}
	if want := 5 - keepSafety; gone["safety"] != want {
		t.Errorf("expired %d safety backups, want %d", gone["safety"], want)
	}
}
