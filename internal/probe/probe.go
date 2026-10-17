// Package probe reads container and stream information from media files using ffprobe.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
}

type Stream struct {
	Index     int
	Kind      string
	Codec     string
	Profile   string
	Language  string
	Title     string
	Width     int
	Height    int
	Channels  int
	IsDefault bool
	IsForced  bool
}

type ffprobeOutput struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		Index       int               `json:"index"`
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Profile     string            `json:"profile"`
		Width       int               `json:"width"`
		Height      int               `json:"height"`
		Channels    int               `json:"channels"`
		Tags        map[string]string `json:"tags"`
		Disposition map[string]int    `json:"disposition"`
	} `json:"streams"`
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

func (p *Prober) Probe(ctx context.Context, path string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.Bin(), "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path)
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
			Index:     s.Index,
			Kind:      s.CodecType,
			Codec:     s.CodecName,
			Profile:   s.Profile,
			Language:  tags["language"],
			Title:     tags["title"],
			Width:     s.Width,
			Height:    s.Height,
			Channels:  s.Channels,
			IsDefault: s.Disposition["default"] == 1,
			IsForced:  s.Disposition["forced"] == 1,
		})
	}
	return res, nil
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
