package subtitles

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/store"
)

const sampleASS = "[Script Info]\nTitle: Test\n\n[V4+ Styles]\nFormat: Name, Fontname\nStyle: Default,Kaiju Sans\n\n[Events]\n" +
	"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
	"Dialogue: 0,0:00:01.00,0:00:03.50,Default,,0,0,0,,{\\pos(10,20)}Hello, world\n" +
	"Dialogue: 0,0:00:20.00,0:00:22.00,Default,,0,0,0,,Later\n"

func TestPackScriptShiftsEventsAndDropsPast(t *testing.T) {
	out := string(PackScript([]byte(sampleASS), -10_000, nil))
	if strings.Contains(out, "Hello") {
		t.Fatalf("an event ending before 0 should be left out:\n%s", out)
	}
	if !strings.Contains(out, "Dialogue: 0,0:00:10.00,0:00:12.00,Default,,0,0,0,,Later") {
		t.Fatalf("event not shifted 10s earlier:\n%s", out)
	}
	// Styles and the rest of the script are kept as they are.
	if !strings.Contains(out, "Style: Default,Kaiju Sans") {
		t.Fatalf("styles changed:\n%s", out)
	}
	// Text with commas stays whole when shifting later.
	later := string(PackScript([]byte(sampleASS), 1_500, nil))
	if !strings.Contains(later, "Dialogue: 0,0:00:02.50,0:00:05.00,Default,,0,0,0,,{\\pos(10,20)}Hello, world") {
		t.Fatalf("event not shifted 1.5s later, or its text was split:\n%s", later)
	}
}

func TestPackScriptAddsFonts(t *testing.T) {
	out := string(PackScript([]byte(sampleASS), 0, []Font{{Name: "kaiju.ttf", Data: []byte("Man")}, {Name: "b.otf", Data: []byte{0xff}}}))
	i := strings.Index(out, "[Fonts]\n")
	if i < 0 || i < strings.Index(out, "[Events]") {
		t.Fatalf("no [Fonts] section after the events:\n%s", out)
	}
	// "Man" is 0x4d 0x61 0x6e: 6-bit groups 19, 22, 5, 46, each plus 33. One byte left over is 2 characters.
	if !strings.Contains(out, "fontname: kaiju.ttf\n47&O\n") || !strings.Contains(out, "fontname: b.otf\n`Q\n") {
		t.Fatalf("fonts not encoded as the ASS spec says:\n%s", out[i:])
	}
}

func TestUUEncodeWrapsAt80(t *testing.T) {
	enc := uuencodeASS(make([]byte, 90)) // 30 groups, 120 characters
	lines := strings.Split(strings.TrimSuffix(enc, "\n"), "\n")
	if len(lines) != 2 || len(lines[0]) != 80 || len(lines[1]) != 40 {
		t.Fatalf("lines %d, lengths %d/%d", len(lines), len(lines[0]), len(lines[len(lines)-1]))
	}
}

func TestASSTimes(t *testing.T) {
	for _, c := range []struct {
		in string
		ms int64
	}{{"0:00:01.00", 1000}, {"1:02:03.45", 3723450}, {" 0:00:00.07", 70}} {
		if got, ok := assTime(c.in); !ok || got != c.ms {
			t.Errorf("assTime(%q) = %d, %v", c.in, got, ok)
		}
	}
	if formatASSTime(3723450) != "1:02:03.45" {
		t.Errorf("formatASSTime = %s", formatASSTime(3723450))
	}
}

// TestScriptAndFonts copies an ASS track and the video's attached fonts with FFmpeg, when it's installed.
func TestScriptAndFonts(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	ass := filepath.Join(dir, "in.ass")
	os.WriteFile(ass, []byte(sampleASS), 0o644)
	font := filepath.Join(dir, "kaiju.ttf")
	os.WriteFile(font, []byte("not really a font, but attached as one"), 0o644)
	video := filepath.Join(dir, "Show.mkv")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc=duration=5:size=160x90", "-i", ass,
		"-attach", font, "-metadata:s:t", "mimetype=application/x-truetype-font",
		"-map", "0", "-map", "1", "-c:v", "libx264", "-c:s", "copy", video).CombinedOutput(); err != nil {
		t.Fatalf("making video: %v %s", err, out)
	}
	f := &store.MediaFile{ID: 7, Path: video, Streams: []store.Stream{{Index: 0, Kind: "video", Codec: "h264"}, {Index: 1, Kind: "subtitle", Codec: "ass"}}}
	tr, ok := Find(f, "s1")
	if !ok || !tr.IsASS() {
		t.Fatalf("track: %+v", tr)
	}
	c := NewCache(filepath.Join(dir, "cache"), func() string { return ffmpeg })
	script, err := c.Script(context.Background(), f, tr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "Style: Default") || !strings.Contains(string(script), "Hello, world") {
		t.Errorf("script lost its styles or events:\n%s", script)
	}
	fonts, err := c.Fonts(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 1 || fonts[0].Name != "kaiju.ttf" || string(fonts[0].Data) != "not really a font, but attached as one" {
		t.Fatalf("fonts: %+v", fonts)
	}
	// Both come from the cache the second time, even with FFmpeg gone.
	c.ffmpeg = func() string { return "" }
	if _, err := c.Script(context.Background(), f, tr); err != nil {
		t.Errorf("script not cached: %v", err)
	}
	if again, err := c.Fonts(context.Background(), f); err != nil || len(again) != 1 {
		t.Errorf("fonts not cached: %v %v", again, err)
	}
}
