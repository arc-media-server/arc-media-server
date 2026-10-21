package transcode

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestConversionStorageRecovery(t *testing.T) {
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "sessions")
	m := New(dir, func() string { return bin }, func() string { return "" }, slog.New(slog.DiscardHandler))
	m.Configure(dir, 1)
	if err := os.WriteFile(dir, []byte("blocked storage"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(Options{Path: "unused", AudioStream: -1, BurnSubtitle: -1}); err == nil {
		t.Fatal("blocked storage accepted")
	}
	if m.Count() != 0 {
		t.Fatal("storage failure retained capacity")
	}
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	s, err := m.Start(Options{Path: "unused", AudioStream: -1, BurnSubtitle: -1})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop(s.ID)
	select {
	case <-s.done:
	case <-time.After(10 * time.Second):
		t.Fatal("stand-in process did not exit")
	}
}

func TestShutdownStopsConversionAndRestartClearsOutput(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mp4")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=duration=2:size=160x90:rate=10", "-c:v", "libx264", src).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	sessions := filepath.Join(dir, "sessions")
	m := New(sessions, func() string { return ffmpeg }, func() string { return "" }, slog.New(slog.DiscardHandler))
	s, err := m.Start(Options{Path: src, VideoStream: 0, AudioStream: -1, BurnSubtitle: -1})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop(s.ID)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("manager shutdown hung")
	}
	select {
	case <-s.done:
	case <-time.After(10 * time.Second):
		t.Fatal("FFmpeg survived shutdown")
	}
	if m.Count() != 0 || m.Get(s.ID) != nil {
		t.Fatal("shutdown retained session")
	}
	// Simulate output left by an abrupt exit, scoped strictly to the synthetic session folder.
	if err := os.MkdirAll(filepath.Join(sessions, "orphan"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessions, "orphan", "segment.m4s"), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	restarted := New(sessions, func() string { return ffmpeg }, func() string { return "" }, slog.New(slog.DiscardHandler))
	if _, err := os.Stat(sessions); !os.IsNotExist(err) {
		t.Fatalf("stale output survived restart: %v", err)
	}
	next, err := restarted.Start(Options{Path: src, VideoStream: 0, AudioStream: -1, BurnSubtitle: -1})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Stop(next.ID)
	select {
	case <-next.done:
	case <-time.After(10 * time.Second):
		t.Fatal("conversion after restart hung")
	}
	if err := next.Err(); err != nil {
		t.Fatal(err)
	}
}
