// Package probe reads container and stream information from media files using ffprobe.
package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Prober struct {
	bin string
}

func New(bin string) *Prober {
	if bin == "" {
		bin = "ffprobe"
	}
	return &Prober{bin: bin}
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
	out, err := exec.CommandContext(ctx, p.bin, "-version").Output()
	if err != nil {
		return "", err
	}
	line, _, _ := strings.Cut(string(out), "\n")
	return strings.TrimSpace(line), nil
}

func (p *Prober) Probe(ctx context.Context, path string) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.bin, "-v", "error", "-print_format", "json",
		"-show_format", "-show_streams", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("ffprobe: %s", msg)
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
