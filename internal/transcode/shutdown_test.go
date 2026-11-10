package transcode

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestRunWaitsForSessionStoppedBeforeShutdown(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix delayed-process fixture")
	}
	bin := filepath.Join(t.TempDir(), "fake-ffmpeg")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexec /bin/sleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	m := New(filepath.Join(t.TempDir(), "transcode"), func() string { return bin }, func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	session, err := m.Start(Options{Path: "isolated fixture", AudioStream: -1, BurnSubtitle: -1})
	if err != nil {
		t.Fatal(err)
	}
	// Model delayed process termination deterministically, without relying on OS timing.
	release := make(chan struct{})
	var once sync.Once
	cancel := session.cancel
	session.cancel = func() { go func() { <-release; cancel() }() }
	defer func() { once.Do(func() { close(release) }); cancel(); <-session.done }()
	m.Stop(session.ID)
	root, stop := context.WithCancel(t.Context())
	stop()
	returned := make(chan struct{})
	go func() { m.Run(root); close(returned) }()
	select {
	case <-returned:
		t.Error("Run returned while an already-stopped session still needed reaping/cleanup")
	case <-time.After(50 * time.Millisecond):
	}
	once.Do(func() { close(release) })
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish after process exit")
	}
	if _, err := os.Stat(session.dir); !os.IsNotExist(err) {
		t.Errorf("shutdown returned before stopped-session cleanup: %v", err)
	}
}

func TestStoppedManagerDoesNotAdmitNewProcesses(t *testing.T) {
	m := New(filepath.Join(t.TempDir(), "transcode"), func() string { return "not-started" }, func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	root, stop := context.WithCancel(t.Context())
	stop()
	m.Run(root)
	if session, err := m.Start(Options{Path: "fixture"}); err == nil || session != nil {
		t.Fatalf("stopped manager admitted process: %+v %v", session, err)
	}
	// Failure must be shutdown admission, not a failed attempt to execute the configured binary.
	if _, err := os.Stat(m.dir); !os.IsNotExist(err) {
		t.Error("stopped manager created a new process directory")
	}
}

func TestCloseReapsPreparedFFmpegAndReleasesOutput(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("prepared FFmpeg missing")
	}
	source := filepath.Join(t.TempDir(), "generated.mp4")
	if out, err := exec.Command(bin, "-hide_banner", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=s=160x90:d=4", "-c:v", "libx264", source).CombinedOutput(); err != nil {
		t.Fatalf("generated source: %v %s", err, out)
	}
	m := New(filepath.Join(t.TempDir(), "sessions"), func() string { return bin }, func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	session, err := m.Start(Options{Path: source, AudioStream: -1, BurnSubtitle: -1})
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
	m.Close() // Concurrent/root/deferred shutdown paths may all call Close.
	select {
	case <-session.done:
	default:
		t.Error("process not reaped")
	}
	select {
	case <-session.throttleDone:
	default:
		t.Error("throttle/output watcher not joined")
	}
	if m.Count() != 0 {
		t.Error("sessions remain after Close")
	}
	if _, err := os.Stat(session.dir); !os.IsNotExist(err) {
		t.Fatalf("output cleanup not joined: %v", err)
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.proc != nil {
		t.Fatal("process retained after Close")
	}
}
