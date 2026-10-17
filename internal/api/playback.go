package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"ams/internal/store"
	"ams/internal/transcode"
)

// How a file reaches the player:
//   - direct: the file as it is, with range requests for seeking;
//   - remux: the video copied as it is into a format the player can open, with the audio
//     converted if needed (quick, nothing lost);
//   - transcode: the video re-encoded to H.264 (slow, uses the CPU).

// clientCaps is what the player can play, as it reports it.
type clientCaps struct {
	Containers []string `json:"containers"` // mp4, webm, mkv
	Video      []string `json:"video"`      // h264, hevc, vp9, av1
	Audio      []string `json:"audio"`      // aac, mp3, opus, flac, ac3, eac3, vorbis
	// The largest picture worth sending (e.g. the screen's height); 0 for no limit.
	MaxHeight int `json:"max_height"`
}

func (c clientCaps) has(list []string, v string) bool { return slices.Contains(list, v) }

type decision struct {
	Method    string
	CopyAudio bool
	Reasons   []string
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

// decide works out how a file can be played with the chosen audio stream (nil for none).
func decide(f *store.MediaFile, video, audio *store.Stream, caps clientCaps) decision {
	d := decision{Method: "direct"}
	videoOK := video != nil && caps.has(caps.Video, video.Codec) &&
		// Browsers can't decode 10-bit H.264 (Hi10P), though they say they play H.264.
		!(video.Codec == "h264" && video.BitDepth > 8)
	switch {
	case video == nil:
		d.Reasons = append(d.Reasons, "no video found")
	case !videoOK && video.Codec == "h264":
		d.Reasons = append(d.Reasons, "10-bit H.264 video")
	case !videoOK:
		d.Reasons = append(d.Reasons, codecName(video.Codec)+" video")
	}
	if fam := containerFamily(f.Container); !caps.has(caps.Containers, fam) {
		d.Reasons = append(d.Reasons, strings.ToUpper(fam)+" file")
	}
	audioOK := audio == nil || caps.has(caps.Audio, audio.Codec)
	if !audioOK {
		d.Reasons = append(d.Reasons, codecName(audio.Codec)+" audio")
	}
	if audio != nil && audio != defaultAudio(f) {
		d.Reasons = append(d.Reasons, "another audio track chosen")
	}
	// Copied audio must be something every HLS player handles: AAC in stereo or less.
	d.CopyAudio = audio != nil && audio.Codec == "aac" && audio.Channels <= 2 && audioOK
	switch {
	case len(d.Reasons) == 0:
		d.Method = "direct"
	case videoOK:
		d.Method = "remux"
	default:
		d.Method = "transcode"
	}
	return d
}

type audioTrackDTO struct {
	Index    int    `json:"index"`
	Language string `json:"language,omitempty"`
	Title    string `json:"title,omitempty"`
	Codec    string `json:"codec"`
	Channels int    `json:"channels,omitempty"`
	Default  bool   `json:"default"`
}

type playbackPlan struct {
	Method  string   `json:"method"` // direct, remux or transcode
	Reasons []string `json:"reasons"`
	URL     string   `json:"url"`
	// The URL is an HLS playlist (play it with hls.js, or natively in Safari).
	HLS       bool   `json:"hls"`
	SessionID string `json:"session_id,omitempty"`
	// Where the stream starts in the file; add it to the player's clock.
	OffsetMS    int64           `json:"offset_ms"`
	DurationMS  int64           `json:"duration_ms"`
	FileID      int64           `json:"file_id"`
	AudioStream int             `json:"audio_stream"` // -1 for none
	AudioTracks []audioTrackDTO `json:"audio_tracks"`
}

// startPlayback: POST /api/items/{id}/playback {"caps": {...}, "file_id", "audio_stream",
// "start_ms"} decides how to play a movie or episode on this player and, unless it can play the
// file as it is, starts converting it. Starting again (e.g. to seek or change audio track) ends
// the player's previous conversion.
func (s *Server) startPlayback(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Caps        clientCaps `json:"caps"`
		FileID      int64      `json:"file_id"`
		AudioStream *int       `json:"audio_stream"`
		StartMS     int64      `json:"start_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.StartMS < 0 {
		writeError(w, http.StatusBadRequest, `send {"caps": {...}} and optionally "file_id", "audio_stream", "start_ms"`)
		return
	}
	files, err := s.store.ItemFiles(r.Context(), it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	var f *store.MediaFile
	for i := range files {
		if !files[i].Missing && (body.FileID == 0 || files[i].ID == body.FileID) {
			f = &files[i]
			break
		}
	}
	if f == nil {
		writeError(w, http.StatusNotFound, "no playable file for this item")
		return
	}
	if why := s.playing.refusalFor(r, it.ID); why != "" {
		writeError(w, http.StatusForbidden, why)
		return
	}

	video := videoStream(f)
	audio := defaultAudio(f)
	if body.AudioStream != nil {
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

	out := playbackPlan{Method: d.Method, Reasons: d.Reasons, DurationMS: f.DurationMS, FileID: f.ID,
		AudioStream: -1, AudioTracks: []audioTrackDTO{}}
	if out.Reasons == nil {
		out.Reasons = []string{}
	}
	if audio != nil {
		out.AudioStream = audio.Index
	}
	for _, st := range f.Streams {
		if st.Kind == "audio" {
			out.AudioTracks = append(out.AudioTracks, audioTrackDTO{Index: st.Index, Language: st.Language,
				Title: st.Title, Codec: st.Codec, Channels: st.Channels, Default: st.IsDefault})
		}
	}

	if d.Method == "direct" {
		out.URL = fmt.Sprintf("/api/items/%s/stream?file=%d", it.ID, f.ID)
		out.OffsetMS = 0
		s.playing.setMethod(r, it.ID, "direct", "", s.transcoder)
		writeJSON(w, http.StatusOK, out)
		return
	}
	if s.transcoder == nil {
		writeError(w, http.StatusServiceUnavailable, "this server can't convert files")
		return
	}
	opts := transcode.Options{Path: f.Path, StartMS: body.StartMS, VideoStream: video.Index, AudioStream: -1,
		CopyVideo: d.Method == "remux", CopyAudio: d.CopyAudio, MaxHeight: body.Caps.MaxHeight, SourceHeight: video.Height,
		VideoCodec: video.Codec,
		Owner:      currentSession(r), ItemID: it.ID}
	if audio != nil {
		opts.AudioStream = audio.Index
	}
	sess, err := s.transcoder.Start(opts)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	// The first segment, and for copied video where it really starts (the player's clock offset).
	if err := s.transcoder.Ready(r.Context(), sess); err != nil {
		s.transcoder.Stop(sess.ID)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.playing.setMethod(r, it.ID, d.Method, sess.ID, s.transcoder)
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
