package trickplay

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/store"
)

func TestNearest(t *testing.T) {
	frames := []frame{{ms: 0}, {ms: 4000}, {ms: 10400}}
	for ms, want := range map[int64]int{0: 0, 1900: 0, 2100: 1, 10000: 2, 50000: 2} {
		if got := nearest(frames, ms); got != want {
			t.Errorf("nearest(%d) = %d, want %d", ms, got, want)
		}
	}
}

// TestMake runs FFmpeg for real when it's installed, on files that once broke it: H.264, x265's
// open GOPs (keyframes decoded out of order), and a clip with a single keyframe.
func TestMake(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	ffprobe := filepath.Join(filepath.Dir(ffmpeg), "ffprobe")
	dir := t.TempDir()
	st, err := store.Open(t.Context(), filepath.Join(dir, "store.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	m := New(filepath.Join(dir, "tp"), st, func() string { return ffmpeg }, slog.New(slog.NewTextHandler(io.Discard, nil)))

	for i, tc := range []struct {
		name      string
		duration  string
		encode    []string
		wantCount int
	}{
		{"h264", "25", []string{"-c:v", "libx264", "-g", "50"}, 3},
		{"hevc", "21", []string{"-c:v", "libx265", "-x265-params", "log-level=error:keyint=250"}, 3},
		{"one keyframe", "3", []string{"-c:v", "libx264", "-g", "1000"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			id := int64(i + 1)
			src := filepath.Join(dir, tc.name+".mkv")
			args := append([]string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=duration=" + tc.duration + ":size=640x272:rate=25"}, tc.encode...)
			if out, err := exec.Command(ffmpeg, append(args, src)...).CombinedOutput(); err != nil {
				t.Fatalf("making source: %v %s", err, out)
			}
			stat, err := os.Stat(src)
			if err != nil {
				t.Fatal(err)
			}
			savedID, err := st.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: src, Size: stat.Size(), ModTime: stat.ModTime().Unix()}, 1, 1)
			if err != nil || savedID != id {
				t.Fatalf("file id=%d err=%v", savedID, err)
			}
			if m.Info(id) != nil {
				t.Fatal("info before making")
			}
			dur := map[string]int64{"25": 25_000, "21": 21_000, "3": 3_000}[tc.duration]
			if err := m.make(context.Background(), store.VideoFile{ID: id, Path: src, DurationMS: dur, Width: 640, Height: 272}); err != nil {
				t.Fatal(err)
			}
			info := m.Info(id)
			if info == nil || info.Count != tc.wantCount || info.Sheets != 1 || info.Width != 320 || info.Height != 136 {
				t.Fatalf("info: %+v", info)
			}
			if _, ok := m.Sheet(id, 1); ok {
				t.Error("sheet 1 shouldn't exist")
			}
			out, _ := exec.Command(ffprobe, "-v", "error", "-show_entries", "stream=width,height", "-of", "csv=p=0",
				func() string {
					path, ok := m.Sheet(id, 0)
					if !ok {
						t.Fatal("sheet missing")
					}
					return path
				}()).Output()
			if got := strings.TrimSpace(string(out)); got != "3200,1360" {
				t.Errorf("sheet size %q", got)
			}
		})
	}
}
