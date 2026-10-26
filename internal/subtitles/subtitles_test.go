package subtitles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ams/internal/store"
)

func TestSideFiles(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Inception (2010).mkv")
	for _, name := range []string{"Inception (2010).mkv", "Inception (2010).srt", "Inception (2010).en.srt",
		"Inception (2010).eng.forced.srt", "Inception (2010).French.sdh.ass", "Inception (2010).fre.vtt",
		"Inception (2010).commentary.srt", "Other Movie.en.srt", "Inception (2010) - Extended.en.srt", "Inception (2010).en.nfo"} {
		os.WriteFile(filepath.Join(dir, name), nil, 0o644)
	}
	got := map[string]Track{}
	for _, tr := range sideFiles(video) {
		got[filepath.Base(tr.Path)] = tr
	}
	if len(got) != 6 {
		t.Fatalf("found %d: %v", len(got), got)
	}
	check := func(name, lang string, forced, sdh bool, title string) {
		t.Helper()
		tr, ok := got[name]
		if !ok || tr.Language != lang || tr.Forced != forced || tr.HearingImpaired != sdh || tr.Title != title || !tr.External {
			t.Errorf("%s: %+v", name, tr)
		}
	}
	check("Inception (2010).srt", "", false, false, "")
	check("Inception (2010).en.srt", "eng", false, false, "")
	check("Inception (2010).eng.forced.srt", "eng", true, false, "")
	check("Inception (2010).French.sdh.ass", "fra", false, true, "")
	check("Inception (2010).fre.vtt", "fra", false, false, "")
	check("Inception (2010).commentary.srt", "", false, false, "commentary")
	if got["Inception (2010).French.sdh.ass"].Codec != "ass" {
		t.Error("codec from the extension")
	}
}

func TestParseVTT(t *testing.T) {
	cues := ParseVTT([]byte("WEBVTT\r\n\r\n00:00:01.000 --> 00:00:04.500\r\nHello <i>there</i> &amp; <c.yellow>you</c>\r\nsecond line\r\n\r\n" +
		"1:02:03.250 --> 1:02:05.000 align:start\n<font color=\"red\"><b>Bold</b></font>\n\n00:10.000 --> 00:12.000\n<script>x</script>\n"))
	if len(cues) != 3 {
		t.Fatalf("cues: %+v", cues)
	}
	if c := cues[0]; c.StartMS != 1000 || c.EndMS != 4500 || c.Text != "Hello <i>there</i> & you\nsecond line" {
		t.Errorf("first: %+v", c)
	}
	if c := cues[1]; c.StartMS != 10000 || c.Text != "x" {
		t.Errorf("short timing / stripped tags: %+v", c)
	}
	if c := cues[2]; c.StartMS != 3723250 || c.Text != "<b>Bold</b>" {
		t.Errorf("hours: %+v", c)
	}
}

func TestToUTF8(t *testing.T) {
	if got := string(toUTF8([]byte("caf\xe9 \x93quoted\x94"))); got != "café “quoted”" {
		t.Errorf("cp1252: %q", got)
	}
	if got := string(toUTF8([]byte("\xef\xbb\xbfalready ✓"))); got != "already ✓" {
		t.Errorf("utf8 with BOM: %q", got)
	}
}

// TestCues extracts subtitles with FFmpeg when it's installed.
func TestCues(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	srt := filepath.Join(dir, "Film.en.srt")
	srtText := "1\n00:00:01,000 --> 00:00:02,500\nCafé <i>time</i>\n\n2\n00:00:03,000 --> 00:00:04,000\nBye\n"
	os.WriteFile(srt, []byte(srtText), 0o644)
	video := filepath.Join(dir, "Film.mkv")
	if out, err := exec.Command(ffmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc=duration=5:size=160x90",
		"-i", srt, "-map", "0", "-map", "1", "-c:v", "libx264", "-c:s", "srt", video).CombinedOutput(); err != nil {
		t.Fatalf("making video: %v %s", err, out)
	}
	// The file beside it is in Windows-1252, as old subtitle files often are.
	os.WriteFile(srt, []byte(strings.ReplaceAll(srtText, "é", "\xe9")), 0o644)
	f := &store.MediaFile{ID: 42, Path: video, Streams: []store.Stream{{Index: 0, Kind: "video", Codec: "h264"}, {Index: 1, Kind: "subtitle", Codec: "subrip"}}}
	tracks := Tracks(f)
	if len(tracks) != 2 || tracks[0].ID != "s1" || !tracks[1].External {
		t.Fatalf("tracks: %+v", tracks)
	}
	c := NewCache(filepath.Join(dir, "cache"), func() string { return ffmpeg })
	for _, tr := range tracks {
		cues, err := c.Cues(context.Background(), f, tr)
		if err != nil {
			t.Fatalf("%s: %v", tr.ID, err)
		}
		if len(cues) != 2 || cues[0].StartMS != 1000 || cues[0].EndMS != 2500 || cues[0].Text != "Café <i>time</i>" {
			t.Errorf("%s cues: %+v", tr.ID, cues)
		}
	}
	// Cached the second time.
	if entries, _ := os.ReadDir(filepath.Join(dir, "cache", "42")); len(entries) != 2 {
		t.Errorf("cache has %d files", len(entries))
	}
}

func TestDownloads(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "Film.mkv")
	os.WriteFile(video, nil, 0o644)
	f := &store.MediaFile{ID: 42, Path: video}
	c := NewCache(filepath.Join(dir, "cache"), func() string { return "" })
	a, err := c.SaveDownload(f, "en", false, true, []byte("1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9\n"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.SaveDownload(f, "pt-BR", true, false, []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if a.Language != "eng" || !a.HearingImpaired || !a.Downloaded || b.Language != "por" || !b.Forced || a.ID == b.ID {
		t.Errorf("tracks: %+v %+v", a, b)
	}
	if got := len(c.Tracks(f)); got != 2 {
		t.Errorf("listed %d", got)
	}
	// Saved as UTF-8, in the data folder rather than beside the video.
	if data, _ := os.ReadFile(a.Path); !strings.Contains(string(data), "Café") || filepath.Dir(filepath.Dir(a.Path)) != filepath.Join(dir, "cache", "downloaded") {
		t.Errorf("saved %q at %s", data, a.Path)
	}
	for in, want := range map[string]string{"eng": "en", "fre": "fr", "fra": "fr", "pt": "pt", "xyz": ""} {
		if got := LanguageCode2(in); got != want {
			t.Errorf("LanguageCode2(%q) = %q", in, got)
		}
	}
}

func TestLegacyCuesAdoptFileOwnershipWithoutExtraction(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "film.mkv")
	if err := os.WriteFile(source, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(100, 0)
	if err := os.Chtimes(source, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|s1", source, info.Size(), info.ModTime().Unix())))
	key := hex.EncodeToString(sum[:12])
	cacheDir := filepath.Join(dir, "cache")
	if err := os.Mkdir(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(cacheDir, key+".json")
	if err := os.WriteFile(legacy, []byte(`[{"start_ms":1000,"end_ms":2000,"text":"Retained"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	c := NewCache(cacheDir, func() string { t.Error("legacy cache re-extracted media"); return "not-called" })
	cues, err := c.Cues(t.Context(), &store.MediaFile{ID: 7, Path: source}, Track{ID: "s1"})
	if err != nil || len(cues) != 1 || cues[0].Text != "Retained" {
		t.Fatalf("cues=%+v err=%v", cues, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("adopted legacy file retained: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "7", key+".json")); err != nil {
		t.Fatal(err)
	}
}
