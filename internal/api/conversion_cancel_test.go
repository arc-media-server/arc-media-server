package api

import (
	"ams/internal/logbuf"
	"ams/internal/settings"
	"ams/internal/store"
	"ams/internal/transcode"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestCancelledHardwareReadinessDoesNotStartCPUFallback(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg unavailable")
	}
	version, err := exec.Command(ffmpeg, "-version").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(strings.SplitN(string(version), "\n", 2)[0])
	s, u := securityServer(t)
	root := t.TempDir()
	source := filepath.Join(root, "fixture.mp4")
	cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=12", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "2", "-c:v", "libx264", "-threads", "1", "-c:a", "aac", "-y", source)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated fixture: %v %s", err, out)
	}
	lib, err := s.store.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "cancel-fixture")
	if err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	file := store.MediaFile{LibraryID: lib, Path: source, Size: stat.Size(), ModTime: stat.ModTime().Unix(), Container: "mp4", DurationMS: 2000, Streams: []store.Stream{{Index: 0, Kind: "video", Codec: "h264", Width: 160, Height: 90}, {Index: 1, Kind: "audio", Codec: "aac", Channels: 2, IsDefault: true}}}
	file.ID, err = s.store.SaveFile(t.Context(), &file, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetFileItems(t.Context(), file.ID, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	// Simulate a previously successful hardware probe, not real GPU availability. Runtime starts
	// below use the actual selected FFmpeg binary. The disconnect happens at first admission.
	probeStub := filepath.Join(root, "probe-stub")
	if err = os.WriteFile(probeStub, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var runtimePhase atomic.Bool
	var starts atomic.Int32
	logs := logbuf.New(64)
	s.log = slog.New(logs.Wrap(slog.NewTextHandler(io.Discard, nil)))
	s.transcoder = transcode.New(filepath.Join(root, "transcode"), func() string {
		if !runtimePhase.Load() {
			return probeStub
		}
		starts.Add(1)
		cancel()
		return ffmpeg
	}, func() string { return "" }, s.log)
	caps := s.transcoder.Detect(t.Context())
	if caps.HWEncoder != "h264_nvenc" {
		t.Fatal("hardware-control probe failed", caps)
	}
	runtimePhase.Store(true)
	s.settings = func() settings.Settings { return settings.Settings{HWAccel: true} }
	r := httptest.NewRequest("POST", "/api/items/"+item+"/playback", strings.NewReader(fmt.Sprintf(`{"file_id":%d,"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"],"max_height":60}}`, file.ID))).WithContext(context.WithValue(ctx, userKey, u))
	r.SetPathValue("id", item)
	w := httptest.NewRecorder()
	s.startPlayback(w, r)
	if got := starts.Load(); got != 1 {
		t.Errorf("disconnect started %d conversions; wanted only the first admission, no CPU retry", got)
	}
	for _, line := range logs.Lines(slog.LevelWarn, 0) {
		if strings.Contains(line.Message, "hardware conversion failed; using the CPU") {
			t.Error("ordinary disconnect logged as hardware failure")
		}
	}
	if strings.Contains(w.Body.String(), "context canceled") {
		t.Error("ordinary disconnect written as server failure")
	}
	if s.transcoder.Count() != 0 {
		t.Fatal("cancelled conversion retained a session")
	}
}

func TestAlreadyCancelledConversionDoesNotStartFFmpeg(t *testing.T) {
	s, _ := securityServer(t)
	var starts atomic.Int32
	s.transcoder = transcode.New(filepath.Join(t.TempDir(), "transcode"), func() string { starts.Add(1); return "ffmpeg" }, func() string { return "" }, s.log)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := httptest.NewRequest("POST", "/", nil).WithContext(ctx)
	_, err := s.startConversion(r, transcode.Options{Path: "not-opened", BurnSubtitle: -1})
	if err != context.Canceled {
		t.Errorf("error=%v", err)
	}
	if starts.Load() != 0 {
		t.Error("already disconnected request started FFmpeg")
	}
	if s.transcoder.Count() != 0 {
		t.Fatal("cancelled conversion retained a session")
	}
}
