package optimize

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDV5OptimizationRefusesUnverifiedColorConversion(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	x := conversionTest(t)
	x.f.Streams[0].Codec = "hevc"
	x.f.Streams[0].DolbyVision = true
	x.f.Streams[0].DolbyVisionProfile = 5
	x.save(t)
	x.m.FFmpeg = func() string { return ffmpeg }
	x.m.ToneMap = func() bool { return true }
	err = x.m.convert(t.Context(), x.queue(t, 720), nil)
	if err == nil || !strings.Contains(err.Error(), "Dolby Vision profile 5") {
		t.Fatalf("unsafe optimizer admitted: %v", err)
	}
	copies, readErr := x.st.OptimizedVersions(t.Context())
	if readErr != nil || len(copies) != 1 || copies[0].Status != "failed" || !strings.Contains(copies[0].Error, "Dolby Vision profile 5") {
		t.Fatalf("missing actionable refusal: %+v %v", copies, readErr)
	}
}

func TestDV5ExistingOptimizedCopyNotSelected(t *testing.T) {
	x := conversionTest(t)
	helperConverter(t, x)
	if err := os.WriteFile(filepath.Join(x.root, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := x.m.convert(t.Context(), x.queue(t, 720), nil); err != nil {
		t.Fatal(err)
	}
	copy := x.copy(t, 720)
	if !Usable(&copy, x.f) {
		t.Fatal("ordinary copy unexpectedly refused")
	}
	x.f.Streams[0].DolbyVision = true
	x.f.Streams[0].DolbyVisionProfile = 5
	if Usable(&copy, x.f) {
		t.Fatal("old unverified DV5 copy still selected")
	}
}
