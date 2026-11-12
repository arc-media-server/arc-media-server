package optimize

import (
	"os"
	"strings"
	"testing"
)

func TestConversionFailureKeepsCauseAndRedactsSource(t *testing.T) {
	x := conversionTest(t)
	t.Setenv("ARC_OPTIMIZE_FAILURE_SOURCE", x.f.Path)
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	x.m.FFmpeg = func() string { return bin }
	err = x.m.convert(t.Context(), x.queue(t, 720), nil)
	if err == nil {
		t.Fatal("failed child succeeded")
	}
	msg := err.Error()
	if !strings.Contains(msg, "decode failed") || !strings.Contains(msg, "conversion failed") {
		t.Fatalf("lost cause/outcome: %q", msg)
	}
	if strings.Contains(msg, x.f.Path) || strings.Contains(msg, "fixture.invalid") || strings.Contains(msg, "secret") || len(msg) > 1100 {
		t.Fatalf("unbounded or private diagnostic: %q", msg)
	}
	saved, readErr := x.st.OptimizedVersions(t.Context())
	if readErr != nil || len(saved) != 1 || saved[0].Status != "failed" || saved[0].Error != msg {
		t.Fatalf("persisted diagnostic: %+v %v", saved, readErr)
	}
}
