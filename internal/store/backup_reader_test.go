package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupDoesNotWaitForWriterAndExcludesUncommittedChanges(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	if err := s.SetSetting(ctx, "backup-writer-fixture", "committed"); err != nil {
		t.Fatal(err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE settings SET value = 'uncommitted' WHERE key = 'backup-writer-fixture'`); err != nil {
		t.Fatal(err)
	}
	backupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	destination := filepath.Join(t.TempDir(), "backup.db")
	if err = s.BackupTo(backupCtx, destination); err != nil {
		t.Fatalf("backup entered the occupied writer queue: %v", err)
	}
	if err = CheckBackup(ctx, destination); err != nil {
		t.Fatal(err)
	}
	copied, err := Open(ctx, destination, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer copied.Close()
	if value, err := copied.Setting(ctx, "backup-writer-fixture"); err != nil || value != "committed" {
		t.Fatalf("backup included uncommitted state: %q %v", value, err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if value, err := s.Setting(ctx, "backup-writer-fixture"); err != nil || value != "uncommitted" {
		t.Fatal("writer failed after backup", value, err)
	}
}

func TestBackupReaderCannotModifyMainDatabase(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	if err := s.SetSetting(ctx, "fixture", "original"); err != nil {
		t.Fatal(err)
	}
	reader, err := s.backupReader(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err = reader.ExecContext(ctx, `UPDATE settings SET value = 'changed' WHERE key = 'fixture'`); err == nil {
		t.Fatal("backup source allowed a write")
	}
	if value, err := s.Setting(ctx, "fixture"); err != nil || value != "original" {
		t.Fatal(value, err)
	}
}

func TestBackupReaderSourceWithSpecialFilenameAndCancelledCopy(t *testing.T) {
	ctx := t.Context()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "source # percent %.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	path := filepath.Join(t.TempDir(), "backup.db")
	if err = s.BackupTo(ctx, path); err != nil {
		t.Fatal("special source filename", err)
	}
	if err = CheckBackup(ctx, path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.BackupTo(ctx, path); err == nil {
		t.Fatal("existing backup overwritten")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(after) != string(before) {
		t.Fatal("existing snapshot changed", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	absent := filepath.Join(t.TempDir(), "cancelled.db")
	if err = s.BackupTo(cancelled, absent); err != context.Canceled {
		t.Fatal("cancelled backup", err)
	}
	if _, err = os.Stat(absent); !os.IsNotExist(err) {
		t.Fatal("cancelled backup created a destination", err)
	}
}
