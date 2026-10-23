package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strconv"
	"time"
)

// These are structured FFprobe values, never filename/track-title guesses.
type formatSideData struct {
	Type    string `json:"side_data_type"`
	Profile int    `json:"dv_profile"`
	RPU     int    `json:"rpu_present_flag"`
}

func applyFormatSideData(s *Stream, data []formatSideData) {
	for _, d := range data {
		switch d.Type {
		case "DOVI configuration record":
			if d.RPU == 1 {
				s.DolbyVision = true
				s.DolbyVisionProfile = d.Profile
			}
		case "Dolby Vision RPU Data", "Dolby Vision Metadata":
			s.DolbyVision = true
		case "HDR Dynamic Metadata SMPTE2094-40 (HDR10+)", "HDR10+ Dynamic Metadata (SMPTE 2094-40)":
			s.HDR10Plus = true
		}
	}
	if s.DolbyVision || s.HDR10Plus {
		s.HDRProbeStatus = "stream"
	}
}

// limitedOutput also bounds malformed/noisy subprocess output. The child has a wall timeout.
type limitedOutput struct {
	buffer    bytes.Buffer
	remaining int
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	if len(p) > b.remaining {
		return 0, errors.New("format probe output limit")
	}
	b.remaining -= len(p)
	return b.buffer.Write(p)
}

func (b *limitedOutput) Bytes() []byte { return b.buffer.Bytes() }
func (b *limitedOutput) Len() int      { return b.buffer.Len() }

// EnrichFormats samples at most 64 packets per candidate video stream, at most four streams,
// with a shared four-second deadline. A negative sample is not whole-file absence evidence.
// Optional-probe failures retain usable basic metadata and are remembered, not retried on
// every unchanged scan. A changed file gets a fresh attempt. Parent cancellation propagates.
func (p *Prober) enrichFrames(ctx context.Context, path string, streams []Stream) error {
	sampleCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	attempts := 0
	for i := range streams {
		s := &streams[i]
		if s.Kind != "video" || (s.Codec != "hevc" && s.Codec != "h265" && s.Codec != "av1" && s.ColorTransfer != "smpte2084" && s.ColorTransfer != "arib-std-b67") {
			continue
		}
		known := s.DolbyVision || s.HDR10Plus
		if !known {
			s.HDRProbeStatus = "unavailable"
		}
		if attempts >= 4 {
			if !known {
				s.HDRProbeStatus = "unavailable"
			}
			continue
		}
		attempts++
		out := &limitedOutput{remaining: 2 << 20}
		stderr := &limitedOutput{remaining: 64 << 10}
		cmd := exec.CommandContext(sampleCtx, p.Bin(), "-v", "error", "-threads", "1", "-probesize", "5000000", "-analyzeduration", "2000000",
			"-select_streams", strconv.Itoa(s.Index), "-read_intervals", "%+#64", "-show_frames",
			"-show_entries", "frame=stream_index:frame_side_data=side_data_type", "-of", "json", path)
		cmd.Stdout = out
		cmd.Stderr = stderr
		err := cmd.Run()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		var raw struct {
			Frames []struct {
				Index int              `json:"stream_index"`
				Data  []formatSideData `json:"side_data_list"`
			} `json:"frames"`
		}
		if err != nil || json.Unmarshal(out.Bytes(), &raw) != nil || len(raw.Frames) == 0 {
			if !known {
				s.HDRProbeStatus = "unavailable"
			}
			continue
		}
		matched := false
		for _, f := range raw.Frames {
			if f.Index == s.Index {
				matched = true
				applyFormatSideData(s, f.Data)
			}
		}
		if matched {
			s.HDRProbeStatus = "sampled"
		} else if !known {
			s.HDRProbeStatus = "unavailable"
		}
	}
	return ctx.Err()
}

// EnrichFormats is the existing-file path: refresh compact structured stream evidence
// (including declared DV profiles and audio profiles/layouts), then sample frames. Both
// subprocess stages share one four-second optional budget; basic metadata is untouched.
func (p *Prober) EnrichFormats(ctx context.Context, path string, streams []Stream) error {
	budget, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	out := &limitedOutput{remaining: 2 << 20}
	stderr := &limitedOutput{remaining: 64 << 10}
	cmd := exec.CommandContext(budget, p.Bin(), "-v", "error", "-probesize", "5000000", "-analyzeduration", "2000000",
		"-show_streams", "-show_entries", "stream=index,codec_type,codec_name,profile,channel_layout:stream_side_data=side_data_type,dv_profile,rpu_present_flag", "-of", "json", path)
	cmd.Stdout = out
	cmd.Stderr = stderr
	var raw ffprobeOutput
	if err := cmd.Run(); err == nil && json.Unmarshal(out.Bytes(), &raw) == nil {
		for i := range streams {
			s := &streams[i]
			for _, r := range raw.Streams {
				if r.Index != s.Index || r.CodecType != s.Kind || r.CodecName != s.Codec {
					continue
				}
				if r.Profile != "" {
					s.Profile = r.Profile
				}
				if r.ChannelLayout != "" {
					s.ChannelLayout = r.ChannelLayout
				}
				applyFormatSideData(s, r.SideData)
			}
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// A local optional deadline must not turn a previously readable file into a probe error.
	_ = p.enrichFrames(budget, path, streams)
	return ctx.Err()
}
