package optimize

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestOptimizedHDRCopyResizesAndMapsToSDR(t *testing.T) {
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
	for _, transfer := range []string{"smpte2084", "arib-std-b67"} {
		t.Run(transfer, func(t *testing.T) {
			x := conversionTest(t)
			trc := 16
			if transfer == "arib-std-b67" {
				trc = 18
			}
			out, err := exec.Command(ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=white:size=1920x1080:rate=2:duration=1", "-c:v", "libx265", "-preset", "ultrafast", "-pix_fmt", "yuv420p10le", "-x265-params", "log-level=error:pools=1", "-bsf:v", fmt.Sprintf("hevc_metadata=colour_primaries=9:transfer_characteristics=%d:matrix_coefficients=9", trc), x.f.Path).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(out))
			}
			details, probeErr := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=color_primaries,color_transfer,color_space,color_range", "-of", "csv=p=0", x.f.Path).CombinedOutput()
			if probeErr != nil || !strings.Contains(string(details), transfer) || !strings.Contains(string(details), "bt2020") {
				t.Fatalf("invalid HDR fixture color: %s %v", details, probeErr)
			}
			x.f.Streams[0].Width, x.f.Streams[0].Height, x.f.Streams[0].ColorTransfer = 1920, 1080, transfer
			x.save(t)
			x.m.FFmpeg = func() string { return ffmpeg }
			x.m.ToneMap = func() bool { return true }
			if err := x.m.convert(t.Context(), x.queue(t, 720), nil); err != nil {
				t.Fatal(err)
			}
			copy := x.copy(t, 720)
			out, err = exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height,color_transfer", "-of", "csv=p=0", copy.Path).CombinedOutput()
			if err != nil || !strings.Contains(string(out), "1280,720,bt709") {
				t.Fatalf("dimensions/SDR: %s %v", out, err)
			}
		})
	}
}
