package api

import (
	"ams/internal/store"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

func sourceHDRFormat(v *store.Stream) string {
	if v.DolbyVision {
		return "dolby_vision"
	}
	if v.HDR10Plus {
		return "hdr10_plus"
	}
	switch v.ColorTransfer {
	case "smpte2084":
		return "hdr10"
	case "arib-std-b67":
		return "hlg"
	}
	return ""
}
func (c clientCaps) validate() error {
	if c.MaxBitDepth < 0 || c.MaxBitDepth > 16 || c.MaxAudioChannels < 0 || c.MaxAudioChannels > 32 || c.MaxHeight < 0 || c.MaxHeight > 16384 {
		return fmt.Errorf("invalid player capability limit")
	}
	if len(c.Containers) > 32 || len(c.Video) > 32 || len(c.Audio) > 32 || len(c.AudioPassthrough) > 32 || len(c.VideoProfiles) > 32 || len(c.HDRFormats) > 8 || len(c.DolbyVisionProfiles) > 16 {
		return fmt.Errorf("too many capability values")
	}
	for _, list := range [][]string{c.Containers, c.Video, c.Audio, c.AudioPassthrough} {
		for _, value := range list {
			if !codecFilterPattern.MatchString(value) {
				return fmt.Errorf("invalid capability identifier")
			}
		}
	}
	for codec, profiles := range c.VideoProfiles {
		if !codecFilterPattern.MatchString(codec) || len(profiles) > 32 {
			return fmt.Errorf("invalid video profiles")
		}
		for _, profile := range profiles {
			if len(profile) == 0 || len(profile) > 64 {
				return fmt.Errorf("invalid video profile")
			}
		}
	}
	for _, format := range c.HDRFormats {
		if !slices.Contains([]string{"hdr10", "hlg", "hdr10_plus", "dolby_vision"}, format) {
			return fmt.Errorf("unknown HDR format")
		}
	}
	for _, profile := range c.DolbyVisionProfiles {
		if profile < 1 || profile > 10 {
			return fmt.Errorf("invalid Dolby Vision profile")
		}
	}
	return nil
}
func (s *Server) writePlaybackPreview(w http.ResponseWriter, out playbackPlan, can bool) {
	out.DecisionOnly = true
	out.CanStart = &can
	out.URL = ""
	out.SessionID = ""
	out.HLS = false
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, out)
}

// Converted output is fMP4 HLS; video encoding is 8-bit H.264, audio encoding AAC stereo.
// Do not claim that a custom native capability set can consume a fallback it explicitly excludes.
func conversionSupported(c clientCaps, d decision, audio *store.Stream) bool {
	// An empty list is how players ask for more after a stream failed (web/src/lib/playback.ts
	// fallbackCaps): no containers means "repackage it", no video codecs "convert the video".
	// It says nothing against Arc's fMP4/H.264 output; only a list that leaves those out does.
	if len(c.Containers) > 0 && !c.has(c.Containers, "mp4") {
		return false
	}
	if d.Method == "transcode" && (len(c.Video) > 0 && !c.has(c.Video, "h264") || c.MaxBitDepth > 0 && c.MaxBitDepth < 8) {
		return false
	}
	if d.Method == "transcode" {
		if profiles, limited := c.VideoProfiles["h264"]; limited && !slices.ContainsFunc(profiles, func(p string) bool { return strings.EqualFold(p, "High") }) {
			return false
		}
	}
	if audio != nil && !d.CopyAudio && (!c.has(c.Audio, "aac") || c.MaxAudioChannels > 0 && c.MaxAudioChannels < 2) {
		return false
	}
	return true
}
