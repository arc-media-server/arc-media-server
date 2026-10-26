package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ams/internal/activity"
	"ams/internal/store"
)

func trashFixture(t *testing.T) (*store.Store, *Scanner, int64, string) {
	t.Helper()
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Gone (2020).mkv": "gone", "Here (2021).mkv": "here"})
	s.Activity = activity.New()
	if err := s.scan(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "Gone (2020).mkv")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.WriteFile(path, []byte("gone"), 0644); err != nil {
			t.Error(err)
		}
	})
	if err := s.scan(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	return st, s, lib, root
}

func TestTrashAutomaticSafety(t *testing.T) {
	for _, scenario := range []string{"success", "backup failure", "root disappears during backup", "restored file", "partial scan", "cancelled scan", "restart", "roots edited", "closed window", "no backup", "empty mount point", "window closes during backup", "second root offline", "file returns during backup"} {
		t.Run(scenario, func(t *testing.T) {
			st, s, lib, root := trashFixture(t)
			ctx := t.Context()
			backups := 0
			before := func(context.Context) error { backups++; return nil }
			allowed := func() bool { return true }
			wantBackup := 0
			switch scenario {
			case "window closes during backup":
				wantBackup = 1
				before = func(context.Context) error { backups++; allowed = func() bool { return false }; return nil }
			case "file returns during backup":
				wantBackup = 1
				before = func(context.Context) error {
					backups++
					return os.WriteFile(filepath.Join(root, "Gone (2020).mkv"), []byte("gone"), 0644)
				}
			case "second root offline":
				second := filepath.Join(t.TempDir(), "offline")
				os.Mkdir(second, 0755)
				if _, err := st.SyncLibrary(ctx, "Fixture", "movies", "tmdb_aired", []string{root, second}); err != nil {
					t.Fatal(err)
				}
				if err := s.scan(ctx, lib); err != nil {
					t.Fatal(err)
				}
				os.Remove(second)
			case "success":
				wantBackup = 1
			case "backup failure":
				wantBackup = 1
				before = func(context.Context) error { backups++; return errors.New("disk full") }
			case "root disappears during backup":
				wantBackup = 1
				before = func(context.Context) error { backups++; return os.Rename(root, root+"-offline") }
				t.Cleanup(func() {
					if err := os.Rename(root+"-offline", root); err != nil {
						t.Error(err)
					}
				})
			case "restored file":
				if err := os.WriteFile(filepath.Join(root, "Gone (2020).mkv"), []byte("gone"), 0644); err != nil {
					t.Fatal(err)
				}
			case "partial scan":
				if err := s.scanPaths(ctx, lib, []string{root}); err != nil {
					t.Fatal(err)
				}
			case "cancelled scan":
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				_ = s.scan(cancelled, lib)
			case "restart":
				s.trashReady = nil
			case "roots edited":
				if _, err := st.SyncLibrary(ctx, "Fixture", "movies", "tmdb_aired", []string{root, t.TempDir()}); err != nil {
					t.Fatal(err)
				}
			case "closed window":
				allowed = func() bool { return false }
			case "no backup":
				before = nil
			case "empty mount point":
				if err := os.Rename(root, root+"-offline"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(root, 0755); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					os.Remove(root)
					if err := os.Rename(root+"-offline", root); err != nil {
						t.Error(err)
					}
				})
			}
			n, err := s.EmptyTrash(ctx, lib, true, before, func() bool { return allowed() })
			if scenario == "success" {
				if err != nil || n != 1 {
					t.Fatalf("removed=%d err=%v", n, err)
				}
				missing, _ := st.MissingFiles(ctx, lib)
				if len(missing) != 0 {
					t.Fatal("missing records retained")
				}
				items, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib})
				if err != nil || len(items) != 1 || items[0].Title != "Here" {
					t.Fatalf("present title lost: %v %v", items, err)
				}
				_, recent := s.Activity.Snapshot()
				if len(recent) < 1 || recent[0].Kind != "empty_trash" || recent[0].Error != "" {
					t.Fatalf("activity=%+v", recent)
				}
				if len(s.TrashCandidates()) != 0 {
					t.Fatal("successful proof replayable")
				}
			} else {
				if err == nil || n != 0 {
					t.Fatalf("unsafe cleanup removed=%d err=%v", n, err)
				}
				missing, _ := st.MissingFiles(ctx, lib)
				if len(missing) != 1 {
					t.Fatalf("unsafe cleanup lost records: %v", missing)
				}
			}
			if backups != wantBackup {
				t.Fatalf("backups=%d want=%d", backups, wantBackup)
			}
		})
	}
}

func TestTrashLeaseAndCancellation(t *testing.T) {
	st, s, lib, _ := trashFixture(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		_, err := s.EmptyTrash(ctx, lib, false, func(ctx context.Context) error { close(entered); <-release; return ctx.Err() }, nil)
		done <- err
	}()
	<-entered
	if s.Start(lib) {
		t.Fatal("scan admitted during backup")
	}
	if _, err := s.EmptyTrash(t.Context(), lib, false, nil, nil); !errors.Is(err, ErrLibraryBusy) {
		t.Fatalf("overlapping cleanup: %v", err)
	}
	s.Request(lib)
	if !s.Stop(lib, s.Status(lib).JobID) {
		t.Fatal("cleanup Stop refused")
	}
	cancel()
	close(release)
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	missing, _ := st.MissingFiles(t.Context(), lib)
	if len(missing) != 1 {
		t.Fatal("cancelled cleanup deleted records")
	}
	if status := s.Status(lib); status.Running || !status.Stopped || status.Stopping {
		t.Fatalf("stopped cleanup did not release/clear queued discovery: %+v", status)
	}
	if len(s.TrashCandidates()) != 0 {
		t.Fatal("stopped cleanup remained automatically eligible")
	}
}
