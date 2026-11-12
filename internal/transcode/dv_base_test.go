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

func TestDVFixtureCompatibleBaseWithFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	source := filepath.Join("..", "probe", "testdata", "dv81-other-frames.mkv")
	for _, copyVideo := range []bool{true, false} {
		t.Run(map[bool]string{true: "strip and remux HDR10", false: "tone map HDR10 base"}[copyVideo], func(t *testing.T) {
			m := New(filepath.Join(t.TempDir(), "sessions"), func() string { return ffmpeg }, func() string { return ffprobe }, slog.New(slog.NewTextHandler(io.Discard, nil)))
			s, err := m.Start(Options{Path: source, VideoStream: 0, AudioStream: -1, CopyVideo: copyVideo, VideoCodec: "hevc", BurnSubtitle: -1, StripDolbyVision: true, ToneMap: !copyVideo})
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
			output := filepath.Join(s.dir, "index.m3u8")
			data, err := exec.Command(ffprobe, "-v", "error", "-show_frames", "-show_entries", "frame_side_data=side_data_type", "-of", "json", output).CombinedOutput()
			if err != nil || strings.Contains(string(data), "Dolby Vision") {
				t.Fatalf("DV metadata not stripped: %v %s", err, data)
			}
			data, err = exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name,color_transfer", "-of", "csv=p=0", output).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(data))
			}
			if copyVideo && !strings.Contains(string(data), "hevc,smpte2084") {
				t.Fatalf("HDR10 base lost: %s", data)
			}
			if !copyVideo && (!strings.Contains(string(data), "h264,bt709") || strings.Contains(string(data), "smpte2084")) {
				t.Fatalf("not SDR: %s", data)
			}
		})
	}
}
