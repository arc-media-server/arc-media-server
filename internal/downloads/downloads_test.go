package downloads

import (
	"ams/internal/store"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) (*store.Store, *store.Download, *Manager, *bytes.Buffer) {
	return fixtureAt(t, filepath.Join(t.TempDir(), "fixture.db"))
}
func fixtureAt(t *testing.T, dbPath string) (*store.Store, *store.Download, *Manager, *bytes.Buffer) {
	t.Helper()
	ctx := t.Context()
	st, err := store.Open(ctx, dbPath, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	u, err := st.CreateUser(ctx, "Owner", true, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.CreateSession(ctx, "private-session-hash", u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	lib, err := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Test"}, "test")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "private-source.mp4")
	if err = os.WriteFile(path, []byte("generated fixture bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	f := store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}
	f.ID, err = st.SaveFile(ctx, &f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.SetFileItems(ctx, f.ID, []string{id}, 0); err != nil {
		t.Fatal(err)
	}
	d, _, err := st.CreateDownload(ctx, u.ID, "private-session-hash", id, "private-operation-key", f)
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	m := New(st, slog.New(slog.NewTextHandler(&logs, nil)))
	return st, d, m, &logs
}
func TestDownloadPreparationHashFreshnessAndPrivacy(t *testing.T) {
	st, d, m, logs := fixture(t)
	before, err := os.ReadFile(d.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Prepare(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	saved, err := st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(before)
	if saved.State != "ready" || saved.SHA256 != hex.EncodeToString(sum[:]) || saved.SourceMTimeNS == 0 {
		t.Fatalf("descriptor %+v", saved)
	}
	after, _ := os.ReadFile(d.SourcePath)
	if !bytes.Equal(before, after) {
		t.Fatal("source modified")
	}
	for _, secret := range []string{d.SourcePath, d.SessionHash, d.Key} {
		if strings.Contains(logs.String(), secret) {
			t.Fatalf("private value in log")
		}
	}
	if strings.Count(logs.String(), "level=INFO") != 2 {
		t.Fatalf("unexpected successful log volume: %s", logs)
	}
}
func TestDownloadPreparationRejectsChangedAndRemovedSources(t *testing.T) {
	for _, change := range []string{"changed", "removed"} {
		t.Run(change, func(t *testing.T) {
			st, d, m, _ := fixture(t)
			if change == "changed" {
				if err := os.WriteFile(d.SourcePath, []byte("replacement"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Remove(d.SourcePath); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.Prepare(t.Context(), d); err != nil {
				t.Fatal(err)
			}
			got, err := st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != "failed" || got.SHA256 != "" {
				t.Fatal("bad source published")
			}
		})
	}
}
func TestDownloadWorkerPauseCancelRestartAndDelete(t *testing.T) {
	st, d, m, _ := fixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	var paused atomic.Bool
	paused.Store(true)
	entered := make(chan struct{}, 1)
	m.Playback = func() bool {
		select {
		case entered <- struct{}{}:
		default:
		}
		return paused.Load()
	}
	done := make(chan error, 1)
	go func() { done <- m.Prepare(ctx, d) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not pause")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
	current, err := st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil || current.State != "preparing" || current.SHA256 != "" {
		t.Fatal("interrupted hash published")
	}
	paused.Store(false)
	if err = m.Prepare(t.Context(), current); err != nil {
		t.Fatal(err)
	}
	current, err = st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil || current.State != "ready" {
		t.Fatal("restart failed")
	}
	if err = st.DeleteDownload(t.Context(), d.UserID, d.SessionHash, d.ID); err != nil {
		t.Fatal(err)
	}
	if err = m.Prepare(t.Context(), d); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Download(t.Context(), d.UserID, d.SessionHash, d.ID); !errors.Is(err, store.ErrDownloadNotFound) {
		t.Fatal("deleted attempt resurrected")
	}
}
func TestDownloadRunJoinsRootCancellation(t *testing.T) {
	_, _, m, _ := fixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	m.Playback = func() bool { return true }
	done := make(chan struct{})
	go func() { defer close(done); m.Run(ctx) }()
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not join")
	}
}

func TestDownloadPreparationRechecksRevocationWhilePaused(t *testing.T) {
	st, d, m, _ := fixture(t)
	ctx := t.Context()
	entered := make(chan struct{}, 1)
	m.Playback = func() bool {
		select {
		case entered <- struct{}{}:
		default:
		}
		return true
	}
	done := make(chan error, 1)
	go func() { done <- m.Prepare(ctx, d) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("not paused")
	}
	if err := st.SetSessionUser(ctx, d.SessionHash, ""); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("revoked preparation did not stop")
	}
	saved, err := st.Download(ctx, d.UserID, d.SessionHash, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != "failed" || saved.SHA256 != "" {
		t.Fatal("revoked source published")
	}
}

func TestDownloadPreparationRejectsSameSizeEditsDuringHash(t *testing.T) {
	st, d, m, _ := fixture(t)
	entered := make(chan struct{}, 1)
	var pause atomic.Bool
	pause.Store(true)
	m.Playback = func() bool {
		select {
		case entered <- struct{}{}:
		default:
		}
		return pause.Load()
	}
	done := make(chan error, 1)
	go func() { done <- m.Prepare(t.Context(), d) }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("not paused")
	}
	if err := os.WriteFile(d.SourcePath, bytes.Repeat([]byte("z"), int(d.Size)), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(d.SourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(d.SourcePath, info.ModTime(), info.ModTime().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	pause.Store(false)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	saved, err := st.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.State != "failed" || saved.SHA256 != "" {
		t.Fatal("changed open file published")
	}
}

func TestDownloadPendingRecordSurvivesDatabaseReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "durable.db")
	st, d, _, logs := fixtureAt(t, dbPath)
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Open(t.Context(), dbPath, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	saved, err := restored.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
	if err != nil || saved.State != "preparing" || saved.Revision != d.Revision {
		t.Fatal("pending descriptor lost on reopen")
	}
	m := New(restored, slog.New(slog.NewTextHandler(logs, nil)))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); m.Run(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		saved, err = restored.Download(t.Context(), d.UserID, d.SessionHash, d.ID)
		if err != nil {
			t.Fatal(err)
		}
		if saved.State == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("pending work not recovered")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(saved.SHA256) != 64 {
		t.Fatal("recovered digest missing")
	}
}
