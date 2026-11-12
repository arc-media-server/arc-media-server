// Package probe reads container and stream information from media files using ffprobe.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Prober struct {
	mu  sync.Mutex
	bin string
}

func New(bin string) *Prober {
	if bin == "" {
		bin = "ffprobe"
	}
	return &Prober{bin: bin}
}

// SetBin changes the ffprobe binary used.
func (p *Prober) SetBin(bin string) {
	p.mu.Lock()
	p.bin = bin
	p.mu.Unlock()
}

// Bin returns the ffprobe binary in use.
func (p *Prober) Bin() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bin
}

// Find looks for a working ffprobe: preferred (if set), then on the PATH, next to AMS itself, and
// in the usual install locations. It returns the binary and its version line.
func Find(ctx context.Context, preferred string) (bin, version string, err error) {
	var candidates []string
	if preferred != "" {
		candidates = append(candidates, preferred)
	}
	exe := "ffprobe"
	if runtime.GOOS == "windows" {
		exe = "ffprobe.exe"
	}
	if p, err := exec.LookPath("ffprobe"); err == nil {
		candidates = append(candidates, p)
	}
	if self, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(self), exe), filepath.Join(filepath.Dir(self), "ffmpeg", "bin", exe))
	}
	switch runtime.GOOS {
	case "windows":
		// winget installs FFmpeg into a versioned folder and only updates PATH for new shells.
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			m, _ := filepath.Glob(filepath.Join(local, "Microsoft", "WinGet", "Packages", "*FFmpeg*", "*", "bin", exe))
			candidates = append(candidates, m...)
		}
		for _, base := range []string{os.Getenv("ProgramFiles"), `C:\`} {
			if base != "" {
				candidates = append(candidates, filepath.Join(base, "ffmpeg", "bin", exe))
			}
		}
		if choco := os.Getenv("ChocolateyInstall"); choco != "" {
			candidates = append(candidates, filepath.Join(choco, "bin", exe))
		}
	default:
		candidates = append(candidates, "/usr/bin/ffprobe", "/usr/local/bin/ffprobe", "/opt/homebrew/bin/ffprobe",
			"/usr/lib/jellyfin-ffmpeg/ffprobe")
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		if v, err := New(c).Version(ctx); err == nil {
			return c, v, nil
		}
	}
	return "", "", errors.New("ffprobe wasn't found; install FFmpeg, or give the full path to ffprobe in Settings")
}

type Result struct {
	Container  string
	DurationMS int64
	Bitrate    int64
	Streams    []Stream
	Chapters   []Chapter
}

// Chapter is a chapter mark in a file.
type Chapter struct {
	StartMS, EndMS int64
	Title          string
}

type Stream struct {
	Index               int
	Kind                string
	Codec               string
	Profile             string
	Language            string
	Title               string
	Width               int
	Height              int
	Channels            int
	IsDefault           bool
	IsForced            bool
	ChannelLayout       string
	DolbyVision         bool
	DVBLCompatibilityID *int
	DolbyVisionProfile  int
	HDR10Plus           bool
	HDRProbeStatus      string
	// Video: pixel format, bits per sample, colour transfer/primaries (HDR) and frames per second.
	PixFmt         string
	BitDepth       int
	ColorTransfer  string
	ColorPrimaries string
	FrameRate      float64
	Bitrate        int64
	// AspectRatio is how wide the picture is shown relative to its height (1.78 for 16:9,
	// 2.39 for scope), with anamorphic pixels taken into account; 0 if unknown.
	AspectRatio float64
}

type ffprobeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		Index         int               `json:"index"`
		CodecType     string            `json:"codec_type"`
		CodecName     string            `json:"codec_name"`
		Profile       string            `json:"profile"`
		Width         int               `json:"width"`
		Height        int               `json:"height"`
		Channels      int               `json:"channels"`
		ChannelLayout string            `json:"channel_layout"`
		SampleAspect  string            `json:"sample_aspect_ratio"`
		DisplayAspect string            `json:"display_aspect_ratio"`
		SideData      []formatSideData  `json:"side_data_list"`
		PixFmt        string            `json:"pix_fmt"`
		BitsRaw       string            `json:"bits_per_raw_sample"`
		Transfer      string            `json:"color_transfer"`
		Primaries     string            `json:"color_primaries"`
		AvgRate       string            `json:"avg_frame_rate"`
		BitRate       string            `json:"bit_rate"`
		Tags          map[string]string `json:"tags"`
		Disposition   map[string]int    `json:"disposition"`
	} `json:"streams"`
	Chapters []struct {
		StartTime string            `json:"start_time"`
		EndTime   string            `json:"end_time"`
		Tags      map[string]string `json:"tags"`
	} `json:"chapters"`
}

// Version runs ffprobe -version to check that the binary is usable.
func (p *Prober) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, p.Bin(), "-version").Output()
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line), nil
}

// probeDelay, from AMS_PROBE_DELAY_MS, slows every probe down to mimic slow disks when timing
// scans in tests.
var probeDelay, _ = strconv.Atoi(os.Getenv("AMS_PROBE_DELAY_MS"))

func (p *Prober) Probe(ctx context.Context, path string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if probeDelay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(probeDelay) * time.Millisecond):
		}
	}

	cmd := exec.CommandContext(ctx, p.Bin(), "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", "-show_chapters", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := errorMessage(stderr.String(), path); msg != "" {
			return nil, fmt.Errorf("%s", msg)
		}
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	var raw ffprobeOutput
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("ffprobe output: %w", err)
	}

	res := &Result{Container: containerName(raw.Format.FormatName, path)}
	if d, err := strconv.ParseFloat(raw.Format.Duration, 64); err == nil {
		res.DurationMS = int64(d * 1000)
	}
	res.Bitrate, _ = strconv.ParseInt(raw.Format.BitRate, 10, 64)

	for _, s := range raw.Streams {
		tags := make(map[string]string, len(s.Tags))
		for k, v := range s.Tags {
			tags[strings.ToLower(k)] = v
		}
		res.Streams = append(res.Streams, Stream{
			Index:         s.Index,
			Kind:          s.CodecType,
			Codec:         s.CodecName,
			Profile:       s.Profile,
			Language:      tags["language"],
			Title:         tags["title"],
			Width:         s.Width,
			Height:        s.Height,
			Channels:      s.Channels,
			ChannelLayout: s.ChannelLayout,
			IsDefault:     s.Disposition["default"] == 1,
			IsForced:      s.Disposition["forced"] == 1,
			PixFmt:        s.PixFmt, BitDepth: bitDepth(s.BitsRaw, s.PixFmt),
			ColorTransfer: s.Transfer, ColorPrimaries: s.Primaries, FrameRate: frameRate(s.AvgRate),
			Bitrate: parseInt(s.BitRate), AspectRatio: aspectRatio(s.CodecType, s.Width, s.Height, s.SampleAspect, s.DisplayAspect),
		})
		applyFormatSideData(&res.Streams[len(res.Streams)-1], s.SideData)
	}
	if err := p.enrichFrames(ctx, path, res.Streams); err != nil {
		return nil, err
	}
	for _, c := range raw.Chapters {
		start, err1 := strconv.ParseFloat(c.StartTime, 64)
		end, err2 := strconv.ParseFloat(c.EndTime, 64)
		if err1 != nil || err2 != nil || end <= start {
			continue
		}
		title := ""
		for k, v := range c.Tags {
			if strings.EqualFold(k, "title") {
				title = strings.TrimSpace(v)
			}
		}
		res.Chapters = append(res.Chapters, Chapter{StartMS: int64(start * 1000), EndMS: int64(end * 1000), Title: title})
	}
	return res, nil
}

// aspectRatio works out a video's display aspect ratio, rounded to two decimals. ffprobe's
// display ratio already allows for non-square pixels; without it, the pixel shape (sample
// ratio) corrects the frame size, so a 720x480 DVD with 32:27 pixels is 16:9 (1.78).
func aspectRatio(kind string, width, height int, sample, display string) float64 {
	if kind != "video" || width <= 0 || height <= 0 {
		return 0
	}
	r := ratio(display)
	if r == 0 {
		r = float64(width) / float64(height)
		if sar := ratio(sample); sar > 0 {
			r *= sar
		}
	}
	return math.Round(r*100) / 100
}

// ratio reads "16:9" as 1.777…; "0:1", "N/A" and nonsense read as 0.
func ratio(s string) float64 {
	a, b, ok := strings.Cut(s, ":")
	if !ok {
		return 0
	}
	n, err1 := strconv.ParseFloat(a, 64)
	d, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil || n <= 0 || d <= 0 {
		return 0
	}
	return n / d
}

var demuxerPrefixRe = regexp.MustCompile(`^\[[^\]]*@ 0x[0-9a-f]+\]\s*`)

// errorMessage reduces ffprobe's error output to its last, most telling line, without the file
// path or internal "[matroska @ 0x...]" prefixes, e.g. "Invalid data found when processing input".
func errorMessage(stderr, path string) string {
	lines := strings.Split(strings.ReplaceAll(stderr, "\r\n", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		line = strings.TrimSpace(strings.TrimPrefix(line, path+":"))
		line = demuxerPrefixRe.ReplaceAllString(line, "")
		if line != "" {
			return line
		}
	}
	return ""
}

// containerName picks a readable name from ffprobe's format list, e.g. "mov,mp4,m4a,..." -> "mp4"
// for a .mp4 file.
func containerName(formatName, path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	names := strings.Split(formatName, ",")
	for _, n := range names {
		if n == ext {
			return n
		}
	}
	return names[0]
}

// bitDepth is the bits per sample of a video stream, from ffprobe's figure or its pixel format
// (e.g. yuv420p10le is 10-bit).
func bitDepth(raw, pixFmt string) int {
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		return n
	}
	switch {
	case pixFmt == "":
		return 0
	case strings.Contains(pixFmt, "12"):
		return 12
	case strings.Contains(pixFmt, "10"), pixFmt == "p010le":
		return 10
	}
	return 8
}

// frameRate turns ffprobe's "24000/1001" into frames per second.
func frameRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	n, err1 := strconv.ParseFloat(num, 64)
	if !ok {
		return n
	}
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}

func parseInt(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

// FFmpegNextTo finds FFmpeg beside the ffprobe in use (they're installed together), or on the
// PATH. It returns "" when there's none.
func FFmpegNextTo(ffprobe string) string {
	if ffprobe != "" {
		name := "ffmpeg"
		if strings.HasSuffix(strings.ToLower(ffprobe), ".exe") {
			name += ".exe"
		}
		p := filepath.Join(filepath.Dir(ffprobe), name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	return ""
}
