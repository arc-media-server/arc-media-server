package api

import (
	"ams/internal/auth"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"ams/internal/store"
	"ams/internal/subtitles"
	"ams/internal/transcode"
	"ams/internal/trickplay"
)

// How a file reaches the player:
//   - direct: the file as it is, with range requests for seeking;
//   - remux: the video copied as it is into a format the player can open, with the audio
//     converted if needed (quick, nothing lost);
//   - transcode: the video re-encoded to H.264 (slow, uses the CPU).

// clientCaps is what the player can play, as it reports it.
type clientCaps struct {
	// Omitted limits preserve legacy browser defaults. HDRFormats nil means unspecified;
	// an explicit empty array means SDR only. Passthrough is usable only for direct files.
	MaxBitDepth         int                 `json:"max_bit_depth,omitempty"`
	MaxAudioChannels    int                 `json:"max_audio_channels,omitempty"`
	HDRFormats          []string            `json:"hdr_formats" api:"optional"`
	DolbyVisionProfiles []int               `json:"dolby_vision_profiles" api:"optional"`
	VideoProfiles       map[string][]string `json:"video_profiles,omitempty"`
	AudioPassthrough    []string            `json:"audio_passthrough,omitempty"`

	Containers []string `json:"containers"` // mp4, webm, mkv
	Video      []string `json:"video"`      // h264, hevc, vp9, av1
	Audio      []string `json:"audio"`      // aac, mp3, opus, flac, ac3, eac3, vorbis
	// The largest picture worth sending (e.g. the screen's height); 0 for no limit.
	MaxHeight int `json:"max_height" api:"optional"`
	// Set on a retry: the method the player was given and couldn't play ("direct" or "remux").
	// The caps are narrowed for the retry, so the reasons say this instead of blaming the file.
	FailedMethod string `json:"failed_method,omitempty"`
}

func (c clientCaps) has(list []string, v string) bool { return slices.Contains(list, v) }

type decision struct {
	Method    string
	CopyAudio bool
	// When the audio is converted: "eac3" or "ac3" to keep surround sound for a player that
	// decodes it ("" for AAC stereo), and how many channels.
	AudioCodec    string
	AudioChannels int
	Reasons       []string
	ReasonCodes   []string
	NeedsToneMap  bool
	UnsupportedDV bool
}

// containerFamily maps ffprobe's container name to the names players report.
func containerFamily(c string) string {
	switch c {
	case "mp4", "mov", "m4v", "m4a", "3gp":
		return "mp4"
	case "matroska":
		return "mkv"
	}
	return c
}

var codecNames = map[string]string{"h264": "H.264", "hevc": "HEVC", "av1": "AV1", "vp9": "VP9", "mpeg4": "MPEG-4",
	"mpeg2video": "MPEG-2", "vc1": "VC-1", "aac": "AAC", "ac3": "Dolby Digital", "eac3": "Dolby Digital Plus",
	"truehd": "Dolby TrueHD", "dts": "DTS", "flac": "FLAC", "mp3": "MP3", "opus": "Opus", "vorbis": "Vorbis"}

func codecName(c string) string {
	if n, ok := codecNames[c]; ok {
		return n
	}
	return strings.ToUpper(c)
}

// videoStream is a file's main video stream (not cover art), or nil.
func videoStream(f *store.MediaFile) *store.Stream {
	for i := range f.Streams {
		st := &f.Streams[i]
		if st.Kind == "video" && st.Codec != "mjpeg" && st.Codec != "png" && st.Codec != "bmp" {
			return st
		}
	}
	return nil
}

// defaultAudio is the audio stream a player picks by itself: the one marked default, or the first.
func defaultAudio(f *store.MediaFile) *store.Stream {
	var first *store.Stream
	for i := range f.Streams {
		st := &f.Streams[i]
		if st.Kind != "audio" {
			continue
		}
		if st.IsDefault {
			return st
		}
		if first == nil {
			first = st
		}
	}
	return first
}

// qualityHeight is the picture height a quality limit (kbit/s) is converted to, as Plex's steps.
// qualityLimits are the quality limits a player or profile can choose (kbit/s), the same list as
// `qualities` in web/src/lib/playback.ts.
var qualityLimits = []int{20000, 12000, 8000, 4000, 3000, 2000, 1500, 720}

func qualityHeight(kbps int) int {
	switch {
	case kbps >= 8000:
		return 1080
	case kbps >= 2000:
		return 720
	case kbps >= 1500:
		return 480
	}
	return 360
}

// qualityName is how a quality limit reads, e.g. "4 Mbps 720p" or "720 kbps 360p".
func qualityName(kbps int) string {
	rate := fmt.Sprintf("%d kbps", kbps)
	if kbps >= 1000 {
		rate = strconv.FormatFloat(float64(kbps)/1000, 'f', -1, 64) + " Mbps"
	}
	return fmt.Sprintf("%s %dp", rate, qualityHeight(kbps))
}

// preferredAudio is the audio stream in a language (the profile's preferred one), or nil if the
// file has none: the one marked default among them, or the first.
func preferredAudio(f *store.MediaFile, language string) *store.Stream {
	var first *store.Stream
	for i := range f.Streams {
		st := &f.Streams[i]
		if st.Kind != "audio" || !subtitles.SameLanguage(st.Language, language) {
			continue
		}
		if st.IsDefault {
			return st
		}
		if first == nil {
			first = st
		}
	}
	return first
}

// decide works out how a file can be played with the chosen audio stream (nil for none).
func decide(f *store.MediaFile, video, audio *store.Stream, caps clientCaps) decision {
	d := decision{Method: "direct"}
	videoOK := video != nil && caps.has(caps.Video, video.Codec) &&
		// Browsers can't decode 10-bit H.264 (Hi10P), though they say they play H.264.
		!(video.Codec == "h264" && video.BitDepth > 8 && caps.MaxBitDepth < video.BitDepth)
	switch {
	case video == nil:
		d.Reasons = append(d.Reasons, "no video found")
		d.ReasonCodes = append(d.ReasonCodes, "no_video")
	case !videoOK && video.Codec == "h264" && video.BitDepth > 8 && caps.has(caps.Video, "h264"):
		// Only when the bit depth is the reason: a player that doesn't list H.264 at all gets
		// "H.264 video", as for any other codec it lacks.
		d.Reasons = append(d.Reasons, "10-bit H.264 video")
		d.ReasonCodes = append(d.ReasonCodes, "video_codec")
	case !videoOK:
		d.Reasons = append(d.Reasons, codecName(video.Codec)+" video")
		d.ReasonCodes = append(d.ReasonCodes, "video_codec")
	}
	if video != nil {
		reject := func(code, reason string) {
			videoOK = false
			d.ReasonCodes = append(d.ReasonCodes, code)
			d.Reasons = append(d.Reasons, reason)
		}
		if caps.MaxBitDepth > 0 && video.BitDepth > caps.MaxBitDepth {
			reject("video_bit_depth", "video bit depth exceeds player limit")
		}
		if caps.MaxHeight > 0 && max(video.Height, video.Width*9/16) > caps.MaxHeight*11/10 {
			reject("video_height", "video exceeds player height limit")
		}
		if profiles, limited := caps.VideoProfiles[video.Codec]; limited && !slices.ContainsFunc(profiles, func(p string) bool { return strings.EqualFold(p, video.Profile) }) {
			reject("video_profile", "video profile not supported")
		}
		format := sourceHDRFormat(video)
		if caps.HDRFormats != nil && format != "" {
			supported := slices.Contains(caps.HDRFormats, format) || format == "hdr10_plus" && slices.Contains(caps.HDRFormats, "hdr10")
			if format == "dolby_vision" && caps.DolbyVisionProfiles != nil && !slices.Contains(caps.DolbyVisionProfiles, video.DolbyVisionProfile) {
				supported = false
			}
			if !supported {
				reject("hdr_output", "source HDR format not supported")
				d.NeedsToneMap = true
				d.UnsupportedDV = format == "dolby_vision"
			}
		}
	}
	if fam := containerFamily(f.Container); !caps.has(caps.Containers, fam) {
		d.Reasons = append(d.Reasons, strings.ToUpper(fam)+" file")
		d.ReasonCodes = append(d.ReasonCodes, "container")
	}
	audioOK := audio == nil || (caps.has(caps.Audio, audio.Codec) || caps.has(caps.AudioPassthrough, audio.Codec))
	if audio != nil && caps.MaxAudioChannels > 0 && audio.Channels > caps.MaxAudioChannels {
		audioOK = false
		d.Reasons = append(d.Reasons, "audio exceeds player channel limit")
		d.ReasonCodes = append(d.ReasonCodes, "audio_channels")
	}
	if !audioOK {
		d.Reasons = append(d.Reasons, codecName(audio.Codec)+" audio")
		d.ReasonCodes = append(d.ReasonCodes, "audio_codec")
	}
	if audio == nil && defaultAudio(f) != nil {
		d.Reasons = append(d.Reasons, "audio disabled")
		d.ReasonCodes = append(d.ReasonCodes, "audio_disabled")
	}
	if audio != nil && audio != defaultAudio(f) {
		d.Reasons = append(d.Reasons, "another audio track chosen")
		d.ReasonCodes = append(d.ReasonCodes, "audio_selection")
	}
	// Copied audio must be something the player decodes in a converted stream: AAC in stereo
	// or less (every HLS player), or Dolby Digital (Plus) when the player says it decodes that.
	dolby := audio != nil && (audio.Codec == "ac3" || audio.Codec == "eac3") && caps.has(caps.Audio, audio.Codec)
	d.CopyAudio = audio != nil && audioOK && (audio.Codec == "aac" && audio.Channels <= 2 || dolby)
	// Surround sound the player can't take as it is (TrueHD, DTS) becomes Dolby Digital Plus 5.1
	// (or Dolby Digital) when the player decodes that, as Plex does, rather than AAC stereo.
	if audio != nil && !d.CopyAudio && audio.Channels > 2 {
		channels := min(audio.Channels, 6)
		if caps.MaxAudioChannels > 0 {
			channels = min(channels, caps.MaxAudioChannels)
		}
		switch {
		case channels <= 2:
		case caps.has(caps.Audio, "eac3"):
			d.AudioCodec, d.AudioChannels = "eac3", channels
		case caps.has(caps.Audio, "ac3"):
			d.AudioCodec, d.AudioChannels = "ac3", channels
		}
	}
	switch {
	case len(d.Reasons) == 0:
		d.Method = "direct"
	case videoOK:
		d.Method = "remux"
	default:
		d.Method = "transcode"
	}
	if failed, ok := map[string]string{"direct": "the player couldn't play the file as it is",
		"remux": "the player couldn't play the repackaged stream"}[caps.FailedMethod]; ok && d.Method != "direct" {
		d.Reasons, d.ReasonCodes = []string{failed}, []string{"player_failed"}
	}
	return d
}

type audioTrackDTO struct {
	Index         int    `json:"index"`
	Language      string `json:"language,omitempty"`
	Title         string `json:"title,omitempty"`
	Codec         string `json:"codec"`
	Channels      int    `json:"channels,omitempty"`
	Profile       string `json:"profile,omitempty"`
	ChannelLayout string `json:"channel_layout,omitempty"`
	Default       bool   `json:"default"`
}

// Optimized playback retains source indices for track choices while identifying the audio
// actually encoded in the served MP4 (video 0, audio 1). It is not source codec metadata.
type optimizedPlaybackDTO struct {
	DurationMS        int64          `json:"duration_ms,omitempty"`
	Bitrate           int64          `json:"bitrate,omitempty"`
	Quality           int            `json:"quality"`
	SourceAudioStream int            `json:"source_audio_stream"`
	Audio             *audioTrackDTO `json:"audio,omitempty"`
}

// audioOutputDTO is the audio as the player receives it, for the player's stats and the
// Dashboard: the codec (as ffprobe names it), channels, and whether Arc converted it.
type audioOutputDTO struct {
	Codec     string `json:"codec"`
	Channels  int    `json:"channels"`
	Converted bool   `json:"converted"`
}

type playbackPlan struct {
	DecisionOnly  bool     `json:"decision_only,omitempty"`
	CanStart      *bool    `json:"can_start,omitempty"`
	NeedsAnalysis bool     `json:"needs_analysis,omitempty"`
	ReasonCodes   []string `json:"reason_codes,omitempty"`

	Optimized *optimizedPlaybackDTO `json:"optimized,omitempty"`
	Method    string                `json:"method"` // direct, remux or transcode
	Reasons   []string              `json:"reasons"`
	URL       string                `json:"url"`
	// The URL is an HLS playlist (play it with hls.js, or natively in Safari).
	HLS       bool   `json:"hls"`
	SessionID string `json:"session_id,omitempty"`
	// Where the stream starts in the file; add it to the player's clock.
	OffsetMS    int64           `json:"offset_ms"`
	DurationMS  int64           `json:"duration_ms"`
	FileID      int64           `json:"file_id"`
	AudioStream int             `json:"audio_stream"` // -1 for none
	AudioTracks []audioTrackDTO `json:"audio_tracks"`
	// The audio as sent (none when there's no audio track).
	AudioOutput *audioOutputDTO `json:"audio_output,omitempty" api:"optional"`
	// Subtitles: text ones are fetched as cues and drawn by the player; picture ones are burned
	// in by asking again with "burn_subtitle".
	SubtitleTracks []subtitles.Track `json:"subtitle_tracks"`
	// When converting: the graphics card doing it ("NVIDIA NVENC"; "" for the CPU), and whether
	// HDR is being mapped to SDR.
	Encoder    string `json:"encoder,omitempty"`
	ToneMapped bool   `json:"tone_mapped,omitempty"`
	// The title's files to choose from (editions and versions), the one playing among them.
	Versions     []versionDTO `json:"versions"`
	BurnSubtitle string       `json:"burn_subtitle,omitempty"`
	// The file's overall bitrate and picture height, so the player offers only lower qualities.
	SourceKbps   int `json:"source_kbps"`
	SourceHeight int `json:"source_height"`
	// The ceiling away from home (the server's remote limits and the profile's internet streaming
	// quality, whichever is lowest; 0 at home), not the player's current requested quality.
	ServerLimitKbps int `json:"server_limit_kbps"`
	// The file's chapter marks (times in the file).
	Chapters []store.Chapter `json:"chapters"`
	// Where the intro, recap and credits are, for Skip Intro and Skip Credits (times in the file).
	Markers []store.Marker `json:"markers"`
}

// startPlayback: POST /api/items/{id}/playback {"caps": {...}, "file_id", "audio_stream",
// "start_ms"} decides how to play a movie or episode on this player and, unless it can play the
// file as it is, starts converting it. Starting again (e.g. to seek or change audio track) ends
// the player's previous conversion.
func (s *Server) startPlayback(w http.ResponseWriter, r *http.Request) { s.servePlayback(w, r, false) }

// playbackPreview: POST /api/items/{id}/playback-preview accepts the playback request and
// returns its decision without probing, starting a stream/conversion, changing playback state
// or preferences. No URL/session is allocated. Unanalysed files return needs_analysis=true.
func (s *Server) playbackPreview(w http.ResponseWriter, r *http.Request) { s.servePlayback(w, r, true) }

func (s *Server) servePlayback(w http.ResponseWriter, r *http.Request, preview bool) {
	if preview {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Caps        clientCaps `json:"caps"`
		FileID      int64      `json:"file_id" api:"optional"`
		AudioStream *int       `json:"audio_stream" api:"optional"`
		StartMS     int64      `json:"start_ms" api:"optional"`
		// A picture subtitle track to burn into the video (which means converting it).
		BurnSubtitle string `json:"burn_subtitle" api:"optional"`
		// A quality limit chosen in the player, in kbit/s (e.g. 4000 for "4 Mbps 720p"); 0 for the
		// original.
		MaxKbps int `json:"max_kbps" api:"optional"`
		// Without file_id, play this edition (e.g. "Director's Cut"; "" for the standard one).
		Edition string `json:"edition" api:"optional"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&body); err != nil || body.StartMS < 0 {
		writeError(w, http.StatusBadRequest, `send {"caps": {...}} and optionally "file_id", "audio_stream", "start_ms"`)
		return
	}
	if err := body.Caps.validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	if body.FileID < 0 || body.MaxKbps < 0 || body.MaxKbps > 200000 || body.AudioStream != nil && *body.AudioStream < -1 {
		writeError(w, 400, "invalid file_id or max_kbps")
		return
	}
	files, f := s.selectPlaybackFile(w, r, it.ID, body.FileID, body.Edition, body.Caps, !preview)
	if f == nil {
		return
	}
	if why := s.playing.refusalFor(r, it.ID); why != "" {
		writeError(w, http.StatusForbidden, why)
		return
	}

	if preview && (!f.Probed || len(f.Streams) == 0) {
		can := false
		writeJSON(w, 200, playbackPlan{Method: "unknown", DecisionOnly: true, CanStart: &can, NeedsAnalysis: true, FileID: f.ID, Reasons: []string{"file needs analysis"}, ReasonCodes: []string{"needs_analysis"}, AudioTracks: []audioTrackDTO{}, SubtitleTracks: []subtitles.Track{}, Versions: versions(files)})
		return
	}
	video := videoStream(f)
	if video == nil {
		if preview {
			can := false
			writeJSON(w, 200, playbackPlan{Method: "unavailable", DecisionOnly: true, CanStart: &can, FileID: f.ID, Reasons: []string{"no video found"}, ReasonCodes: []string{"no_video"}})
		} else {
			writeError(w, 422, "no video found")
		}
		return
	}
	var audio *store.Stream
	if body.AudioStream == nil {
		audio = s.playbackDefaultAudio(r, f)
	} else {
		audio = nil
		for i := range f.Streams {
			if f.Streams[i].Kind == "audio" && f.Streams[i].Index == *body.AudioStream {
				audio = &f.Streams[i]
			}
		}
		if audio == nil && *body.AudioStream >= 0 {
			writeError(w, http.StatusBadRequest, "that audio track isn't in the file")
			return
		}
	}
	d := decide(f, video, audio, body.Caps)
	burn := -1
	if body.BurnSubtitle != "" {
		t, ok := s.findSubtitle(f, body.BurnSubtitle)
		if !ok || !t.Image || t.External {
			writeError(w, http.StatusBadRequest, "only picture subtitles inside the file are burned in")
			return
		}
		burn = t.StreamIndex
		d.Method = "transcode"
		d.Reasons = append(d.Reasons, "picture subtitles shown")
		d.ReasonCodes = append(d.ReasonCodes, "subtitle_burn")
	}
	// Away from home, streams keep within the remote limits (Settings > Remote Access).
	serverLimit := s.remoteLimit(r, it.ID)
	if limit := serverLimit; limit > 0 && (body.MaxKbps == 0 || body.MaxKbps > limit) {
		body.MaxKbps = limit
	}
	limitHeight := 0
	if body.MaxKbps > 0 && video != nil {
		limitHeight = qualityHeight(body.MaxKbps)
		if f.Bitrate == 0 || f.Bitrate > int64(body.MaxKbps)*1000*11/10 || video.Height > limitHeight*11/10 {
			d.Method = "transcode"
			d.Reasons = append(d.Reasons, fmt.Sprintf("quality set to %s", qualityName(body.MaxKbps)))
			d.ReasonCodes = append(d.ReasonCodes, "quality_limit")
		} else {
			limitHeight = 0 // the file is already within the limit
		}
	}

	out := playbackPlan{Method: d.Method, Reasons: d.Reasons, ReasonCodes: d.ReasonCodes, DurationMS: f.DurationMS, FileID: f.ID,
		AudioStream: -1, AudioTracks: []audioTrackDTO{}, SubtitleTracks: s.subtitleTracks(f), BurnSubtitle: body.BurnSubtitle,
		Versions: versions(files), SourceKbps: int(f.Bitrate / 1000), ServerLimitKbps: serverLimit}
	if video != nil {
		out.SourceHeight = video.Height
	}
	var err error
	if out.Chapters, err = s.store.FileChapters(r.Context(), f.ID); err != nil {
		s.fail(w, err)
		return
	}
	if out.Markers, err = s.playableMarkers(r, it.LibraryID, f.ID); err != nil {
		s.fail(w, err)
		return
	}
	if out.Reasons == nil {
		out.Reasons = []string{}
	}
	if audio != nil {
		out.AudioStream = audio.Index
		out.AudioOutput = &audioOutputDTO{Codec: audio.Codec, Channels: audio.Channels}
	}
	out.AudioTracks = audioTracks(f)

	// A file this player can't play as it is may have an optimized copy it can (with the file's
	// selected audio and no subtitles burned in).
	if d.Method != "direct" && body.BurnSubtitle == "" {
		maxHeight := 0
		if body.MaxKbps > 0 {
			maxHeight = qualityHeight(body.MaxKbps)
		}
		if body.Caps.MaxHeight > 0 && (maxHeight == 0 || body.Caps.MaxHeight < maxHeight) {
			maxHeight = body.Caps.MaxHeight
		}
		if o := s.optimizedFor(r, f, body.Caps, maxHeight, body.MaxKbps, audio); o != nil {
			out.Method, out.Reasons = "direct", []string{fmt.Sprintf("the optimized %dp version", o.Quality)}
			out.URL, out.OffsetMS = optimizedURL(it.ID, o), 0
			out.Optimized = &optimizedPlaybackDTO{Quality: o.Quality, SourceAudioStream: o.AudioStream, DurationMS: o.DurationMS, Bitrate: o.Bitrate}
			out.AudioOutput = nil // the copy's own audio, if it has any
			if o.AudioStream >= 0 {
				out.Optimized.Audio = &audioTrackDTO{Index: 1, Language: o.AudioLanguage, Title: o.AudioTitle, Codec: "aac", Channels: 2, Default: true}
				out.AudioOutput = &audioOutputDTO{Codec: "aac", Channels: 2, Converted: true}
			}
			out.ReasonCodes = []string{"optimized_copy"}
			if preview {
				s.writePlaybackPreview(w, out, true)
				return
			}
			optimized := streamDetail{Reasons: out.Reasons, Video: videoName(video), VideoOutput: fmt.Sprintf("H.264 %dp (optimized version)", o.Quality)}
			if audio != nil {
				optimized.Audio = audioName(audio.Codec, audio.Channels)
				optimized.AudioOutput = audioName("aac", 2)
			}
			s.playing.setMethod(r, it.ID, "direct", "", optimized, s.transcoder)
			s.logPlayback(r, f.ID, out.Method, out.Reasons)
			writeJSON(w, http.StatusOK, out)
			return
		}
	}
	if d.NeedsToneMap && (d.UnsupportedDV || s.transcoder == nil || !s.settings().ToneMap || !s.transcoder.Caps().ToneMapping) {
		out.ReasonCodes = append(out.ReasonCodes, "hdr_conversion_unavailable")
		if preview {
			s.writePlaybackPreview(w, out, false)
		} else {
			writeError(w, 422, "source HDR format cannot be safely converted for this player's reported capabilities")
		}
		return
	}
	if preview && d.Method == "direct" {
		s.writePlaybackPreview(w, out, true)
		return
	}
	if !preview {
		s.logPlayback(r, f.ID, d.Method, d.Reasons)
		if d.Method == "transcode" && s.log.Enabled(r.Context(), slog.LevelDebug) {
			// What the player said it plays, so a conversion caused by a browser's answer can be
			// told from one caused by the file (detailed logging only).
			s.log.Debug("playback caps", "file", f.ID, "containers", strings.Join(body.Caps.Containers, ","),
				"video", strings.Join(body.Caps.Video, ","), "audio", strings.Join(body.Caps.Audio, ","),
				"max_height", body.Caps.MaxHeight, "max_bit_depth", body.Caps.MaxBitDepth)
		}
	}
	if d.Method == "direct" {
		out.URL = fmt.Sprintf("/api/items/%s/stream?file=%d", it.ID, f.ID)
		out.OffsetMS = 0
		s.playing.setMethod(r, it.ID, "direct", "", directDetail(video, audio, nil), s.transcoder)
		writeJSON(w, http.StatusOK, out)
		return
	}
	if !conversionSupported(body.Caps, d, audio) {
		out.ReasonCodes = append(out.ReasonCodes, "conversion_output_unsupported")
		if preview {
			s.writePlaybackPreview(w, out, false)
		} else {
			writeError(w, 422, "player cannot play this server's conversion output")
		}
		return
	}
	if s.transcoder == nil {
		if preview {
			out.ReasonCodes = append(out.ReasonCodes, "conversion_unavailable")
			s.writePlaybackPreview(w, out, false)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "this server can't convert files")
		return
	}
	opts := transcode.Options{Path: f.Path, StartMS: body.StartMS, VideoStream: video.Index, AudioStream: -1,
		CopyVideo: d.Method == "remux", CopyAudio: d.CopyAudio, AudioCodec: d.AudioCodec, AudioChannels: d.AudioChannels,
		MaxHeight: body.Caps.MaxHeight, SourceHeight: video.Height,
		VideoCodec: video.Codec, DolbyVision: video.DolbyVision, BurnSubtitle: burn,
		Owner: currentSession(r), ItemID: it.ID}
	if audio != nil {
		opts.AudioStream = audio.Index
	}
	if limitHeight > 0 {
		// Leave room for the audio (AAC stereo: a limited stream doesn't spend 640 kbit/s on
		// surround) within the limit.
		opts.VideoKbps = max(body.MaxKbps-160, 300)
		opts.AudioCodec, opts.AudioChannels = "", 0
		if opts.MaxHeight == 0 || limitHeight < opts.MaxHeight {
			opts.MaxHeight = limitHeight
		}
	}
	if audio != nil && !opts.CopyAudio {
		out.AudioOutput = &audioOutputDTO{Codec: "aac", Channels: min(audio.Channels, 2), Converted: true}
		if opts.AudioCodec != "" {
			out.AudioOutput.Codec, out.AudioOutput.Channels = opts.AudioCodec, opts.AudioChannels
		}
	}
	if d.Method == "transcode" {
		caps, st := s.transcoder.Caps(), s.settings()
		if st.HWAccel {
			opts.HWEncoder = caps.HWEncoder
		}
		opts.ToneMap = st.ToneMap && caps.ToneMapping && hdrName(video) != ""
		opts.FastToneMap = opts.ToneMap && caps.FastToneMap
	}
	if preview {
		out.Encoder, out.ToneMapped = transcode.EncoderLabel(opts.HWEncoder), opts.ToneMap
		s.writePlaybackPreview(w, out, true)
		return
	}
	sess, err := s.startConversion(r, opts)
	if errors.Is(err, transcode.ErrBusy) {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if err != nil && opts.HWEncoder != "" {
		// The graphics card couldn't do this one (an unusual format, a busy or old driver): use the CPU.
		s.log.Warn("hardware conversion failed; using the CPU", "encoder", opts.HWEncoder, "err", err)
		opts.HWEncoder = ""
		sess, err = s.startConversion(r, opts)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.Encoder, out.ToneMapped = transcode.EncoderLabel(opts.HWEncoder), opts.ToneMap
	s.playing.setMethod(r, it.ID, d.Method, sess.ID, conversionDetail(video, audio, d.Reasons, opts), s.transcoder)
	out.HLS, out.SessionID, out.OffsetMS = true, sess.ID, sess.OffsetMS
	out.URL = "/api/transcode/" + sess.ID + "/index.m3u8"
	writeJSON(w, http.StatusOK, out)
}

// session returns the transcode session in the path if it belongs to this browser.
func (s *Server) transcodeSession(w http.ResponseWriter, r *http.Request) *transcode.Session {
	if s.transcoder == nil {
		writeError(w, http.StatusNotFound, "no such session")
		return nil
	}
	sess := s.transcoder.Get(r.PathValue("sid"))
	if sess == nil || sess.Options.Owner != currentSession(r) {
		writeError(w, http.StatusNotFound, "this playback has ended; start it again")
		return nil
	}
	return sess
}

// transcodeFile: GET /api/transcode/{sid}/{file} serves a session's playlist or segments.
func (s *Server) transcodeFile(w http.ResponseWriter, r *http.Request) {
	sess := s.transcodeSession(w, r)
	if sess == nil {
		return
	}
	// Counts as streaming for the dashboard, and stops when the admin stops it.
	_, done, refused := s.playing.openStream(r, sess.Options.ItemID, "")
	if refused != "" {
		writeError(w, http.StatusForbidden, refused)
		return
	}
	defer done()
	w.Header().Set("Cache-Control", "no-store")
	name := r.PathValue("file")
	if name == "index.m3u8" {
		pl, err := sess.Playlist(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write(pl)
		return
	}
	sess.Requested(name)
	path, ok := sess.File(name)
	if !ok {
		writeError(w, http.StatusNotFound, "not ready")
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, path)
}

// pingTranscode: POST /api/transcode/{sid}/ping keeps a paused player's session alive.
func (s *Server) pingTranscode(w http.ResponseWriter, r *http.Request) {
	if s.transcodeSession(w, r) == nil {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// stopTranscode: DELETE /api/transcode/{sid} ends a session (the player closed or seeked).
func (s *Server) stopTranscode(w http.ResponseWriter, r *http.Request) {
	if sess := s.transcodeSession(w, r); sess != nil {
		s.transcoder.Stop(sess.ID)
		w.WriteHeader(http.StatusNoContent)
	}
}

// subtitleCues: GET /api/items/{id}/subtitles/{track}?file= returns a subtitle track's cues as
// JSON, for the web player to show.
func (s *Server) subtitleCues(w http.ResponseWriter, r *http.Request) {
	t, cues, ok := s.readSubtitle(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"track": t, "cues": cues})
}

// subtitleVTT: GET /api/items/{id}/subtitles/{track}/vtt?file=&shift_ms= returns a text subtitle
// track as WebVTT, for a cast device to show itself. shift_ms moves every cue (negative is
// earlier): a converted stream's start point and the viewer's timing adjustment. Cues that end
// before 0 are left out. Cast devices fetch it through a cast link (see cast.go).
//
// Players wait for a subtitle file before they start the video, and reading a track out of a
// large video the first time takes minutes (the whole file is read). So the answer starts at
// once with the WebVTT header, stays alive with blank lines (which WebVTT allows between cues)
// while the track is read, then carries the cues: the video starts straight away and the
// subtitles follow.
func (s *Server) subtitleVTT(w http.ResponseWriter, r *http.Request) {
	fileID, _ := strconv.ParseInt(r.URL.Query().Get("file"), 10, 64)
	_, f, ok := s.itemFile(w, r, fileID)
	if !ok {
		return
	}
	t, ok := s.findSubtitle(f, r.PathValue("track"))
	if !ok {
		writeError(w, http.StatusNotFound, "no such subtitle track")
		return
	}
	if s.subtitles == nil {
		writeError(w, http.StatusServiceUnavailable, "subtitles can't be read on this server")
		return
	}
	if t.Image {
		writeError(w, http.StatusUnprocessableEntity, "picture subtitles are shown by burning them into the video")
		return
	}
	shift, _ := strconv.ParseInt(r.URL.Query().Get("shift_ms"), 10, 64)
	type result struct {
		cues []subtitles.Cue
		err  error
	}
	done := make(chan result, 1)
	go func() {
		cues, err := s.subtitles.Cues(r.Context(), f, t)
		done <- result{cues, err}
	}()
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	flusher, _ := w.(http.Flusher)
	write := func(text string) bool {
		if _, err := w.Write([]byte(text)); err != nil {
			return false
		}
		if flusher != nil {
			flusher.Flush()
		}
		return true
	}
	// Ready already (cached, or a file beside the video): the whole file in one answer.
	select {
	case res := <-done:
		if res.err != nil {
			writeError(w, http.StatusUnprocessableEntity, res.err.Error())
			return
		}
		write(webVTT(res.cues, shift))
		return
	case <-time.After(vttWaitBeforeHeader):
	}
	if !write("WEBVTT\n") {
		return
	}
	keepAlive := time.NewTicker(vttKeepAlive)
	defer keepAlive.Stop()
	for {
		select {
		case res := <-done:
			if res.err != nil {
				// Too late for an error status: the player gets a file without cues.
				if r.Context().Err() == nil {
					s.log.Warn("reading subtitles failed", "file", f.ID, "track", t.ID, "err", res.err)
				}
				return
			}
			write(strings.TrimPrefix(webVTT(res.cues, shift), "WEBVTT\n"))
			return
		case <-keepAlive.C:
			if !write("\n") {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

// How long /vtt waits for a track before answering with just the header, and how often it sends
// a blank line after that to keep the player (and any proxy) from giving up on the connection.
var (
	vttWaitBeforeHeader = 500 * time.Millisecond
	vttKeepAlive        = 10 * time.Second
)

// subtitleASS: GET /api/items/{id}/subtitles/{track}/ass?file=&shift_ms= returns an ASS/SSA track
// as its own script, for players that draw it with libass (keeping its fonts, colours, positions
// and effects): shifted as /vtt is, with the fonts the video carries packed into a [Fonts]
// section (up to 40 MB of them). Other tracks answer 400; use /vtt for those.
func (s *Server) subtitleASS(w http.ResponseWriter, r *http.Request) {
	fileID, _ := strconv.ParseInt(r.URL.Query().Get("file"), 10, 64)
	_, f, ok := s.itemFile(w, r, fileID)
	if !ok {
		return
	}
	t, ok := s.findSubtitle(f, r.PathValue("track"))
	if !ok {
		writeError(w, http.StatusNotFound, "no such subtitle track")
		return
	}
	if !t.IsASS() {
		writeError(w, http.StatusBadRequest, "this track isn't ASS or SSA; use /vtt")
		return
	}
	if s.subtitles == nil {
		writeError(w, http.StatusServiceUnavailable, "subtitles can't be read on this server")
		return
	}
	script, err := s.subtitles.Script(r.Context(), f, t)
	if err != nil {
		if r.Context().Err() == nil {
			s.log.Warn("subtitle script failed", "file", f.ID, "track", t.ID, "err", err)
		}
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	// Without its fonts the script still plays, in similar system fonts.
	fonts, err := s.subtitles.Fonts(r.Context(), f)
	if err != nil && r.Context().Err() == nil {
		s.log.Warn("subtitle fonts unavailable", "file", f.ID, "err", err)
	}
	shift, _ := strconv.ParseInt(r.URL.Query().Get("shift_ms"), 10, 64)
	w.Header().Set("Content-Type", "text/x-ssa; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(subtitles.PackScript(script, shift, fonts))
}

// readSubtitle reads the text subtitle track named in the path for the ?file= (or first present)
// file of the item, writing an error if it can't.
func (s *Server) readSubtitle(w http.ResponseWriter, r *http.Request) (subtitles.Track, []subtitles.Cue, bool) {
	fileID, _ := strconv.ParseInt(r.URL.Query().Get("file"), 10, 64)
	_, f, ok := s.itemFile(w, r, fileID)
	if !ok {
		return subtitles.Track{}, nil, false
	}
	t, ok := s.findSubtitle(f, r.PathValue("track"))
	if !ok {
		writeError(w, http.StatusNotFound, "no such subtitle track")
		return subtitles.Track{}, nil, false
	}
	if s.subtitles == nil {
		writeError(w, http.StatusServiceUnavailable, "subtitles can't be read on this server")
		return subtitles.Track{}, nil, false
	}
	cues, err := s.subtitles.Cues(r.Context(), f, t)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return subtitles.Track{}, nil, false
	}
	return t, cues, true
}

// webVTT writes cues as a WebVTT file, each moved by shiftMS.
func webVTT(cues []subtitles.Cue, shiftMS int64) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n")
	for _, c := range cues {
		start, end := c.StartMS+shiftMS, c.EndMS+shiftMS
		if end <= 0 || end <= start {
			continue
		}
		start = max(start, 0)
		// Cue text can't hold blank lines or "-->"; <i>, <b> and <u> are WebVTT tags too.
		var lines []string
		for _, line := range strings.Split(strings.ReplaceAll(c.Text, "-->", "->"), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, strings.ReplaceAll(line, "&", "&amp;"))
			}
		}
		if len(lines) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s --> %s\n%s\n", vttTime(start), vttTime(end), strings.Join(lines, "\n"))
	}
	return b.String()
}

func vttTime(ms int64) string {
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}

// versionDTO describes one of a title's files, for choosing between editions and versions.
type versionDTO struct {
	FileID     int64  `json:"file_id"`
	Edition    string `json:"edition,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	VideoCodec string `json:"video_codec,omitempty"`
	// HDR10 or HLG, from the colour transfer.
	HDR           string   `json:"hdr,omitempty"`
	SourceFormats []string `json:"source_formats,omitempty"`
	Size          int64    `json:"size"`
	DurationMS    int64    `json:"duration_ms,omitempty"`
}

func hdrName(st *store.Stream) string {
	switch st.ColorTransfer {
	case "smpte2084":
		return "HDR10"
	case "arib-std-b67":
		return "HLG"
	}
	return ""
}

// versions lists a title's playable files (the first part of each, for multi-part movies).
func versions(files []store.MediaFile) []versionDTO {
	out := []versionDTO{}
	for _, f := range files {
		if f.Missing || f.Part > 1 {
			continue
		}
		v := versionDTO{FileID: f.ID, Edition: f.Edition, Size: f.Size, DurationMS: f.DurationMS}
		if st := videoStream(&f); st != nil {
			v.Width, v.Height, v.VideoCodec, v.HDR = st.Width, st.Height, st.Codec, hdrName(st)
			if st.DolbyVision {
				v.SourceFormats = append(v.SourceFormats, "Dolby Vision")
			}
			if st.HDR10Plus {
				v.SourceFormats = append(v.SourceFormats, "HDR10+")
			}
			if len(v.SourceFormats) == 0 && v.HDR != "" {
				v.SourceFormats = append(v.SourceFormats, v.HDR)
			}
		}
		out = append(out, v)
	}
	return out
}

// bestVersion picks the file to play: of the chosen edition, one that needs no video conversion
// if there is one, and then the sharpest that fits the screen.
func bestVersion(files []store.MediaFile, edition string, caps clientCaps) *store.MediaFile {
	var candidates []*store.MediaFile
	for i := range files {
		if !files[i].Missing && files[i].Part <= 1 {
			candidates = append(candidates, &files[i])
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	// Only the asked-for edition, if the title has it.
	var ofEdition []*store.MediaFile
	for _, f := range candidates {
		if strings.EqualFold(f.Edition, edition) {
			ofEdition = append(ofEdition, f)
		}
	}
	if len(ofEdition) > 0 {
		candidates = ofEdition
	}
	type scored struct {
		f         *store.MediaFile
		transcode bool
		tooBig    bool
		height    int
	}
	var all []scored
	for _, f := range candidates {
		v := videoStream(f)
		sc := scored{f: f, transcode: decide(f, v, defaultAudio(f), caps).Method == "transcode"}
		if v != nil {
			sc.height = max(v.Height, v.Width*9/16)
		}
		sc.tooBig = caps.MaxHeight > 0 && sc.height > caps.MaxHeight*11/10
		all = append(all, sc)
	}
	slices.SortStableFunc(all, func(a, b scored) int {
		switch {
		case a.transcode != b.transcode:
			if a.transcode {
				return 1
			}
			return -1
		case a.tooBig != b.tooBig:
			if a.tooBig {
				return 1
			}
			return -1
		case a.tooBig:
			return a.height - b.height // smallest of those bigger than the screen
		}
		return b.height - a.height // otherwise the sharpest
	})
	return all[0].f
}

// trickplayInfo: GET /api/items/{id}/trickplay?file=<id> describes a file's timeline thumbnails
// and where its sheets are. Until they're made it's 404, and the file moves up the queue.
func (s *Server) trickplayInfo(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	fileID, _ := strconv.ParseInt(r.URL.Query().Get("file"), 10, 64)
	if !s.itemHasFile(w, r, it.ID, fileID) {
		return
	}
	lib, err := s.store.GetLibrary(r.Context(), it.LibraryID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if s.trickplay == nil || lib == nil || !lib.Trickplay {
		writeError(w, http.StatusNotFound, "scrubbing previews are off for this library")
		return
	}
	info := s.trickplay.Info(fileID)
	if info == nil {
		s.trickplay.Request(fileID)
		writeError(w, http.StatusNotFound, "not made yet")
		return
	}
	sheets := make([]string, info.Sheets)
	for i := range sheets {
		sheets[i] = fmt.Sprintf("/api/items/%s/trickplay/%d/%d", it.ID, fileID, i)
	}
	writeJSON(w, http.StatusOK, map[string]any{"info": info, "sheets": sheets,
		"bif": fmt.Sprintf("/api/items/%s/trickplay/%d/bif", it.ID, fileID)})
}

// trickplayBIF: GET /api/items/{id}/trickplay/{file}/bif is the same thumbnails as one BIF file,
// the format Roku's player shows while seeking. It's 404 until the thumbnails are made.
func (s *Server) trickplayBIF(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	fileID, _ := strconv.ParseInt(r.PathValue("file"), 10, 64)
	if !s.itemHasFile(w, r, it.ID, fileID) {
		return
	}
	if s.trickplay == nil {
		writeError(w, http.StatusNotFound, "not made yet")
		return
	}
	path, err := s.trickplay.BIF(fileID)
	if errors.Is(err, trickplay.ErrNotMade) {
		s.trickplay.Request(fileID)
		writeError(w, http.StatusNotFound, "not made yet")
		return
	}
	if err != nil {
		s.log.Warn("trickplay BIF failed", "file", fileID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, path)
}

// trickplaySheet: GET /api/items/{id}/trickplay/{file}/{sheet} is one sheet of thumbnails.
func (s *Server) trickplaySheet(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	fileID, _ := strconv.ParseInt(r.PathValue("file"), 10, 64)
	n, err := strconv.Atoi(r.PathValue("sheet"))
	if err != nil || !s.itemHasFile(w, r, it.ID, fileID) || s.trickplay == nil {
		if err != nil {
			writeError(w, http.StatusNotFound, "no such sheet")
		}
		return
	}
	path, ok := s.trickplay.Sheet(fileID, n)
	if !ok {
		writeError(w, http.StatusNotFound, "no such sheet")
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeFile(w, r, path)
}

// itemHasFile checks a file belongs to an item, writing a 404 if not.
func (s *Server) itemHasFile(w http.ResponseWriter, r *http.Request, itemID string, fileID int64) bool {
	files, err := s.store.ItemFiles(r.Context(), itemID)
	if err != nil {
		s.fail(w, err)
		return false
	}
	for _, f := range files {
		if f.ID == fileID {
			return true
		}
	}
	writeError(w, http.StatusNotFound, "that file isn't part of this title")
	return false
}

// startConversion starts a conversion and waits for its first segment (and, for copied video, where
// it really starts: the player's clock offset).
func (s *Server) startConversion(r *http.Request, opts transcode.Options) (*transcode.Session, error) {
	sess, err := s.transcoder.Start(opts)
	if err != nil {
		return nil, err
	}
	if err := s.transcoder.Ready(r.Context(), sess); err != nil {
		s.transcoder.Stop(sess.ID)
		return nil, err
	}
	return sess, nil
}

// transcoderCaps is what the converter can do, for the Settings page.
func (s *Server) transcoderCaps() transcode.Capabilities {
	if s.transcoder == nil {
		return transcode.Capabilities{}
	}
	caps := s.transcoder.Caps()
	if s.logs != nil {
		for i := range caps.Attempts {
			caps.Attempts[i].Error = s.logs.Redact(caps.Attempts[i].Error)
		}
	}
	return caps
}

// compatibility: POST /api/libraries/{id}/compatibility {"caps": {...}} says how a library's titles
// would play on a player that can play those things: how many directly, repackaged or converted,
// why, and which titles need converting.
func (s *Server) compatibility(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Caps clientCaps `json:"caps"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, `send {"caps": {...}}`)
		return
	}
	if err := body.Caps.validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	ctx := r.Context()
	byItem, err := s.store.LibraryItemFiles(ctx, lib.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	type converted struct {
		Item    itemDTO  `json:"item"`
		Reasons []string `json:"reasons"`
	}
	out := struct {
		Counts    map[string]int `json:"counts"`  // direct, remux, transcode
		Reasons   map[string]int `json:"reasons"` // why, across everything not played directly
		Converted []converted    `json:"converted"`
		Total     int            `json:"total"`
	}{Counts: map[string]int{"direct": 0, "remux": 0, "transcode": 0}, Reasons: map[string]int{}, Converted: []converted{}}

	var convertIDs []string
	convertReasons := map[string][]string{}
	for itemID, files := range byItem {
		// Files that couldn't be read are listed in Library Health instead.
		var readable []store.MediaFile
		for _, f := range files {
			if f.ProbeError == "" && videoStream(&f) != nil {
				readable = append(readable, f)
			}
		}
		f := bestVersion(readable, "", body.Caps)
		if f == nil {
			continue
		}
		d := decide(f, videoStream(f), defaultAudio(f), body.Caps)
		out.Counts[d.Method]++
		out.Total++
		for _, reason := range d.Reasons {
			out.Reasons[reason]++
		}
		if d.Method == "transcode" {
			convertIDs = append(convertIDs, itemID)
			convertReasons[itemID] = d.Reasons
		}
	}
	// The titles needing conversion, by name; episodes with their show.
	series := map[string]*store.Item{}
	for _, id := range convertIDs {
		it, err := s.store.GetItem(ctx, id)
		if err != nil || it == nil {
			continue
		}
		d := toDTO(*it)
		if it.Kind == "episode" {
			sr, ok := series[it.ParentID]
			if !ok {
				sr, _ = s.store.GetItem(ctx, it.ParentID)
				series[it.ParentID] = sr
			}
			if sr != nil {
				d.SeriesTitle = sr.Title
			}
		}
		out.Converted = append(out.Converted, converted{Item: d, Reasons: convertReasons[id]})
	}
	slices.SortFunc(out.Converted, func(a, b converted) int {
		return strings.Compare(a.Item.SeriesTitle+"\x00"+a.Item.SortTitle, b.Item.SeriesTitle+"\x00"+b.Item.SortTitle)
	})
	writeJSON(w, http.StatusOK, out)
}

// remoteLimit is the most a stream away from home may use (kbps), or 0 for no limit: the server's
// per-stream limit, a share of 80% of the home's upload speed between the remote streams playing,
// and the profile's own internet streaming quality, whichever is lowest. None of them apply at home.
func (s *Server) remoteLimit(r *http.Request, itemID string) int {
	if auth.IsLocal(r) {
		return 0
	}
	st := s.settings()
	limit := st.RemoteKbps
	if st.UploadKbps > 0 {
		share := st.UploadKbps * 8 / 10 / (s.playing.remoteOthers(r, itemID) + 1)
		if limit == 0 || share < limit {
			limit = max(share, 500)
		}
	}
	if s.store != nil {
		if p, err := s.loadPrefs(r); err == nil && p.RemoteQualityKbps > 0 && (limit == 0 || p.RemoteQualityKbps < limit) {
			limit = p.RemoteQualityKbps
		}
	}
	return limit
}

// playbackFile resolves source files without starting playback or conversion.
func (s *Server) playbackFile(w http.ResponseWriter, r *http.Request, itemID string, fileID int64, edition string, caps clientCaps) ([]store.MediaFile, *store.MediaFile) {
	return s.selectPlaybackFile(w, r, itemID, fileID, edition, caps, true)
}
func (s *Server) selectPlaybackFile(w http.ResponseWriter, r *http.Request, itemID string, fileID int64, edition string, caps clientCaps, probe bool) ([]store.MediaFile, *store.MediaFile) {
	files, err := s.store.ItemFiles(r.Context(), itemID)
	if err == nil && probe && s.scanner != nil {
		// A title played before its scan got to reading its files: read them now.
		var read bool
		if read, err = s.scanner.EnsureProbed(r.Context(), files); err == nil && read {
			files, err = s.store.ItemFiles(r.Context(), itemID)
		}
	}
	if err != nil {
		s.fail(w, err)
		return nil, nil
	}
	var f *store.MediaFile
	if fileID != 0 {
		for i := range files {
			if !files[i].Missing && files[i].ID == fileID {
				f = &files[i]
			}
		}
	} else {
		f = bestVersion(files, edition, caps)
	}
	if f == nil {
		writeError(w, http.StatusNotFound, "no playable file for this item")
		return nil, nil
	}
	return files, f
}

func (s *Server) playbackDefaultAudio(r *http.Request, f *store.MediaFile) *store.Stream {
	audio := defaultAudio(f)
	if p, err := s.loadPrefs(r); err == nil && p.AudioLanguage != "" {
		if a := preferredAudio(f, p.AudioLanguage); a != nil {
			audio = a
		}
	}
	return audio
}

func audioTracks(f *store.MediaFile) []audioTrackDTO {
	out := []audioTrackDTO{}
	for _, st := range f.Streams {
		if st.Kind == "audio" {
			out = append(out, audioTrackDTO{Index: st.Index, Language: st.Language,
				Title: st.Title, Codec: st.Codec, Channels: st.Channels, Default: st.IsDefault, Profile: st.Profile, ChannelLayout: st.ChannelLayout})
		}
	}
	return out
}

// playbackOptions: POST /api/items/{id}/playback-options resolves the edition/version and
// source track choices without starting a stream, conversion, progress or preference update.
// Send {"caps": {...}} and optionally "file_id" or "edition" as for playback.
func (s *Server) playbackOptions(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Caps    clientCaps `json:"caps"`
		FileID  int64      `json:"file_id" api:"optional"`
		Edition string     `json:"edition" api:"optional"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, `send {"caps": {...}} and optionally "file_id" or "edition"`)
		return
	}
	if err := body.Caps.validate(); err != nil {
		writeError(w, 400, err.Error())
		return
	}
	files, f := s.playbackFile(w, r, it.ID, body.FileID, body.Edition, body.Caps)
	if f == nil {
		return
	}
	audio := s.playbackDefaultAudio(r, f)
	index := -1
	if audio != nil {
		index = audio.Index
	}
	writeJSON(w, http.StatusOK, struct {
		FileID         int64             `json:"file_id" api:"optional"`
		AudioStream    int               `json:"audio_stream" api:"optional"`
		AudioTracks    []audioTrackDTO   `json:"audio_tracks"`
		SubtitleTracks []subtitles.Track `json:"subtitle_tracks"`
		Versions       []versionDTO      `json:"versions"`
	}{f.ID, index, audioTracks(f), s.subtitleTracks(f), versions(files)})
}

// logPlayback records who started what, on which device, and how it plays (and why), so a
// conversion or a refused format can be traced from the log.
func (s *Server) logPlayback(r *http.Request, fileID int64, method string, reasons []string) {
	title, _ := s.store.FileActivityLabel(r.Context(), fileID)
	name := ""
	if u := currentUser(r); u != nil {
		name = u.Name
	}
	how, ok := map[string]string{"direct": "direct play", "remux": "repackaged", "transcode": "converted"}[method]
	if !ok {
		how = method
	}
	s.log.Info("playback", "title", title, "profile", name, "device", deviceName(r.UserAgent()), "method", how, "because", strings.Join(reasons, ", "))
}

// detectHardware: POST /api/transcoder/detect starts one bounded hardware encoding recheck (admin).
// Repeated requests while detection is running reuse that job. No media files are inspected.
func (s *Server) detectHardware(w http.ResponseWriter, r *http.Request) {
	if s.transcoder == nil {
		writeError(w, http.StatusServiceUnavailable, "transcoding unavailable")
		return
	}
	started := s.transcoder.RequestDetect(context.WithoutCancel(r.Context()))
	writeJSON(w, http.StatusAccepted, map[string]bool{"started": started})
}
