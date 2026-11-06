package transcode

import (
	"context"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// A real FFmpeg conversion pauses once it's far enough ahead of what the player asked for, makes
// no more while paused, resumes when the player catches up, and still stops cleanly while paused.
func TestConversionWaitsForThePlayer(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	clip := filepath.Join(dir, "clip.mp4")
	// Three minutes (30 segments); it converts in a few seconds unthrottled.
	if out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc=duration=180:size=160x90:rate=10",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "30", clip).CombinedOutput(); err != nil {
		t.Fatalf("making clip: %v %s", err, out)
	}
	m := New(filepath.Join(dir, "sessions"), func() string { return ffmpeg }, func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	// Thresholds scaled down for a 3-minute clip. A one-hour timer ensures only real
	// playlist notifications/player wakeups can satisfy this regression.
	m.throttling = throttling{pauseAhead: 24 * time.Second, resumeAhead: 18 * time.Second, tick: time.Hour}
	s, err := m.Start(Options{Path: clip, VideoStream: 0, AudioStream: -1, BurnSubtitle: -1, SourceHeight: 90})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := s.Playlist(ctx); err != nil {
		t.Fatal(err)
	}
	s.Requested("s00000.m4s")
	waitFor(t, "pause", s.Throttled)
	time.Sleep(500 * time.Millisecond)
	paused := s.segments()
	// Paused about 24s (4 segments) ahead, with at most a segment or two in flight.
	if paused < 4 || paused > 8 {
		t.Fatalf("paused with %d segments made, want about 5", paused)
	}
	time.Sleep(time.Second)
	if n := s.segments(); n != paused {
		t.Fatalf("made %d segments while paused (was %d)", n, paused)
	}
	select {
	case <-s.done:
		t.Fatal("FFmpeg finished the whole clip despite the player being at the start")
	default:
	}

	// The player plays one more segment: conversion resumes at once and pauses again a segment
	// on, so the playlist keeps growing while someone watches (a still one looks stuck to players).
	s.Requested("s00001.m4s")
	waitFor(t, "resume after one segment", func() bool { return s.segments() > paused })
	waitFor(t, "pause again", s.Throttled)
	if n := s.segments(); n > paused+3 {
		t.Fatalf("made %d segments after the player moved one on (was %d), want about one", n, paused)
	}

	// Asking for a segment that isn't made yet resumes at once.
	s.Requested("s00029.m4s")
	if s.Throttled() {
		t.Fatal("still paused after the player asked for a segment not made yet")
	}
	s.Requested("s00001.m4s") // seeks back: pauses again once far enough ahead
	waitFor(t, "pause after seeking back", s.Throttled)

	// Stopping a paused conversion ends FFmpeg.
	m.Stop(s.ID)
	select {
	case <-s.done:
		if s.Throttled() {
			t.Fatal("finished conversion still reported throttled")
		}
		select {
		case <-s.throttleDone:
		case <-time.After(time.Second):
			t.Fatal("pacing watcher did not stop")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a paused conversion didn't stop")
	}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting to %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// While someone watches, a throttled playlist must grow at least every few segment lengths:
// Media3 fails a live playlist that doesn't change for 3.5 target durations.
func TestThrottlingResumesWithinOneSegment(t *testing.T) {
	if gap := defaultThrottling.pauseAhead - defaultThrottling.resumeAhead; gap <= 0 || gap > segmentSeconds*time.Second {
		t.Fatalf("pause/resume gap %v: the playlist would stand still for longer than a segment", gap)
	}
}
