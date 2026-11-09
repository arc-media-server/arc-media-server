package transcode

import (
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRemuxOffsetRelativeToSourceTimeline(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "nonzero.ts")
	out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=duration=16:size=160x90:rate=25", "-f", "lavfi", "-i", "sine=duration=16", "-c:v", "libx264", "-g", "50", "-keyint_min", "50", "-sc_threshold", "0", "-c:a", "ac3", "-output_ts_offset", "1000", "-f", "mpegts", src).CombinedOutput()
	if err != nil {
		t.Fatalf("generate shifted TS: %v %s", err, out)
	}
	for _, start := range []int64{0, 9000} {
		t.Run(time.Duration(start*time.Millisecond.Nanoseconds()).String(), func(t *testing.T) {
			m := New(filepath.Join(t.TempDir(), "sessions"), func() string { return ffmpeg }, func() string { return ffprobe }, slog.New(slog.NewTextHandler(io.Discard, nil)))
			s, err := m.Start(Options{Path: src, StartMS: start, VideoStream: 0, AudioStream: 1, CopyVideo: true, BurnSubtitle: -1})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Stop(s.ID)
			if err := m.Ready(t.Context(), s); err != nil {
				t.Fatal(err)
			}
			want := int64(0)
			if start > 0 {
				want = 8000
			}
			if s.OffsetMS < want-200 || s.OffsetMS > want+1200 {
				t.Errorf("offset_ms=%d, expected source-relative about %d", s.OffsetMS, want)
			}
		})
	}
}
