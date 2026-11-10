package transcode

import (
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUltrawideConversionFitsQualityBox(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	src := filepath.Join(t.TempDir(), "wide.mkv")
	out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=red:size=3840x1600:rate=2:duration=1", "-c:v", "libx264", "-preset", "ultrafast", src).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	m := New(filepath.Join(t.TempDir(), "sessions"), func() string { return ffmpeg }, func() string { return ffprobe }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s, err := m.Start(Options{Path: src, VideoStream: 0, AudioStream: -1, BurnSubtitle: -1, MaxHeight: 1080, SourceHeight: 1600})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop(s.ID)
	select {
	case <-s.done:
	case <-time.After(30 * time.Second):
		t.Fatal("conversion stalled")
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	out, err = exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0:s=x", filepath.Join(s.dir, "index.m3u8")).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "1920x800") {
		t.Fatalf("not fitted to 1080 quality box: %v %s", err, out)
	}
}

func TestQualityScaleNeverUpscalesAndLimitsWidthBelowHeight(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	for _, tc := range []struct{ source, want string }{{"160x90", "160x90"}, {"3200x800", "1920x480"}, {"720x1280", "608x1080"}} {
		t.Run(tc.source, func(t *testing.T) {
			outPath := filepath.Join(t.TempDir(), "scaled.mp4")
			out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=size="+tc.source+":rate=1:duration=1", "-vf", FitScaleFilter(1080), "-c:v", "libx264", "-preset", "ultrafast", outPath).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(out))
			}
			out, err = exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0:s=x", outPath).CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != tc.want {
				t.Fatalf("scale %s: %v %s", tc.source, err, out)
			}
		})
	}
}
