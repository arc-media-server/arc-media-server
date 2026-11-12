package transcode

import (
	"context"
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

func TestResizePrecedesToneMapAndSubtitleOverlay(t *testing.T) {
	for _, fast := range []bool{false, true} {
		for _, burn := range []int{-1, 1} {
			for _, encoder := range []string{"", "h264_vaapi"} {
				args := strings.Join(buildArgs(Options{Path: "fixture.mkv", VideoStream: 0, AudioStream: -1, BurnSubtitle: burn, SourceHeight: 2160, MaxHeight: 1080, ToneMap: true, FastToneMap: fast, HWEncoder: encoder}, 0, "out"), " ")
				mapping := "zscale="
				if fast {
					mapping = "tonemapx="
				}
				scale, tone := strings.Index(args, "scale="), strings.Index(args, mapping)
				if scale < 0 || tone < scale {
					t.Errorf("resize must precede tone mapping (fast=%v burn=%d): %s", fast, burn, args)
				}
				if burn >= 0 {
					overlay := strings.Index(args, "overlay=")
					if overlay < tone || !strings.Contains(args, "scale2ref=") {
						t.Errorf("caption canvas must follow resized/tone-mapped video: %s", args)
					}
				}
				if encoder != "" && strings.Index(args, "hwupload") < tone {
					t.Errorf("upload precedes CPU filters: %s", args)
				}
			}
		}
	}
}

func TestResizedHDRToneMapWithGeneratedPictureCaption(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not installed")
	}
	version, err := exec.Command(ffmpeg, "-version").Output()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(strings.SplitN(string(version), "\n", 2)[0])
	dir := t.TempDir()
	pgs := filepath.Join(dir, "caption.sup")
	if err = os.WriteFile(pgs, fixturePGS(), 0600); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "hdr.mkv")
	output, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=black:size=640x360:rate=4:duration=2", "-i", pgs, "-map", "0:v:0", "-map", "1:s:0", "-c:v", "libx265", "-preset", "ultrafast", "-pix_fmt", "yuv420p10le", "-x265-params", "log-level=error:pools=1", "-bsf:v", "hevc_metadata=colour_primaries=9:transfer_characteristics=16:matrix_coefficients=9", "-c:s", "copy", source).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(output))
	}
	for _, fast := range []bool{false, true} {
		for _, burn := range []int{-1, 1} {
			t.Run(fmt.Sprintf("fast=%v/burn=%d", fast, burn), func(t *testing.T) {
				m := New(filepath.Join(t.TempDir(), "sessions"), func() string { return ffmpeg }, func() string { return ffprobe }, slog.New(slog.NewTextHandler(io.Discard, nil)))
				s, err := m.Start(Options{Path: source, VideoStream: 0, AudioStream: -1, BurnSubtitle: burn, SourceHeight: 360, MaxHeight: 180, ToneMap: true, FastToneMap: fast})
				if err != nil {
					t.Fatal(err)
				}
				defer m.Stop(s.ID)
				ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
				defer cancel()
				select {
				case <-s.done:
				case <-ctx.Done():
					t.Fatal("conversion stalled")
				}
				if err = s.Err(); err != nil {
					t.Fatal(err)
				}
				playlist := filepath.Join(s.dir, "index.m3u8")
				details, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height,color_transfer", "-of", "csv=p=0", playlist).CombinedOutput()
				if err != nil || !strings.Contains(string(details), "320,180,bt709") {
					t.Fatalf("SDR dimensions/color: %s (%v)", details, err)
				}
				frame, err := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-ss", "0.5", "-i", playlist, "-frames:v", "1", "-pix_fmt", "gray", "-f", "rawvideo", "-").Output()
				if err != nil || len(frame) != 320*180 {
					t.Fatal("decoded frame", len(frame), err)
				}
				minY, maxY := 180, -1
				for index, pixel := range frame {
					if pixel > 200 {
						minY = min(minY, index/320)
						maxY = max(maxY, index/320)
					}
				}
				if burn < 0 && maxY >= 0 {
					t.Fatal("caption present with burn disabled")
				}
				if burn >= 0 && (minY < 164 || maxY < 167 || maxY > 171) {
					t.Fatalf("caption rows %d..%d want about 166..170", minY, maxY)
				}
			})
		}
	}
}
