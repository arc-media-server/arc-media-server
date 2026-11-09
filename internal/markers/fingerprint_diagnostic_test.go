package markers

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestFingerprintDiagnosticsBoundedAndPrivate(t *testing.T) {
	var b fingerprintStderr
	b.Write([]byte("opening /private/media/title.mkv https://example.invalid/token?key=fake-secret\n"))
	b.Write([]byte(strings.Repeat("x", 100000)))
	if b.Len() > 4096 {
		t.Fatal("unbounded stderr")
	}
	detail := fingerprintDetail(b.String(), "/private/media/title.mkv")
	if strings.Contains(detail, "/private/") || strings.Contains(detail, "fake-secret") || strings.Contains(detail, "https://") {
		t.Fatalf("unsafe diagnostic: %s", detail)
	}
}
func TestFingerprintCancellationIsNotAFileFailure(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = fingerprint(ctx, ffmpeg, "unused", 0, 60)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
