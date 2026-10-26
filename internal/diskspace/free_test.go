package diskspace

import (
	"runtime"
	"testing"
)

func TestAvailableOnCurrentFilesystem(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		t.Skip("free-space guard unavailable on this platform")
	}
	n, err := Available(t.TempDir())
	if err != nil || n == 0 {
		t.Fatalf("cannot establish available space: %d %v", n, err)
	}
}
