package transcode

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestADTSAACRepackageAndConvert(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe, _ := exec.LookPath("ffprobe")
	dir := t.TempDir()
	source := filepath.Join(dir, "adts.ts")
	out, err := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=duration=8:size=160x90:rate=25", "-f", "lavfi", "-i", "sine=duration=8", "-c:v", "libx264", "-g", "50", "-c:a", "aac", "-ac", "2", "-f", "mpegts", source).CombinedOutput()
	if err != nil {
		t.Fatalf("generate ADTS TS: %v %s", err, out)
	}
	for _, copyVideo := range []bool{true, false} {
		t.Run(map[bool]string{true: "remux", false: "convert"}[copyVideo], func(t *testing.T) {
			m := New(filepath.Join(t.TempDir(), "sessions"), func() string { return ffmpeg }, func() string { return ffprobe }, slog.New(slog.NewTextHandler(io.Discard, nil)))
			s, err := m.Start(Options{Path: source, VideoStream: 0, AudioStream: 1, CopyVideo: copyVideo, CopyAudio: true, AudioCodec: "aac", BurnSubtitle: -1})
			if err != nil {
				t.Fatal(err)
			}
			defer m.Stop(s.ID)
			select {
			case <-s.done:
			case <-time.After(30 * time.Second):
				t.Fatal("conversion did not finish")
			}
			if err := s.Err(); err != nil {
				t.Fatalf("ADTS audio failed: %v", err)
			}
			if err := m.Ready(t.Context(), s); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=codec_name", "-of", "csv=p=0", filepath.Join(s.dir, "index.m3u8")).CombinedOutput()
			if err != nil || !strings.Contains(string(out), "aac") {
				t.Fatalf("playable AAC missing: %v %s", err, out)
			}
		})
	}
}

func TestFailedConversionNeverServesPartialPlaylist(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.m3u8"), []byte("#EXTM3U\n#EXTINF:0.1,\ns00000.m4s\n"), 0600); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("conversion crashed after first segment")
	s := &Session{dir: dir, done: make(chan struct{}), err: failure}
	close(s.done)
	if _, err := s.Playlist(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("partial failed playlist served: %v", err)
	}
}

func TestCopiedAACFilterDoesNotTouchDolby(t *testing.T) {
	for _, codec := range []string{"aac", "ac3", "eac3"} {
		args := strings.Join(buildArgs(Options{Path: "in.ts", AudioStream: 1, CopyAudio: true, AudioCodec: codec, CopyVideo: true, BurnSubtitle: -1}, 0, "out"), " ")
		if strings.Contains(args, "aac_adtstoasc") != (codec == "aac") {
			t.Errorf("wrong filter for %s: %s", codec, args)
		}
	}
}

func TestBoundedFFmpegDiagnosticsPreserveCauseAndRedactURL(t *testing.T) {
	var b stderrBuffer
	first := "Malformed AAC bitstream\n"
	b.Write([]byte(first))
	for range 100 {
		b.Write([]byte(strings.Repeat("filler\n", 100)))
	}
	b.Write([]byte("Error writing trailer https://example.invalid/private?api_key=fake-secret\n"))
	if len(b.head)+len(b.tail) > stderrLimit {
		t.Fatal("unbounded stderr")
	}
	detail := ffmpegFailure(b.String(), "")
	if !strings.Contains(detail, "Malformed AAC") || !strings.Contains(detail, "Error writing trailer") {
		t.Fatalf("cause/outcome missing: %s", detail)
	}
	if strings.Contains(detail, "fake-secret") || strings.Contains(detail, "https://") {
		t.Fatalf("URL leaked: %s", detail)
	}
}

func TestPlaylistWaitKeepsPendingStartAlive(t *testing.T) {
	s := &Session{dir: t.TempDir(), done: make(chan struct{}), lastSeen: time.Now().Add(-2 * idleTimeout)}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if _, err := s.Playlist(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected wait: %v", err)
	}
	if time.Since(s.lastSeen) > time.Second {
		t.Fatal("pending readiness left idle timestamp stale")
	}
}
