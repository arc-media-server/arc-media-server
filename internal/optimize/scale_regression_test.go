package optimize

import (
	"os/exec"
	"strings"
	"testing"
)

func TestUltrawideOptimizedCopyFitsQualityBox(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	x := conversionTest(t)
	out, err := exec.Command(ffmpeg, "-y", "-v", "error", "-f", "lavfi", "-i", "color=red:size=3840x1600:rate=2:duration=1", "-c:v", "libx264", "-preset", "ultrafast", x.f.Path).CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	x.f.Streams[0].Width, x.f.Streams[0].Height = 3840, 1600
	x.save(t)
	x.m.FFmpeg = func() string { return ffmpeg }
	if err := x.m.convert(t.Context(), x.queue(t, 1080), nil); err != nil {
		t.Fatal(err)
	}
	copy := x.copy(t, 1080)
	out, err = exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height", "-of", "csv=p=0:s=x", copy.Path).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "1920x800") {
		t.Fatalf("not fitted to 1080 quality box: %v %s", err, out)
	}
}
