// Package subtitles finds a video's subtitles (inside the file, or in files beside it named like
// "Movie (2010).en.forced.srt") and turns text subtitles into cues the player draws itself.
// Picture subtitles (Blu-ray PGS, DVD) can't be drawn as text: they're burned into the video
// while converting it.
package subtitles

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ams/internal/store"
)

// Track is one subtitle track a video has.
type Track struct {
	// "s<stream index>" for one inside the file, "x<hash>" for a file beside it.
	ID       string `json:"id"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Codec    string `json:"codec"`
	Forced   bool   `json:"forced"`
	Default  bool   `json:"default"`
	// For people who are deaf or hard of hearing (SDH / CC).
	HearingImpaired bool `json:"hearing_impaired"`
	External        bool `json:"external"`
	// Found online and kept in Arc's data folder (never beside the video).
	Downloaded bool `json:"downloaded"`
	// A picture format: shown by burning it into the video.
	Image bool `json:"image"`

	StreamIndex int    `json:"-"` // inside the file
	Path        string `json:"-"` // beside it
}

// Cue is one subtitle on screen.
type Cue struct {
	StartMS int64 `json:"start_ms"`
	EndMS   int64 `json:"end_ms"`
	// The text, with line breaks and only <i>, <b> and <u> kept.
	Text string `json:"text"`
}

var textCodecs = map[string]bool{"subrip": true, "srt": true, "ass": true, "ssa": true, "mov_text": true,
	"webvtt": true, "text": true, "microdvd": true, "subviewer": true}

var imageCodecs = map[string]bool{"hdmv_pgs_subtitle": true, "dvd_subtitle": true, "dvb_subtitle": true, "xsub": true}

var sideExts = map[string]string{".srt": "subrip", ".vtt": "webvtt", ".ass": "ass", ".ssa": "ssa", ".sub": "microdvd"}

// Tracks lists a file's subtitles: those inside it, then files beside it.
func Tracks(f *store.MediaFile) []Track {
	out := []Track{}
	for _, st := range f.Streams {
		if st.Kind != "subtitle" || (!textCodecs[st.Codec] && !imageCodecs[st.Codec]) {
			continue
		}
		title := strings.ToLower(st.Title)
		out = append(out, Track{ID: "s" + strconv.Itoa(st.Index), Language: st.Language, Title: st.Title, Codec: st.Codec,
			Forced: st.IsForced || strings.Contains(title, "forced"), Default: st.IsDefault,
			HearingImpaired: strings.Contains(title, "sdh") || strings.Contains(title, "hearing"),
			Image:           imageCodecs[st.Codec], StreamIndex: st.Index})
	}
	return append(out, sideFiles(f.Path)...)
}

// sideFiles finds subtitle files beside a video whose names start with the video's name.
func sideFiles(videoPath string) []Track {
	dir := filepath.Dir(videoPath)
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Track
	for _, e := range entries {
		name := e.Name()
		codec, ok := sideExts[strings.ToLower(filepath.Ext(name))]
		if e.IsDir() || !ok {
			continue
		}
		base := strings.TrimSuffix(name, filepath.Ext(name))
		if !strings.EqualFold(base, stem) && !strings.HasPrefix(strings.ToLower(base), strings.ToLower(stem)+".") {
			continue
		}
		t := Track{Codec: codec, External: true, Path: filepath.Join(dir, name)}
		// The parts after the video's name: a language, and flags like forced or sdh.
		for _, part := range strings.Split(strings.ToLower(base[len(stem):]), ".") {
			switch part {
			case "":
			case "forced", "foreign":
				t.Forced = true
			case "sdh", "cc", "hi":
				t.HearingImpaired = true
			case "default":
				t.Default = true
			default:
				if t.Language == "" {
					t.Language = languageCode(part)
				}
				if t.Language == "" {
					t.Title = part
				}
			}
		}
		sum := sha256.Sum256([]byte(name))
		t.ID = "x" + hex.EncodeToString(sum[:6])
		out = append(out, t)
	}
	slices.SortFunc(out, func(a, b Track) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// Find returns a file's track by id.
func Find(f *store.MediaFile, id string) (Track, bool) {
	for _, t := range Tracks(f) {
		if t.ID == id {
			return t, true
		}
	}
	return Track{}, false
}

func (c *Cache) extract(ctx context.Context, src string, t Track) ([]Cue, error) {
	bin := c.ffmpeg()
	if bin == "" {
		return nil, errors.New("FFmpeg wasn't found, so subtitles can't be read")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	args := []string{"-hide_banner", "-nostdin", "-v", "error"}
	var stdin []byte
	if t.External {
		b, err := os.ReadFile(src)
		if err != nil {
			return nil, err
		}
		// Old subtitle files are often in Windows-1252 rather than UTF-8.
		stdin = toUTF8(b)
		args = append(args, "-f", ffmpegFormat(t.Codec), "-i", "pipe:0", "-map", "0:s:0")
	} else {
		args = append(args, "-i", src, "-map", "0:"+strconv.Itoa(t.StreamIndex))
	}
	args = append(args, "-c:s", "webvtt", "-f", "webvtt", "pipe:1")
	cmd := exec.CommandContext(ctx, bin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndex(msg, "\n"); i >= 0 {
			msg = msg[i+1:]
		}
		return nil, fmt.Errorf("couldn't read the subtitles: %s", msg)
	}
	return ParseVTT(out), nil
}

func ffmpegFormat(codec string) string {
	switch codec {
	case "subrip":
		return "srt"
	case "ssa":
		return "ass"
	}
	return codec
}

// toUTF8 returns b as UTF-8, reading it as Windows-1252 if it isn't valid UTF-8 already.
func toUTF8(b []byte) []byte {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	if utf8.Valid(b) {
		return b
	}
	var sb strings.Builder
	for _, c := range b {
		if r, ok := cp1252[c]; ok {
			sb.WriteRune(r)
		} else {
			sb.WriteRune(rune(c)) // the rest of Windows-1252 matches Latin-1
		}
	}
	return []byte(sb.String())
}

var cp1252 = map[byte]rune{0x80: '€', 0x82: '‚', 0x83: 'ƒ', 0x84: '„', 0x85: '…', 0x86: '†', 0x87: '‡', 0x88: 'ˆ',
	0x89: '‰', 0x8A: 'Š', 0x8B: '‹', 0x8C: 'Œ', 0x8E: 'Ž', 0x91: '‘', 0x92: '’', 0x93: '“', 0x94: '”', 0x95: '•',
	0x96: '–', 0x97: '—', 0x98: '˜', 0x99: '™', 0x9A: 'š', 0x9B: '›', 0x9C: 'œ', 0x9E: 'ž', 0x9F: 'Ÿ'}

var (
	timingRe = regexp.MustCompile(`^((?:\d+:)?\d{1,2}:\d{2}[.,]\d{3})\s+-->\s+((?:\d+:)?\d{1,2}:\d{2}[.,]\d{3})`)
	tagRe    = regexp.MustCompile(`<[^>]*>`)
	keepRe   = regexp.MustCompile(`^</?[ibu]>$`)
)

// ParseVTT reads WebVTT cues.
func ParseVTT(b []byte) []Cue {
	cues := []Cue{}
	sc := bufio.NewScanner(bytes.NewReader(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	var cur *Cue
	var lines []string
	flush := func() {
		if cur != nil && len(lines) > 0 {
			cur.Text = cleanText(strings.Join(lines, "\n"))
			if cur.Text != "" {
				cues = append(cues, *cur)
			}
		}
		cur, lines = nil, nil
	}
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t")
		if m := timingRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &Cue{StartMS: vttTime(m[1]), EndMS: vttTime(m[2])}
			continue
		}
		if line == "" {
			flush()
			continue
		}
		if cur != nil {
			lines = append(lines, line)
		}
	}
	flush()
	slices.SortStableFunc(cues, func(a, b Cue) int { return int(a.StartMS - b.StartMS) })
	return cues
}

// cleanText keeps only <i>, <b> and <u> tags and decodes entities.
func cleanText(s string) string {
	s = tagRe.ReplaceAllStringFunc(s, func(tag string) string {
		if keepRe.MatchString(strings.ToLower(tag)) {
			return strings.ToLower(tag)
		}
		return ""
	})
	// Entities are decoded, then the kept tags are the only markup the player recognises.
	return strings.TrimSpace(html.UnescapeString(s))
}

func vttTime(s string) int64 {
	s = strings.ReplaceAll(s, ",", ".")
	parts := strings.Split(s, ":")
	var h, m int64
	var sec float64
	switch len(parts) {
	case 3:
		h, _ = strconv.ParseInt(parts[0], 10, 64)
		m, _ = strconv.ParseInt(parts[1], 10, 64)
		sec, _ = strconv.ParseFloat(parts[2], 64)
	case 2:
		m, _ = strconv.ParseInt(parts[0], 10, 64)
		sec, _ = strconv.ParseFloat(parts[1], 64)
	}
	return (h*3600+m*60)*1000 + int64(sec*1000+0.5)
}

// languageCode recognises a language in a subtitle file name: a 2- or 3-letter code or an
// English name. It returns the 3-letter code, or "" if it isn't one.
func languageCode(s string) string {
	s = strings.ToLower(s)
	if c, ok := langNames[s]; ok {
		return c
	}
	for code3, code2 := range langCodes {
		if s == code3 || s == code2 {
			return code3
		}
	}
	if b, ok := bibliographic[s]; ok {
		return b
	}
	return ""
}

// ISO 639-2 codes and their 639-1 equivalents, for the languages subtitles are commonly in.
var langCodes = map[string]string{"eng": "en", "fra": "fr", "deu": "de", "spa": "es", "ita": "it", "por": "pt",
	"nld": "nl", "swe": "sv", "nor": "no", "dan": "da", "fin": "fi", "pol": "pl", "ces": "cs", "hun": "hu",
	"ron": "ro", "ell": "el", "tur": "tr", "rus": "ru", "ukr": "uk", "ara": "ar", "heb": "he", "hin": "hi",
	"jpn": "ja", "kor": "ko", "zho": "zh", "tha": "th", "vie": "vi", "ind": "id", "msa": "ms", "bul": "bg",
	"hrv": "hr", "srp": "sr", "slk": "sk", "slv": "sl", "est": "et", "lav": "lv", "lit": "lt", "isl": "is",
	"cat": "ca", "eus": "eu", "glg": "gl", "fas": "fa", "urd": "ur", "ben": "bn", "tam": "ta", "tel": "te",
	"fil": "tl"}

// Older bibliographic codes some tools write.
var bibliographic = map[string]string{"fre": "fra", "ger": "deu", "dut": "nld", "cze": "ces", "rum": "ron",
	"gre": "ell", "chi": "zho", "per": "fas", "slo": "slk", "ice": "isl", "baq": "eus", "may": "msa"}

var langNames = map[string]string{"english": "eng", "french": "fra", "german": "deu", "spanish": "spa",
	"italian": "ita", "portuguese": "por", "dutch": "nld", "swedish": "swe", "norwegian": "nor", "danish": "dan",
	"finnish": "fin", "polish": "pol", "czech": "ces", "hungarian": "hun", "romanian": "ron", "greek": "ell",
	"turkish": "tur", "russian": "rus", "ukrainian": "ukr", "arabic": "ara", "hebrew": "heb", "hindi": "hin",
	"japanese": "jpn", "korean": "kor", "chinese": "zho", "thai": "tha", "vietnamese": "vie", "indonesian": "ind"}

// downloadDir is where subtitles found online for a file are kept.
func (c *Cache) downloadDir(fileID int64) string {
	return filepath.Join(c.dir, "downloaded", strconv.FormatInt(fileID, 10))
}

// Tracks lists a file's subtitles: inside it, beside it, then any downloaded for it.
func (c *Cache) Tracks(f *store.MediaFile) []Track {
	out := Tracks(f)
	entries, err := os.ReadDir(c.downloadDir(f.ID))
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".srt") {
			continue
		}
		// Named <language>[.forced][.sdh].<n>.srt
		t := Track{Codec: "subrip", External: true, Downloaded: true, Path: filepath.Join(c.downloadDir(f.ID), name)}
		for i, part := range strings.Split(strings.TrimSuffix(name, ".srt"), ".") {
			switch {
			case i == 0:
				t.Language = languageCode(part)
			case part == "forced":
				t.Forced = true
			case part == "sdh":
				t.HearingImpaired = true
			}
		}
		sum := sha256.Sum256([]byte("downloaded/" + name))
		t.ID = "d" + hex.EncodeToString(sum[:6])
		out = append(out, t)
	}
	return out
}

// Find returns a file's track by id, including downloaded ones.
func (c *Cache) Find(f *store.MediaFile, id string) (Track, bool) {
	for _, t := range c.Tracks(f) {
		if t.ID == id {
			return t, true
		}
	}
	return Track{}, false
}

// SaveDownload keeps a subtitle file found online for a video file, in Arc's data folder.
// language is a 2- or 3-letter code.
func (c *Cache) SaveDownload(f *store.MediaFile, language string, forced, sdh bool, data []byte) (Track, error) {
	lang := languageCode(strings.SplitN(strings.ToLower(language), "-", 2)[0])
	if lang == "" {
		lang = "und"
	}
	name := lang
	if forced {
		name += ".forced"
	}
	if sdh {
		name += ".sdh"
	}
	dir := c.downloadDir(f.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Track{}, err
	}
	n := 1
	for {
		if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("%s.%d.srt", name, n))); err != nil {
			break
		}
		n++
	}
	file := fmt.Sprintf("%s.%d.srt", name, n)
	if err := os.WriteFile(filepath.Join(dir, file), toUTF8(data), 0o644); err != nil {
		return Track{}, err
	}
	sum := sha256.Sum256([]byte("downloaded/" + file))
	id := "d" + hex.EncodeToString(sum[:6])
	t, ok := c.Find(f, id)
	if !ok {
		return Track{}, errors.New("the downloaded subtitles couldn't be read back")
	}
	return t, nil
}

// SameLanguage reports whether two language codes or names mean the same language ("en", "eng"
// and "English" do; so do "ger" and "deu").
func SameLanguage(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ca, cb := languageCode(a), languageCode(b)
	if ca == "" || cb == "" {
		return strings.EqualFold(a, b)
	}
	return ca == cb
}

// LanguageCode2 turns a 3-letter language code into the 2-letter one services use ("eng" -> "en").
func LanguageCode2(code string) string {
	code = strings.ToLower(code)
	if c, ok := bibliographic[code]; ok {
		code = c
	}
	if two, ok := langCodes[code]; ok {
		return two
	}
	if len(code) == 2 {
		return code
	}
	return ""
}
