package transcode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRollingHLSPrunesOnlyConsumedHistory(t *testing.T) {
	for _, rolling := range []bool{false, true} {
		t.Run(fmt.Sprint(rolling), func(t *testing.T) {
			var opts Options
			json.Unmarshal([]byte(fmt.Sprintf(`{"RollingHLS":%v}`, rolling)), &opts)
			dir := t.TempDir()
			var playlist strings.Builder
			playlist.WriteString("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-TARGETDURATION:6\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-MAP:URI=\"init.mp4\"\n")
			os.WriteFile(filepath.Join(dir, "init.mp4"), nil, 0600)
			for n := 0; n < 80; n++ {
				name := fmt.Sprintf("s%05d.m4s", n)
				os.WriteFile(filepath.Join(dir, name), nil, 0600)
				fmt.Fprintf(&playlist, "#EXTINF:6.000,\n%s\n", name)
			}
			os.WriteFile(filepath.Join(dir, "index.m3u8"), []byte(playlist.String()), 0600)
			s := &Session{Options: opts, dir: dir, done: make(chan struct{}), throttleWake: make(chan struct{}, 1), lastSeen: time.Now()}
			s.Requested("s99999.m4s")
			if _, err := os.Stat(filepath.Join(dir, "s00000.m4s")); err != nil {
				t.Fatal("nonexistent future request deleted history")
			}
			s.Requested("s00050.m4s")
			_, err := os.Stat(filepath.Join(dir, "s00000.m4s"))
			if rolling && !os.IsNotExist(err) {
				t.Fatal("old consumed segment retained")
			}
			if !rolling && err != nil {
				t.Fatal("legacy cache behavior changed")
			}
			for _, name := range []string{"init.mp4", "s00030.m4s", "s00050.m4s", "s00079.m4s"} {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Fatal("current/forward segment removed", name)
				}
			}
			got, err := s.Playlist(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if rolling && (strings.Contains(string(got), "s00000.m4s") || strings.Contains(string(got), "PLAYLIST-TYPE:EVENT") || !strings.Contains(string(got), "#EXT-X-MEDIA-SEQUENCE:30")) {
				t.Fatalf("stale moving playlist: %s", got)
			}
		})
	}
}

func TestConversionFreeSpaceAdmissionAndStop(t *testing.T) {
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := New(filepath.Join(t.TempDir(), "sessions"), func() string { return bin }, func() string { return "" }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.freeSpace = func(string) (uint64, error) { return conversionSpaceReserve - 1, nil }
	if _, err := m.Start(Options{Path: "unused", AudioStream: -1, BurnSubtitle: -1}); !errors.Is(err, ErrStorageFull) {
		t.Fatalf("low storage admitted: %v", err)
	}
	if m.Count() != 0 {
		t.Fatal("rejected start retained capacity")
	}
	stopped := false
	s := &Session{ID: "fixture", dir: filepath.Join(m.dir, "fixture"), done: make(chan struct{}), cancel: func() { stopped = true }}
	m.sessions[s.ID] = s
	m.checkStorage()
	if !stopped || !errors.Is(s.Err(), ErrStorageFull) {
		t.Fatal("active conversion did not stop with diagnosable storage failure")
	}
	// Subsequent passes must not repeat the outcome or cancel again.
	stopped = false
	m.checkStorage()
	if stopped {
		t.Fatal("storage failure repeatedly canceled session")
	}
}

func TestRollingHLSRealConvertedSegments(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()
	source := filepath.Join(dir, "source.mp4")
	data, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=blue:size=160x90:rate=2:duration=180", "-c:v", "libx264", "-preset", "ultrafast", "-g", "6", source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(data))
	}
	m := New(filepath.Join(dir, "sessions"), func() string { return ffmpeg }, func() string { return ffprobe }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.throttling = throttling{pauseAhead: 24 * time.Second, resumeAhead: 18 * time.Second, tick: time.Second}
	s, err := m.Start(Options{Path: source, AudioStream: -1, BurnSubtitle: -1, RollingHLS: true})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop(s.ID)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if err := m.Ready(ctx, s); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for s.segments() < 28 && time.Now().Before(deadline) {
		if err := s.Err(); err != nil {
			t.Fatal(err)
		}
		made := s.segments()
		if made > 0 {
			s.Requested(fmt.Sprintf("s%05d.m4s", made-1))
		}
		time.Sleep(20 * time.Millisecond)
	}
	made := s.segments()
	if made < 28 {
		t.Fatalf("conversion did not resume across window: %d segments", made)
	}
	s.Requested("s00026.m4s")
	if _, ok := s.File("s00000.m4s"); ok {
		t.Fatal("real consumed history retained")
	}
	if _, ok := s.File("s00026.m4s"); !ok {
		t.Fatal("current real segment removed")
	}
	playlist, err := s.Playlist(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(playlist), "s00000.m4s") || strings.Contains(string(playlist), "PLAYLIST-TYPE:EVENT") {
		t.Fatalf("old playlist served: %s", playlist)
	}
	// The on-disk producer playlist retains old entries; probe the served sliding snapshot instead.
	served := filepath.Join(s.dir, "served.m3u8")
	if err := os.WriteFile(served, playlist, 0600); err != nil {
		t.Fatal(err)
	}
	data, err = exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", served).Output()
	if err != nil || !strings.Contains(string(data), "h264") {
		t.Fatalf("retained window unplayable: %v %s", err, data)
	}
}
