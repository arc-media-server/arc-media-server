// Package transcode converts media with FFmpeg for clients that can't play a file as it is. Each
// playback gets a session: FFmpeg writes an HLS playlist and fMP4 segments into its own folder,
// growing as it goes, from a start point. Seeking beyond what's been converted starts a new
// session at the new point, and the player adds that offset to its clock.
package transcode

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// How long a session lives without any request from its player (the player pings while open).
const idleTimeout = 2 * time.Minute

// Options say what to make from which file.
type Options struct {
	Path string
	// Where to start, in milliseconds.
	StartMS int64
	// The streams to use, by their index in the file; AudioStream -1 for none.
	VideoStream, AudioStream int
	// Copy the video as it is (remux), or re-encode it to H.264.
	CopyVideo bool
	// Copy the audio as it is (AAC stereo, or Dolby Digital the player decodes), or convert it:
	// to AudioCodec ("eac3" or "ac3", with AudioChannels) for surround sound, else AAC stereo.
	CopyAudio     bool
	AudioCodec    string
	AudioChannels int
	// When re-encoding: the largest height to output (0 keeps the source size) and the video
	// bitrate in kbit/s (0 for a quality-based default).
	MaxHeight    int
	VideoKbps    int
	SourceHeight int
	// A picture subtitle stream to burn into the video (re-encoding only), or -1.
	BurnSubtitle int
	// When re-encoding: a hardware H.264 encoder to use (e.g. h264_nvenc), "" for the CPU, and
	// whether to map HDR to SDR.
	HWEncoder  string
	HWDevice   string
	HWDriver   string
	HWLowPower bool
	ToneMap    bool
	// Use tonemapx (jellyfin-ffmpeg) for ToneMap instead of the zscale chain.
	FastToneMap bool
	// The source video codec (e.g. hevc), for tagging copied video.
	VideoCodec string
	// The source video has Dolby Vision: copied video should keep it (checked once it's written).
	DolbyVision bool
	// Who started it and for what, for the server's own checks.
	Owner, ItemID string
}

// Session is one running conversion.
type Session struct {
	ID string
	// Where the output starts in the source, in milliseconds: the player's clock is offset by it.
	OffsetMS int64
	Options  Options

	dir           string
	cancel        context.CancelFunc
	done          chan struct{}
	detectMu      sync.Mutex
	probeHardware func(context.Context, string, []string, string) ([]byte, error)
	renderNodes   func() []string
	mu            sync.Mutex
	err           error
	stderr        bytes.Buffer
	lastSeen      time.Time
	// Throttling (throttle.go): FFmpeg's process, the media segment the player last asked for
	// (-1 before any) and whether FFmpeg is paused for being far enough ahead.
	proc       *os.Process
	requested  int
	paused     bool
	throttling throttling
	// pausedAt is the segment the player was on when the conversion paused; when the player
	// moves on from it, the conversion resumes and makes at least one segment (more than
	// madeAtResume) before it pauses again.
	pausedAt, madeAtResume int
	throttleWake           chan struct{}
	throttleDone           chan struct{}
	outputWatcher          *fsnotify.Watcher
	throttleFailure        func()
}

// Manager runs sessions.
type Manager struct {
	detectMu      sync.Mutex
	probeHardware func(context.Context, string, []string, string) ([]byte, error)
	renderNodes   func() []string
	ffmpeg        func() string
	probe         func() string
	log           *slog.Logger
	// Whether the "Dolby Vision dropped" warning was logged this run.
	dvWarning            sync.Once
	throttleWatchWarning sync.Once
	throttlePauseWarning sync.Once

	mu       sync.Mutex
	dir      string // where sessions write, chosen in Settings
	maxVideo int    // video conversions at once, 0 for no limit
	// How far ahead of the player sessions convert (throttle.go); each session keeps its own copy.
	throttling throttling
	sessions   map[string]*Session
	caps       Capabilities
}

// ErrBusy means as many videos are being converted as Settings allows.
var ErrBusy = errors.New("Arc is already converting as many videos as it's allowed to at once; try again when one finishes, or play a version that doesn't need converting")

// Configure sets where new sessions write (a folder of their own inside dir) and how many videos
// may be converted at once (0 for no limit). Running sessions carry on where they are.
func (m *Manager) Configure(dir string, maxVideo int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if dir != m.dir {
		// A folder of Arc's own: clear what a crash may have left, unless sessions are running there.
		inUse := false
		for _, s := range m.sessions {
			inUse = inUse || filepath.Dir(s.dir) == dir
		}
		if !inUse {
			os.RemoveAll(dir)
		}
	}
	m.dir, m.maxVideo = dir, maxVideo
}

// New makes a manager keeping sessions under dir (cleared now: nothing in it outlives a restart).
// ffmpeg and ffprobe return the binaries to use.
func New(dir string, ffmpeg, ffprobe func() string, log *slog.Logger) *Manager {
	os.RemoveAll(dir)
	return &Manager{dir: dir, ffmpeg: ffmpeg, probe: ffprobe, log: log, sessions: map[string]*Session{}, throttling: defaultThrottling}
}

// Run ends idle sessions until ctx is done, then ends them all.
func (m *Manager) Run(ctx context.Context) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			for id := range m.sessions {
				m.stopLocked(id)
			}
			m.mu.Unlock()
			return
		case <-t.C:
			m.mu.Lock()
			for id, s := range m.sessions {
				s.mu.Lock()
				idle := time.Since(s.lastSeen) > idleTimeout
				s.mu.Unlock()
				if idle {
					m.log.Debug("ending idle transcode", "session", id)
					m.stopLocked(id)
				}
			}
			m.mu.Unlock()
		}
	}
}

// Start begins a session.
func (m *Manager) Start(opts Options) (*Session, error) {
	bin := m.ffmpeg()
	if bin == "" {
		return nil, errors.New("FFmpeg wasn't found, so this file can't be converted")
	}
	m.mu.Lock()
	// Reserve capacity through registration. Releasing the lock before starting the process
	// lets simultaneous requests all observe the same free slot.
	defer m.mu.Unlock()
	if opts.HWEncoder != "" && opts.HWEncoder == m.caps.HWEncoder && opts.HWDevice == "" {
		opts.HWDevice, opts.HWDriver, opts.HWLowPower = m.caps.HWDevice, m.caps.HWDriver, m.caps.HWLowPower
	}
	dir, max := m.dir, m.maxVideo
	if max > 0 && !opts.CopyVideo {
		n := 0
		for _, o := range m.sessions {
			if !o.Options.CopyVideo {
				n++
			}
		}
		if n >= max {
			return nil, ErrBusy
		}
	}
	var b [8]byte
	rand.Read(b[:])
	s := &Session{ID: hex.EncodeToString(b[:]), Options: opts, OffsetMS: opts.StartMS, lastSeen: time.Now(),
		done: make(chan struct{}), throttleWake: make(chan struct{}, 1), throttleDone: make(chan struct{}), requested: -1, throttling: m.throttling}
	s.dir = filepath.Join(dir, s.ID)
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}

	s.outputWatcher = s.watchOutput()
	if s.outputWatcher == nil {
		m.throttleWatchWarning.Do(func() { m.log.Warn("conversion output notifications unavailable; pacing uses periodic checks") })
	}
	s.throttleFailure = func() {
		m.throttlePauseWarning.Do(func() {
			m.log.Warn("conversion pacing could not suspend or resume the process; converting at normal speed")
		})
	}
	args := buildArgs(opts, float64(opts.StartMS)/1000, s.dir)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	cmd := exec.CommandContext(ctx, bin, args...)
	if opts.HWDriver != "" {
		cmd.Env = withVADriver(os.Environ(), opts.HWDriver)
	}
	cmd.Stderr = &s.stderr
	if err := cmd.Start(); err != nil {
		if s.outputWatcher != nil {
			s.outputWatcher.Close()
		}
		cancel()
		os.RemoveAll(s.dir)
		return nil, fmt.Errorf("starting FFmpeg: %w", err)
	}
	m.log.Info("transcode started", "session", s.ID, "file", filepath.Base(opts.Path), "copy_video", opts.CopyVideo,
		"copy_audio", opts.CopyAudio, "audio_codec", opts.AudioCodec, "start", time.Duration(s.OffsetMS)*time.Millisecond)
	m.log.Debug("ffmpeg", "args", strings.Join(args, " "))
	s.proc = cmd.Process
	go s.throttle()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		if err != nil && ctx.Err() == nil {
			s.err = fmt.Errorf("FFmpeg stopped: %s", lastLine(s.stderr.String()))
			m.log.Warn("transcode failed", "session", s.ID, "err", s.err)
		}
		s.paused = false
		s.proc = nil
		s.mu.Unlock()
		close(s.done)
	}()

	m.sessions[s.ID] = s
	return s, nil
}

func buildArgs(o Options, start float64, dir string) []string {
	a := []string{"-hide_banner", "-nostdin", "-v", "error"}
	encode := !o.CopyVideo
	if encode && o.HWEncoder == "h264_vaapi" {
		device := o.HWDevice
		if device == "" {
			device = "/dev/dri/renderD128"
		}
		a = append(a, "-vaapi_device", device)
	}
	if encode && o.HWEncoder == "h264_qsv" && o.HWDevice != "" {
		a = append(a, "-qsv_device", o.HWDevice)
	}
	if encode && o.HWEncoder != "" {
		// Decode on the graphics card too when it can; frames come back for filtering.
		a = append(a, "-hwaccel", "auto")
	}
	if start > 0 {
		a = append(a, "-ss", secs(start))
		if o.CopyVideo {
			// Copied video can only start on a keyframe. Let FFmpeg start from the one it finds
			// and keep the file's own timestamps; Ready measures where the output really begins.
			a = append(a, "-noaccurate_seek")
		}
	}
	if o.CopyVideo {
		a = append(a, "-copyts")
	}
	a = append(a, "-i", o.Path)

	// Filters, in order: HDR to SDR, picture subtitles drawn on, scaling, hardware upload.
	var pre, post []string
	if encode && o.ToneMap {
		if o.FastToneMap {
			pre = append(pre, FastToneMapFilter)
		} else {
			pre = append(pre, ToneMapFilter)
		}
	}
	if encode && o.MaxHeight > 0 && o.SourceHeight > o.MaxHeight {
		post = append(post, "scale=-2:"+strconv.Itoa(o.MaxHeight))
	}
	if encode && o.HWEncoder == "h264_vaapi" {
		post = append(post, "format=nv12", "hwupload")
	}
	if encode && o.BurnSubtitle >= 0 {
		in := fmt.Sprintf("[0:%d]", o.VideoStream)
		fc := ""
		if len(pre) > 0 {
			fc = in + strings.Join(pre, ",") + "[pre];"
			in = "[pre]"
		}
		fc += fmt.Sprintf("%s[0:%d]overlay=eof_action=pass", in, o.BurnSubtitle)
		if len(post) > 0 {
			fc += "," + strings.Join(post, ",")
		}
		a = append(a, "-filter_complex", fc+"[v]", "-map", "[v]")
	} else {
		a = append(a, "-map", "0:"+strconv.Itoa(o.VideoStream))
	}
	if o.AudioStream >= 0 {
		a = append(a, "-map", "0:"+strconv.Itoa(o.AudioStream))
	}
	a = append(a, "-map_metadata", "-1", "-map_chapters", "-1", "-sn", "-dn")

	if o.CopyVideo {
		a = append(a, "-c:v", "copy", "-avoid_negative_ts", "disabled")
		if o.VideoCodec == "hevc" {
			// Safari and Media Source Extensions want HEVC tagged hvc1.
			a = append(a, "-tag:v", "hvc1")
		}
		// FFmpeg writes a copied stream's Dolby Vision configuration (dvcC/dvvC) into MP4 only
		// when allowed "unofficial" boxes; without it a player sees plain HEVC and shows the
		// HDR10 base layer instead of Dolby Vision. Streams without Dolby Vision are unaffected.
		a = append(a, "-strict", "unofficial")
	} else {
		if filters := append(pre, post...); o.BurnSubtitle < 0 && len(filters) > 0 {
			a = append(a, "-vf", strings.Join(filters, ","))
		}
		a = append(a, encoderArgs(o)...)
		// A keyframe every 3 seconds so segments are even and seeking is quick.
		a = append(a, "-force_key_frames", "expr:gte(t,n_forced*3)")
	}
	if o.AudioStream >= 0 {
		switch {
		case o.CopyAudio:
			a = append(a, "-c:a", "copy")
		case (o.AudioCodec == "eac3" || o.AudioCodec == "ac3") && o.AudioChannels > 2:
			a = append(a, "-c:a", o.AudioCodec, "-ac", strconv.Itoa(min(o.AudioChannels, 6)), "-b:a", "640k")
		default:
			a = append(a, "-c:a", "aac", "-ac", "2", "-b:a", "192k")
		}
	}
	return append(a, "-max_muxing_queue_size", "4096",
		"-f", "hls", "-hls_time", "6", "-hls_playlist_type", "event", "-hls_list_size", "0",
		"-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", "init.mp4", "-hls_flags", "temp_file+independent_segments",
		// Forward slashes: FFmpeg finds the folder for init.mp4 by looking for "/".
		"-hls_segment_filename", filepath.ToSlash(filepath.Join(dir, "s%05d.m4s")), filepath.ToSlash(filepath.Join(dir, "index.m3u8")))
}

// ToneMapFilter maps HDR (PQ or HLG) to SDR so converted video (and previews) don't look washed out.
const ToneMapFilter = "zscale=t=linear:npl=100,format=gbrpf32le,zscale=p=bt709,tonemap=tonemap=hable:desat=0,zscale=t=bt709:m=bt709:r=tv,format=yuv420p"

// FastToneMapFilter is the same mapping with jellyfin-ffmpeg's tonemapx: BT.2390, many times
// faster than zscale's float chain, and it applies Dolby Vision's own colour metadata where the
// stream has it. tonemapx takes 10-bit 4:2:0, so other input is converted to that first.
const FastToneMapFilter = "format=yuv420p10le,tonemapx=tonemap=bt2390:desat=0:peak=100:t=bt709:m=bt709:p=bt709:r=tv:format=yuv420p"

// encoderArgs chooses the H.264 encoder and its quality settings.
func encoderArgs(o Options) []string {
	if o.HWEncoder == "" {
		a := []string{"-c:v", "libx264", "-preset", "veryfast", "-profile:v", "high", "-pix_fmt", "yuv420p"}
		if o.VideoKbps > 0 {
			k := strconv.Itoa(o.VideoKbps)
			return append(a, "-b:v", k+"k", "-maxrate", k+"k", "-bufsize", strconv.Itoa(o.VideoKbps*2)+"k")
		}
		return append(a, "-crf", "21", "-maxrate", "20M", "-bufsize", "40M")
	}
	// Hardware encoders: a bitrate that suits the picture size, unless one was asked for.
	kbps := o.VideoKbps
	if kbps == 0 {
		h := o.SourceHeight
		if o.MaxHeight > 0 && o.MaxHeight < h {
			h = o.MaxHeight
		}
		switch {
		case h > 1600:
			kbps = 25000
		case h > 1100:
			kbps = 16000
		case h > 800:
			kbps = 10000
		case h > 600:
			kbps = 5000
		default:
			kbps = 2500
		}
	}
	k, max, buf := strconv.Itoa(kbps)+"k", strconv.Itoa(kbps*3/2)+"k", strconv.Itoa(kbps*2)+"k"
	a := []string{"-c:v", o.HWEncoder, "-b:v", k, "-maxrate", max, "-bufsize", buf}
	if o.HWLowPower {
		a = append(a, "-low_power", "1")
	}
	switch o.HWEncoder {
	case "h264_nvenc":
		a = append(a, "-preset", "p4", "-profile:v", "high", "-pix_fmt", "yuv420p", "-forced-idr", "1")
	case "h264_qsv":
		a = append(a, "-preset", "medium", "-profile:v", "high", "-pix_fmt", "nv12")
	case "h264_amf":
		a = append(a, "-quality", "balanced", "-profile:v", "high", "-pix_fmt", "yuv420p")
	case "h264_videotoolbox":
		a = append(a, "-profile:v", "high", "-pix_fmt", "yuv420p", "-allow_sw", "1")
	case "h264_vaapi":
		a = append(a, "-profile:v", "high")
	}
	return a
}

// Ready waits for a session's first segment and, for copied video, measures where the output starts in
// the file (its first timestamp) and sets OffsetMS to it.
func (m *Manager) Ready(ctx context.Context, s *Session) error {
	ffprobe := m.probe()
	if _, err := s.Playlist(ctx); err != nil {
		return err
	}
	m.checkDolbyVision(s)
	if !s.Options.CopyVideo || s.Options.StartMS == 0 || ffprobe == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-show_entries", "format=start_time", "-of", "csv=p=0",
		filepath.Join(s.dir, "index.m3u8")).Output()
	if err != nil {
		return nil // keep the requested start; the clock may be a little out
	}
	if v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil && v >= 0 {
		s.OffsetMS = int64(v * 1000)
	}
	return nil
}

// checkDolbyVision warns (once per run) when copied Dolby Vision video came out without its
// configuration box: FFmpeg before 6.1 drops it when packaging HLS, so players show the HDR10 base.
func (m *Manager) checkDolbyVision(s *Session) {
	if !s.Options.CopyVideo || !s.Options.DolbyVision {
		return
	}
	init, err := os.ReadFile(filepath.Join(s.dir, "init.mp4"))
	if err != nil || bytes.Contains(init, []byte("dvvC")) || bytes.Contains(init, []byte("dvcC")) || bytes.Contains(init, []byte("dvwC")) {
		return
	}
	m.dvWarning.Do(func() {
		m.log.Warn("FFmpeg didn't keep Dolby Vision when repackaging, so players get the HDR10 base; FFmpeg 6.1 or newer keeps it",
			"session", s.ID)
	})
}

// Get returns a running session and marks it as in use.
func (m *Manager) Get(id string) *Session {
	m.mu.Lock()
	s := m.sessions[id]
	m.mu.Unlock()
	if s != nil {
		s.mu.Lock()
		s.lastSeen = time.Now()
		s.mu.Unlock()
	}
	return s
}

// Peek returns a running session without marking it as in use (for status displays).
func (m *Manager) Peek(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

// Stop ends a session and deletes its files.
func (m *Manager) Stop(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopLocked(id)
}

func (m *Manager) stopLocked(id string) {
	s := m.sessions[id]
	if s == nil {
		return
	}
	delete(m.sessions, id)
	s.cancel()
	go func() {
		// Windows won't delete files FFmpeg still has open.
		select {
		case <-s.done:
		case <-time.After(10 * time.Second):
		}
		os.RemoveAll(s.dir)
	}()
	m.log.Info("transcode ended", "session", id)
}

// Count is how many sessions are running.
func (m *Manager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

// Playlist waits until the first segment is ready (or ctx ends) and returns the playlist.
func (s *Session) Playlist(ctx context.Context) ([]byte, error) {
	path := filepath.Join(s.dir, "index.m3u8")
	for {
		if b, err := os.ReadFile(path); err == nil && bytes.Contains(b, []byte("#EXTINF")) {
			return b, nil
		}
		select {
		case <-s.done:
			// Finished (a very short file) or failed.
			if b, err := os.ReadFile(path); err == nil && bytes.Contains(b, []byte("#EXTINF")) {
				return b, nil
			}
			s.mu.Lock()
			err := s.err
			s.mu.Unlock()
			if err == nil {
				err = errors.New("FFmpeg didn't produce anything")
			}
			return nil, err
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

var fileRe = regexp.MustCompile(`^(init\.mp4|s\d{5}\.m4s)$`)

// File returns the path of a finished output file (the init segment or a media segment).
func (s *Session) File(name string) (string, bool) {
	if !fileRe.MatchString(name) {
		return "", false
	}
	p := filepath.Join(s.dir, name)
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}

// Err is why FFmpeg stopped early, if it did.
func (s *Session) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func secs(f float64) string { return strconv.FormatFloat(f, 'f', 3, 64) }

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n")), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		return "no details"
	}
	return lines[len(lines)-1]
}
