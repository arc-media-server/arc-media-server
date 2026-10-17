package transcode

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildArgs(t *testing.T) {
	copyArgs := strings.Join(buildArgs(Options{Path: "in.mkv", VideoStream: 0, AudioStream: 2, CopyVideo: true, VideoCodec: "hevc", BurnSubtitle: -1}, 20, "out"), " ")
	for _, want := range []string{"-ss 20.000 -noaccurate_seek -copyts -i in.mkv", "-map 0:0 -map 0:2", "-c:v copy -avoid_negative_ts disabled -tag:v hvc1", "-c:a aac -ac 2", "-hls_segment_type fmp4"} {
		if !strings.Contains(copyArgs, want) {
			t.Errorf("copy args missing %q:\n%s", want, copyArgs)
		}
	}
	enc := strings.Join(buildArgs(Options{Path: "in.mkv", AudioStream: -1, MaxHeight: 720, SourceHeight: 2160, VideoKbps: 4000, BurnSubtitle: -1}, 0, "out"), " ")
	for _, want := range []string{"-c:v libx264", "-b:v 4000k", "scale=-2:720"} {
		if !strings.Contains(enc, want) {
			t.Errorf("encode args missing %q:\n%s", want, enc)
		}
	}
	burn := strings.Join(buildArgs(Options{Path: "in.mkv", VideoStream: 0, AudioStream: 1, BurnSubtitle: 3, MaxHeight: 720, SourceHeight: 1080}, 0, "out"), " ")
	if !strings.Contains(burn, "-filter_complex [0:0][0:3]overlay=eof_action=pass,scale=-2:720[v] -map [v] -map 0:1") || strings.Contains(burn, "-vf") {
		t.Errorf("burn-in args:\n%s", burn)
	}
	for _, unwanted := range []string{"-ss", "-c:a", "-copyts"} {
		if strings.Contains(enc, unwanted) {
			t.Errorf("unexpected %q:\n%s", unwanted, enc)
		}
	}
}

// TestSession runs FFmpeg for real when it's installed: remuxing from part-way through starts on
// a keyframe, and Ready measures where.
func TestSession(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()
	m := New(filepath.Join(dir, "sessions"), func() string { return ffmpeg }, func() string { return ffprobe },
		slog.New(slog.NewTextHandler(io.Discard, nil)))

	for _, tc := range []struct {
		name, codec string
		encode      []string
		wantOffset  float64 // the keyframe before 9s
	}{
		// A keyframe every 2 seconds.
		{"h264", "h264", []string{"-c:v", "libx264", "-g", "50", "-keyint_min", "50"}, 8},
		// x265's open GOPs: frames after a keyframe can be shown before it.
		{"hevc", "hevc", []string{"-c:v", "libx265", "-x265-params", "log-level=error:keyint=100:min-keyint=100"}, 8},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := filepath.Join(dir, tc.name+".mkv")
			args := append([]string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=duration=20:size=320x180:rate=25",
				"-f", "lavfi", "-i", "sine=duration=20"}, tc.encode...)
			if out, err := exec.Command(ffmpeg, append(args, "-c:a", "ac3", src)...).CombinedOutput(); err != nil {
				t.Fatalf("making source: %v %s", err, out)
			}
			s, err := m.Start(Options{Path: src, StartMS: 9000, VideoStream: 0, AudioStream: 1, CopyVideo: true, VideoCodec: tc.codec, BurnSubtitle: -1})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := m.Ready(ctx, s); err != nil {
				t.Fatal(err)
			}
			// Within a frame or two (B-frames start a little before the keyframe).
			if got := float64(s.OffsetMS) / 1000; got < tc.wantOffset-0.2 || got > tc.wantOffset+0.05 {
				t.Errorf("offset = %.3fs, want about %vs", got, tc.wantOffset)
			}
			<-s.done
			if _, ok := s.File("init.mp4"); !ok {
				t.Error("no init segment")
			}
			if _, ok := s.File("../secret"); ok {
				t.Error("served a path outside the session")
			}
			// The output has video, not just audio.
			out, _ := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0",
				filepath.Join(s.dir, "index.m3u8")).Output()
			if !strings.Contains(string(out), tc.codec) {
				t.Errorf("output streams: %s", out)
			}
			m.Stop(s.ID)
			deadline := time.Now().Add(3 * time.Second)
			for {
				if _, err := os.Stat(s.dir); os.IsNotExist(err) {
					break
				}
				if time.Now().After(deadline) {
					t.Error("session folder left behind")
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		})
	}
}
