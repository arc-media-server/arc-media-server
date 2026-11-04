package backup

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"ams/internal/store"
)

func TestBackupStorageFailureRecovers(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.SyncLibrary(ctx, "Keep me", "movies", "tmdb_aired", []string{"/fixture"}); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(dir, "backups")
	// A file obstructing the directory reliably simulates unavailable storage on Windows
	// and Linux without changing permissions on any real disk.
	if err := os.WriteFile(blocked, []byte("obstruction"), 0600); err != nil {
		t.Fatal(err)
	}
	m := New(blocked, st, slog.New(slog.DiscardHandler))
	if _, err := m.Create(ctx, ReasonManual); err == nil {
		t.Fatal("backup unexpectedly succeeded")
	}
	libs, err := st.ListLibraries(ctx)
	if err != nil || len(libs) != 1 || libs[0].Name != "Keep me" {
		t.Fatalf("live data changed: %v %v", libs, err)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	b, err := m.Create(ctx, ReasonManual)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CheckBackup(ctx, filepath.Join(blocked, b.Name)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(blocked)
	if err != nil || len(entries) != 1 {
		t.Fatalf("incomplete backup files remain: %v %v", entries, err)
	}
}

func TestInvalidRestoreLeavesLiveData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.SyncLibrary(ctx, "Keep me", "movies", "tmdb_aired", []string{"/fixture"}); err != nil {
		t.Fatal(err)
	}
	m := New(filepath.Join(dir, "backups"), st, slog.New(slog.DiscardHandler))
	if err := os.MkdirAll(m.Dir(), 0755); err != nil {
		t.Fatal(err)
	}
	name := "ams-20260101-000000-manual.db"
	if err := os.WriteFile(filepath.Join(m.Dir(), name), []byte("not SQLite"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Restore(ctx, name); err == nil {
		t.Fatal("corrupt backup accepted")
	}
	libs, err := st.ListLibraries(ctx)
	if err != nil || len(libs) != 1 || libs[0].Name != "Keep me" {
		t.Fatalf("live data changed: %v %v", libs, err)
	}
	entries, err := os.ReadDir(m.Dir())
	if err != nil || len(entries) != 1 {
		t.Fatalf("rejected restore created files: %v %v", entries, err)
	}
}

func TestRestoreAndUndoPreserveSettingsEditsAndResume(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, filepath.Join(dir, "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Original"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	set := func(title string, position int64) {
		t.Helper()
		if err := st.EditItem(ctx, id, store.ItemEdit{Title: &title}); err != nil {
			t.Fatal(err)
		}
		if err := st.SetSetting(ctx, "server_name", title); err != nil {
			t.Fatal(err)
		}
		if _, err := st.ReportProgress(ctx, u.ID, id, position, 7200000); err != nil {
			t.Fatal(err)
		}
	}
	check := func(title string, position int64) {
		t.Helper()
		it, err := st.GetItem(ctx, id)
		if err != nil || it == nil || it.Title != title || len(it.LockedFields) != 1 || it.LockedFields[0] != "title" {
			t.Fatalf("item=%+v err=%v", it, err)
		}
		name, err := st.Setting(ctx, "server_name")
		if err != nil || name != title {
			t.Fatalf("setting=%q err=%v", name, err)
		}
		data, err := st.UserDataFor(ctx, u.ID, []string{id})
		if err != nil || data[id].PositionMS != position {
			t.Fatalf("resume=%+v err=%v", data, err)
		}
	}
	set("Backed up", 120000)
	m := New(filepath.Join(dir, "backups"), st, slog.New(slog.DiscardHandler))
	b, err := m.Create(ctx, ReasonManual)
	if err != nil {
		t.Fatal(err)
	}
	set("Before restore", 240000)
	if err := m.Restore(ctx, b.Name); err != nil {
		t.Fatal(err)
	}
	check("Backed up", 120000)
	list, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	var safety string
	for _, b := range list {
		if b.Reason == ReasonBeforeRestore {
			safety = b.Name
		}
	}
	if safety == "" {
		t.Fatal("missing undo backup")
	}
	if err := m.Restore(ctx, safety); err != nil {
		t.Fatal(err)
	}
	check("Before restore", 240000)
}

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

func TestFreshSafetyBackupSurvivesFutureDatedRetention(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(t.Context(), filepath.Join(root, "arc.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	m := New(filepath.Join(root, "backups"), st, slog.New(slog.DiscardHandler))
	if err = os.MkdirAll(m.Dir(), 0700); err != nil {
		t.Fatal(err)
	}
	// Existing backups may have timestamps ahead of the current clock after a clock correction.
	for day := 1; day <= keepSafety; day++ {
		name := "ams-2099010" + string(rune('0'+day)) + "-000000-before-upgrade.db"
		if err = os.WriteFile(filepath.Join(m.Dir(), name), []byte("retained fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b, err := m.Create(t.Context(), ReasonBeforeUpgrade)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CheckBackup(t.Context(), filepath.Join(m.Dir(), b.Name)); err != nil {
		t.Fatal("fresh safety copy was pruned", err)
	}
}
