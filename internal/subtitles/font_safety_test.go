package subtitles

import (
	"ams/internal/store"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFontAttachmentNamesCannotEscapeCache(t *testing.T) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the real attachment regression")
	}
	for _, kind := range []string{"absolute", "traversal", "windows-separator"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			outside := filepath.Join(root, "outside.ttf")
			name := outside
			if kind == "traversal" {
				name = "../../../outside.ttf"
			}
			if kind == "windows-separator" {
				name = `..\outside.ttf`
			}
			original := []byte("existing file must remain unchanged")
			if err = os.WriteFile(outside, original, 0600); err != nil {
				t.Fatal(err)
			}
			attachment := filepath.Join(root, "input-font.ttf")
			if err = os.WriteFile(attachment, []byte("fixture attachment bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			media := filepath.Join(root, "fixture.mkv")
			cmd := exec.Command(bin, "-v", "error", "-nostdin", "-f", "lavfi", "-i", "color=size=32x32:duration=0.1", "-c:v", "libx264", "-threads", "1", "-attach", attachment, "-metadata:s:t:0", "mimetype=application/x-truetype-font", "-metadata:s:t:0", "filename="+name, media)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("create fixture: %v %s", err, out)
			}
			cache := NewCache(filepath.Join(root, "cache"), func() string { return bin })
			fonts, err := cache.Fonts(context.Background(), &store.MediaFile{ID: 7, Path: media})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(outside)
			if err != nil || string(got) != string(original) {
				t.Errorf("attachment overwrote a file outside the cache: %q %v", got, err)
			}
			if len(fonts) != 0 {
				t.Errorf("unsafe attachment name was accepted: %+v", fonts)
			}
		})
	}
}

func TestFontExtractionNeverDelegatesOutputName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command-boundary wrapper; crafted attachment tests run on every platform")
	}
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg fixture required")
	}
	root := t.TempDir()
	font := filepath.Join(root, "font.ttf")
	os.WriteFile(font, []byte("fixture-font"), 0600)
	media := filepath.Join(root, "fixture.mkv")
	if out, err := exec.Command(bin, "-v", "error", "-f", "lavfi", "-i", "color=size=32x32:duration=0.1", "-c:v", "libx264", "-threads", "1", "-attach", font, "-metadata:s:t:0", "mimetype=application/x-truetype-font", media).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v %s", err, out)
	}
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	argsFile := filepath.Join(root, "arguments")
	wrapper := filepath.Join(root, "ffmpeg-wrapper")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" >> %s\nexec %s \"$@\"\n", quote(argsFile), quote(bin))
	if err = os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	c := NewCache(filepath.Join(root, "cache"), func() string { return wrapper })
	if _, err = c.Fonts(t.Context(), &store.MediaFile{ID: 9, Path: media}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(string(data), "\n")
	found := false
	for i, arg := range args {
		if strings.HasPrefix(arg, "-dump_attachment:") {
			found = true
			if i+1 >= len(args) || args[i+1] == "" {
				t.Error("FFmpeg was allowed to choose an attachment's output path from untrusted metadata")
			}
		}
	}
	if !found {
		t.Fatal("fixture did not exercise attachment extraction")
	}
}
