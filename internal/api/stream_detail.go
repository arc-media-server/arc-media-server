package api

import (
	"fmt"
	"strings"

	"ams/internal/store"
	"ams/internal/transcode"
)

// streamDetail is how a playback is being sent, for the Dashboard (as Plex's "stream details"):
// why it isn't played as it is, the video and audio in the file and what the player gets, and
// what does any converting (a named graphics card, or the CPU).
type streamDetail struct {
	Reasons []string `json:"reasons,omitempty"`
	// The file's video and audio, e.g. "HEVC 4K Dolby Vision" and "Dolby TrueHD 7.1".
	Video string `json:"video,omitempty"`
	Audio string `json:"audio,omitempty"`
	// What the player gets when it differs from the file ("" when sent as it is).
	VideoOutput string `json:"video_output,omitempty"`
	AudioOutput string `json:"audio_output,omitempty"`
	// Converting the video: the graphics card ("NVIDIA NVENC"), or "CPU"; "" when nothing is encoded.
	Encoder string `json:"encoder,omitempty"`
	// Whether the encoder is a graphics card (hardware conversion).
	Hardware bool `json:"hardware,omitempty"`
	// HDR is mapped to SDR for a screen without it.
	ToneMapped bool `json:"tone_mapped,omitempty"`
	// Picture subtitles are drawn into the video.
	SubtitlesBurned bool `json:"subtitles_burned,omitempty"`
}

// heightName names a picture height as people do: 4K, 1080p, 720p…
func heightName(h int) string {
	switch {
	case h >= 2000:
		return "4K"
	case h >= 1000:
		return "1080p"
	case h >= 700:
		return "720p"
	case h > 0:
		return "SD"
	}
	return ""
}

func channelsName(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "Mono"
	case 2:
		return "Stereo"
	case 6:
		return "5.1"
	case 8:
		return "7.1"
	}
	return fmt.Sprintf("%d ch", n)
}

func joinNonEmpty(parts ...string) string {
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

// videoName reads a video stream as e.g. "HEVC 4K Dolby Vision" or "H.264 1080p".
func videoName(v *store.Stream) string {
	if v == nil {
		return ""
	}
	hdr := hdrName(v)
	switch {
	case v.DolbyVision:
		hdr = "Dolby Vision"
	case v.HDR10Plus:
		hdr = "HDR10+"
	}
	return joinNonEmpty(codecName(v.Codec), heightName(v.Height), hdr)
}

func audioName(codec string, channels int) string {
	if codec == "" {
		return ""
	}
	return joinNonEmpty(codecName(codec), channelsName(channels))
}

// directDetail is a file played as it is.
func directDetail(video, audio *store.Stream, reasons []string) streamDetail {
	d := streamDetail{Reasons: reasons, Video: videoName(video)}
	if audio != nil {
		d.Audio = audioName(audio.Codec, audio.Channels)
	}
	return d
}

// conversionDetail is a repackaged or converted stream, from what the conversion was asked to make.
func conversionDetail(video, audio *store.Stream, reasons []string, o transcode.Options) streamDetail {
	d := directDetail(video, audio, reasons)
	if !o.CopyVideo {
		h := 0
		if video != nil {
			h = video.Height
		}
		if o.MaxHeight > 0 && (h == 0 || o.MaxHeight < h) {
			h = o.MaxHeight
		}
		d.VideoOutput = joinNonEmpty("H.264", heightName(h))
		d.Encoder = transcode.EncoderLabel(o.HWEncoder)
		d.Hardware = d.Encoder != ""
		if !d.Hardware {
			d.Encoder = "CPU"
		}
		d.ToneMapped = o.ToneMap
		d.SubtitlesBurned = o.BurnSubtitle >= 0
	}
	if audio != nil && !o.CopyAudio {
		switch {
		case o.AudioCodec != "" && o.AudioChannels > 2:
			d.AudioOutput = audioName(o.AudioCodec, o.AudioChannels)
		default:
			d.AudioOutput = audioName("aac", min(audio.Channels, 2))
		}
	}
	return d
}
