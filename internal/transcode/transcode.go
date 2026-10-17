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
	// Copy the audio as it is (it's already AAC stereo), or convert it to AAC stereo.
	CopyAudio bool
	// When re-encoding: the largest height to output (0 keeps the source size) and the video
	// bitrate in kbit/s (0 for a quality-based default).
	MaxHeight    int
	VideoKbps    int
	SourceHeight int
	// A picture subtitle stream to burn into the video (re-encoding only), or -1.
	BurnSubtitle int
	// The source video codec (e.g. hevc), for tagging copied video.
	VideoCodec string
	// Who started it and for what, for the server's own checks.
	Owner, ItemID string
}

// Session is one running conversion.
type Session struct {
	ID string
	// Where the output starts in the source, in milliseconds: the player's clock is offset by it.
	OffsetMS int64
	Options  Options

	dir      string
	cancel   context.CancelFunc
	done     chan struct{}
	mu       sync.Mutex
	err      error
	stderr   bytes.Buffer
	lastSeen time.Time
}

// Manager runs sessions.
type Manager struct {
	dir    string
	ffmpeg func() string
	probe  func() string
	log    *slog.Logger

	mu       sync.Mutex
	sessions map[string]*Session
}

// New makes a manager keeping sessions under dir (cleared now: nothing in it outlives a restart).
// ffmpeg and ffprobe return the binaries to use.
func New(dir string, ffmpeg, ffprobe func() string, log *slog.Logger) *Manager {
	os.RemoveAll(dir)
	return &Manager{dir: dir, ffmpeg: ffmpeg, probe: ffprobe, log: log, sessions: map[string]*Session{}}
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
	var b [8]byte
	rand.Read(b[:])
	s := &Session{ID: hex.EncodeToString(b[:]), Options: opts, OffsetMS: opts.StartMS, lastSeen: time.Now(),
		done: make(chan struct{})}
	s.dir = filepath.Join(m.dir, s.ID)
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}

	args := buildArgs(opts, float64(opts.StartMS)/1000, s.dir)

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stderr = &s.stderr
	if err := cmd.Start(); err != nil {
		cancel()
		os.RemoveAll(s.dir)
		return nil, fmt.Errorf("starting FFmpeg: %w", err)
	}
	m.log.Info("transcode started", "session", s.ID, "file", filepath.Base(opts.Path), "copy_video", opts.CopyVideo,
		"copy_audio", opts.CopyAudio, "start", time.Duration(s.OffsetMS)*time.Millisecond)
	m.log.Debug("ffmpeg", "args", strings.Join(args, " "))
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		if err != nil && ctx.Err() == nil {
			s.err = fmt.Errorf("FFmpeg stopped: %s", lastLine(s.stderr.String()))
			m.log.Warn("transcode failed", "session", s.ID, "err", s.err)
		}
		s.mu.Unlock()
		close(s.done)
	}()

	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s, nil
}

func buildArgs(o Options, start float64, dir string) []string {
	a := []string{"-hide_banner", "-nostdin", "-v", "error"}
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
	var scale string
	if !o.CopyVideo && o.MaxHeight > 0 && o.SourceHeight > o.MaxHeight {
		scale = "scale=-2:" + strconv.Itoa(o.MaxHeight)
	}
	if o.BurnSubtitle >= 0 && !o.CopyVideo {
		// Picture subtitles drawn onto the video, then scaled with it.
		fc := fmt.Sprintf("[0:%d][0:%d]overlay=eof_action=pass", o.VideoStream, o.BurnSubtitle)
		if scale != "" {
			fc += "," + scale
		}
		a = append(a, "-filter_complex", fc+"[v]", "-map", "[v]")
		scale = ""
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
	} else {
		a = append(a, "-c:v", "libx264", "-preset", "veryfast", "-profile:v", "high", "-pix_fmt", "yuv420p",
			// A keyframe every 3 seconds so segments are even and seeking is quick.
			"-force_key_frames", "expr:gte(t,n_forced*3)")
		if o.VideoKbps > 0 {
			k := strconv.Itoa(o.VideoKbps)
			a = append(a, "-b:v", k+"k", "-maxrate", k+"k", "-bufsize", strconv.Itoa(o.VideoKbps*2)+"k")
		} else {
			a = append(a, "-crf", "21", "-maxrate", "20M", "-bufsize", "40M")
		}
		if scale != "" {
			a = append(a, "-vf", scale)
		}
	}
	if o.AudioStream >= 0 {
		if o.CopyAudio {
			a = append(a, "-c:a", "copy")
		} else {
			a = append(a, "-c:a", "aac", "-ac", "2", "-b:a", "192k")
		}
	}
	return append(a, "-max_muxing_queue_size", "4096",
		"-f", "hls", "-hls_time", "6", "-hls_playlist_type", "event", "-hls_list_size", "0",
		"-hls_segment_type", "fmp4", "-hls_fmp4_init_filename", "init.mp4", "-hls_flags", "temp_file+independent_segments",
		// Forward slashes: FFmpeg finds the folder for init.mp4 by looking for "/".
		"-hls_segment_filename", filepath.ToSlash(filepath.Join(dir, "s%05d.m4s")), filepath.ToSlash(filepath.Join(dir, "index.m3u8")))
}

// Ready waits for a session's first segment and, for copied video, measures where the output starts in
// the file (its first timestamp) and sets OffsetMS to it.
func (m *Manager) Ready(ctx context.Context, s *Session) error {
	ffprobe := m.probe()
	if _, err := s.Playlist(ctx); err != nil {
		return err
	}
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
