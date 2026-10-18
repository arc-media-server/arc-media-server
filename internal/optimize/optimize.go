// Package optimize makes optimized versions: easy-to-play copies of files (H.264 video and stereo
// AAC in an MP4 that starts playing at once, at most 1080p or 720p), in Arc's data folder, so
// players that can't play a file as it is (or remote ones on a limited connection) needn't wait
// for live conversion. As Plex's Optimize. Media files are only read.
package optimize

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/activity"
	"ams/internal/store"
	"ams/internal/transcode"
)

// Qualities are the copies that can be made: the most lines tall, and the video bitrate (kbps).
var Qualities = map[int]int{1080: 8000, 720: 4000}

// Manager makes the queued copies one at a time.
type Manager struct {
	store *store.Store
	dir   string
	log   *slog.Logger

	FFmpeg   func() string
	ToneMap  func() bool // FFmpeg can map HDR to SDR
	Activity *activity.Tracker

	mu     sync.Mutex
	cancel map[string]context.CancelFunc // the copy being made, by file and quality
	wake   chan struct{}
}

func New(dir string, st *store.Store, log *slog.Logger) *Manager {
	return &Manager{store: st, dir: dir, log: log, cancel: map[string]context.CancelFunc{}, wake: make(chan struct{}, 1)}
}

// Wake starts on newly queued copies.
func (m *Manager) Wake() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func key(fileID int64, quality int) string { return fmt.Sprintf("%d-%d", fileID, quality) }

// Delete removes a copy, stopping it first if it's being made.
func (m *Manager) Delete(ctx context.Context, fileID int64, quality int) error {
	m.mu.Lock()
	if c := m.cancel[key(fileID, quality)]; c != nil {
		c()
	}
	m.mu.Unlock()
	path, err := m.store.DeleteOptimized(ctx, fileID, quality)
	if err != nil {
		return err
	}
	if path != "" {
		os.Remove(path)
	}
	return nil
}

// Run makes queued copies until ctx ends.
func (m *Manager) Run(ctx context.Context) {
	if err := m.store.ResetConverting(ctx); err != nil {
		m.log.Warn("resuming optimized versions failed", "err", err)
	}
	for {
		for ctx.Err() == nil {
			o, err := m.store.NextOptimize(ctx)
			if err != nil {
				m.log.Warn("listing optimized versions failed", "err", err)
				break
			}
			if o == nil {
				break
			}
			m.make(ctx, o)
		}
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		case <-time.After(10 * time.Minute):
		}
	}
}

// make converts one file, reporting progress in Activity.
func (m *Manager) make(ctx context.Context, o *store.Optimized) {
	name := o.Title
	if o.SeriesTitle != "" {
		name = o.SeriesTitle + ": " + o.Title
	}
	task := m.Activity.Start("optimize", fmt.Sprintf("Optimizing %s (%dp)", name, o.Quality))
	err := m.convert(ctx, o, task)
	task.Finish(err)
	if err != nil && ctx.Err() == nil {
		m.log.Warn("optimizing failed", "title", name, "err", err)
	}
}

func (m *Manager) convert(ctx context.Context, o *store.Optimized, task *activity.Handle) error {
	files, err := m.store.ItemFiles(ctx, o.ItemID)
	if err != nil {
		return err
	}
	var f *store.MediaFile
	for i := range files {
		if files[i].ID == o.FileID {
			f = &files[i]
		}
	}
	fail := func(err error) error {
		m.store.SetOptimizeStatus(context.WithoutCancel(ctx), o.FileID, o.Quality, "failed", "", 0, err.Error())
		return err
	}
	ffmpeg := ""
	if m.FFmpeg != nil {
		ffmpeg = m.FFmpeg()
	}
	switch {
	case f == nil || f.Missing:
		return fail(fmt.Errorf("the file isn't there"))
	case ffmpeg == "":
		return fail(fmt.Errorf("FFmpeg wasn't found"))
	}
	if err := m.store.SetOptimizeStatus(ctx, o.FileID, o.Quality, "converting", "", 0, ""); err != nil {
		return err
	}
	cctx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	m.cancel[key(o.FileID, o.Quality)] = cancel
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		delete(m.cancel, key(o.FileID, o.Quality))
		m.mu.Unlock()
	}()

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fail(err)
	}
	out := filepath.Join(m.dir, key(o.FileID, o.Quality)+".mp4")
	tmp := out + ".part"
	args := []string{"-hide_banner", "-loglevel", "error", "-nostats", "-progress", "pipe:1", "-y", "-i", f.Path, "-map", "0:v:0"}
	audio := -1
	for _, s := range f.Streams {
		if s.Kind == "audio" && (audio < 0 || s.IsDefault) {
			audio = s.Index
			if s.IsDefault {
				break
			}
		}
	}
	if audio >= 0 {
		args = append(args, "-map", "0:"+strconv.Itoa(audio))
	}
	scale := fmt.Sprintf("scale=-2:'min(%d,ih)'", o.Quality)
	for _, s := range f.Streams {
		if s.Kind == "video" && (s.ColorTransfer == "smpte2084" || s.ColorTransfer == "arib-std-b67") && m.ToneMap != nil && m.ToneMap() {
			scale = transcode.ToneMapFilter + "," + scale // HDR becomes SDR that looks right everywhere
		}
	}
	kbps := Qualities[o.Quality]
	args = append(args, "-vf", scale+",format=yuv420p", "-c:v", "libx264", "-preset", "faster", "-crf", "21",
		"-maxrate", fmt.Sprintf("%dk", kbps), "-bufsize", fmt.Sprintf("%dk", kbps*2), "-profile:v", "high",
		"-c:a", "aac", "-ac", "2", "-b:a", "160k", "-movflags", "+faststart", "-f", "mp4", tmp)
	cmd := exec.CommandContext(cctx, ffmpeg, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fail(err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return fail(err)
	}
	// FFmpeg's progress lines say how far it's got.
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "out_time_us="); ok && f.DurationMS > 0 {
			if us, err := strconv.ParseInt(v, 10, 64); err == nil {
				task.Progress(int(min(us/1000, f.DurationMS)/1000), int(f.DurationMS/1000))
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		os.Remove(tmp)
		if cctx.Err() != nil && ctx.Err() == nil {
			return nil // deleted while being made
		}
		if ctx.Err() != nil {
			return ctx.Err() // Arc is stopping: it's made again next time
		}
		msg := strings.TrimSpace(stderr.String())
		if i := strings.LastIndexByte(msg, '\n'); i >= 0 {
			msg = msg[i+1:]
		}
		return fail(fmt.Errorf("converting failed: %s", msg))
	}
	if err := os.Rename(tmp, out); err != nil {
		return fail(err)
	}
	var size int64
	if info, err := os.Stat(out); err == nil {
		size = info.Size()
	}
	return m.store.SetOptimizeStatus(ctx, o.FileID, o.Quality, "done", out, size, "")
}
