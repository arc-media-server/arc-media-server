package subtitles

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ams/internal/store"
)

func TestInternalTextTracksShareOneMediaPass(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("prepared FFmpeg missing")
	}
	dir := t.TempDir()
	for _, name := range []string{"one", "two"} {
		if err := os.WriteFile(filepath.Join(dir, name+".srt"), []byte("1\n00:00:00,100 --> 00:00:00,900\n"+name+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(dir, "multi.mkv")
	out, err := exec.Command(bin, "-hide_banner", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=s=16x16:d=1", "-i", filepath.Join(dir, "one.srt"), "-i", filepath.Join(dir, "two.srt"), "-map", "0:v", "-map", "1:s", "-map", "2:s", "-c:v", "libx264", "-c:s", "srt", source).CombinedOutput()
	if err != nil {
		t.Fatalf("making local source: %v %s", err, out)
	}
	f := &store.MediaFile{ID: 1, Path: source, Streams: []store.Stream{{Index: 0, Kind: "video", Codec: "h264"}, {Index: 1, Kind: "subtitle", Codec: "subrip"}, {Index: 2, Kind: "subtitle", Codec: "subrip"}}}
	var calls atomic.Int32
	c := NewCache(filepath.Join(dir, "cache"), func() string { calls.Add(1); return bin })
	for i, tr := range Tracks(f) {
		cues, err := c.Cues(t.Context(), f, tr)
		if err != nil || len(cues) != 1 || !strings.Contains(cues[0].Text, []string{"one", "two"}[i]) {
			t.Fatalf("track=%s cues=%+v err=%v", tr.ID, cues, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("reread same media for different text tracks: %d FFmpeg commands", calls.Load())
	}
}

func TestFailedExtractionIsNotRepeatedOnEveryRequest(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("prepared FFmpeg missing")
	}
	source := filepath.Join(t.TempDir(), "bad.mkv")
	if err := os.WriteFile(source, []byte("invalid fixture, not media"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	c := NewCache(t.TempDir(), func() string { calls.Add(1); return bin })
	f := &store.MediaFile{ID: 1, Path: source}
	track := Track{ID: "s1", StreamIndex: 1, Codec: "subrip"}
	for range 2 {
		if _, err := c.Cues(context.Background(), f, track); err == nil {
			t.Fatal("expected corrupt-source failure")
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("same failed revision retried immediately: %d", calls.Load())
	}
}

func TestExtractionBackoffExpiresAndLogsOnceWithoutPaths(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "private-api_key=not-a-key.mkv")
	if err := os.WriteFile(source, []byte("bad fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	var calls atomic.Int32
	c := NewCacheWithContext(t.Context(), t.TempDir(), func() string { calls.Add(1); return bin }, slog.New(slog.NewTextHandler(&logs, nil)))
	defer c.Close()
	now := time.Now()
	c.now = func() time.Time { return now }
	f := &store.MediaFile{ID: 9, Path: source}
	track := Track{ID: "s1", StreamIndex: 1, Codec: "subrip"}
	for range 2 {
		if _, err := c.Cues(t.Context(), f, track); err == nil || strings.Contains(err.Error(), source) {
			t.Fatalf("unsafe/missing failure: %v", err)
		}
	}
	if calls.Load() != 1 || strings.Count(logs.String(), "subtitle extraction failed") != 1 || strings.Contains(logs.String(), "api_key") || strings.Contains(logs.String(), filepath.Dir(source)) {
		t.Fatalf("unbounded/unsafe diagnostics: calls=%d %s", calls.Load(), logs.String())
	}
	now = now.Add(subtitleFailureDelay + time.Second)
	if _, err := c.Cues(t.Context(), f, track); err == nil {
		t.Fatal("expected failure")
	}
	if calls.Load() != 2 {
		t.Fatal("expired hint did not permit retry")
	}
	if err := os.WriteFile(source, []byte("different bad fixture revision"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Cues(t.Context(), f, track); err == nil {
		t.Fatal("expected failure")
	}
	if calls.Load() != 3 {
		t.Fatal("changed revision retained old failure")
	}
}

func TestExtractionSurvivesWaiterCancellationButCloseReapsIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process fixture; actual extraction remains platform-neutral")
	}
	bin := filepath.Join(t.TempDir(), "fake-ffmpeg")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexec /bin/sleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "fixture.mkv")
	if err := os.WriteFile(source, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	root, cancelRoot := context.WithCancel(t.Context())
	defer cancelRoot()
	started := make(chan struct{}, 1)
	var calls atomic.Int32
	var logs bytes.Buffer
	c := NewCacheWithContext(root, t.TempDir(), func() string { calls.Add(1); started <- struct{}{}; return bin }, slog.New(slog.NewTextHandler(&logs, nil)))
	defer c.Close()
	waiter, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	f := &store.MediaFile{ID: 10, Path: source}
	tr := Track{ID: "s1", StreamIndex: 1, Codec: "subrip"}
	go func() { _, err := c.Cues(waiter, f, tr); done <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("extraction did not start")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("waiter: %v", err)
	}
	another, cancelAnother := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancelAnother()
	if _, err := c.Cues(another, f, tr); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second wait: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatal("wait cancellation restarted the media pass")
	}
	closed := make(chan struct{})
	go func() { c.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not reap extraction")
	}
	if _, err := c.Cues(t.Context(), f, tr); !errors.Is(err, context.Canceled) {
		t.Fatalf("closed cache admitted work: %v", err)
	}
	if strings.Contains(logs.String(), "WARN") {
		t.Fatalf("expected shutdown was logged as failure: %s", logs.String())
	}
}

func TestExtractionAdmissionAndFailureHintsRemainBounded(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process fixture")
	}
	bin := filepath.Join(t.TempDir(), "fake-ffmpeg")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexec /bin/sleep 60\n"), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "fixture.mkv")
	if err := os.WriteFile(source, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	c := NewCacheWithContext(t.Context(), t.TempDir(), func() string { return bin }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer c.Close()
	done := make(chan error, maxSubtitleJobs)
	tr := Track{ID: "s1", StreamIndex: 1, Codec: "subrip"}
	for i := range maxSubtitleJobs {
		go func() {
			_, err := c.Cues(t.Context(), &store.MediaFile{ID: int64(i + 1), Path: source}, tr)
			done <- err
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		c.mu.Lock()
		n := len(c.running)
		c.mu.Unlock()
		if n == maxSubtitleJobs {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("jobs were not admitted")
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := c.Cues(t.Context(), &store.MediaFile{ID: 100, Path: source}, tr); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("unbounded admission: %v", err)
	}
	c.Close()
	for range maxSubtitleJobs {
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown waiter: %v", err)
		}
	}

	hints := NewCacheWithContext(t.Context(), t.TempDir(), func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer hints.Close()
	for i := range maxSubtitleFailures + 10 {
		if _, err := hints.Cues(t.Context(), &store.MediaFile{ID: int64(i + 1), Path: source}, tr); err == nil {
			t.Fatal("missing binary succeeded")
		}
	}
	hints.mu.Lock()
	defer hints.mu.Unlock()
	if len(hints.failed) > maxSubtitleFailures {
		t.Fatalf("unbounded failure hints: %d", len(hints.failed))
	}
}
