package markers

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCreditsRun(t *testing.T) {
	f, c := false, true
	var frames []bool
	for range 100 {
		frames = append(frames, f) // the film
	}
	for i := range 60 {
		frames = append(frames, c || i%10 == 0) // the credits, with a few brighter cards
	}
	for range 20 {
		frames = append(frames, f) // a scene after the credits
	}
	start, end, ok := creditsRun(frames)
	if !ok || start != 100 || end != 160 {
		t.Fatalf("got %d-%d %v, want 100-160", start, end, ok)
	}
	if _, _, ok := creditsRun(frames[:130]); ok {
		t.Error("30 seconds of dark frames counted as credits")
	}
}

func TestDetectCredits(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("needs ffmpeg")
	}
	path := filepath.Join(t.TempDir(), "movie.mp4")
	// 19 minutes of colourful film, 150 seconds of white lines on black, then a 30-second scene.
	cmd := exec.Command(ffmpeg, "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=s=160x90:r=1:d=1140",
		"-f", "lavfi", "-i", "color=black:s=160x90:r=1:d=150,drawbox=x=40:y=20:w=80:h=4:color=white:t=fill,drawbox=x=50:y=50:w=60:h=4:color=white:t=fill",
		"-f", "lavfi", "-i", "testsrc2=s=160x90:r=1:d=30",
		"-filter_complex", "[0][1][2]concat=n=3:v=1:a=0", "-c:v", "libx264", "-preset", "ultrafast", "-g", "2", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("making movie: %v %s", err, out)
	}
	m, err := detectCredits(context.Background(), ffmpeg, path, 1320_000)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.StartMS < 1138_000 || m.StartMS > 1143_000 || m.EndMS < 1287_000 || m.EndMS > 1292_000 {
		t.Fatalf("credits = %+v, want about 1140s-1290s", m)
	}
}
