// Package optimize makes optimized versions: easy-to-play copies of files (H.264 video and stereo
// AAC in an MP4 that starts playing at once, at most 1080p or 720p), in Arc's data folder, so
// players that can't play a file as it is (or remote ones on a limited connection) needn't wait
// for live conversion. As Plex's Optimize. Media files are only read.
package optimize

import (
	"ams/internal/events"
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/activity"
	"ams/internal/probe"
	"ams/internal/store"
	"ams/internal/transcode"
)

// Qualities are the copies that can be made: the most lines tall, and the video bitrate (kbps).
var Qualities = map[int]int{1080: 8000, 720: 4000}

// Manager makes the queued copies one at a time.
type Manager struct {
	// Events is configured before workers start; invalidations contain no item metadata.
	Events *events.Hub
	store  *store.Store
	dir    string
	log    *slog.Logger

	// Probe inspects only the completed output. Failure leaves the copy usable without
	// a bitrate budget, but ineligible for limited playback.
	Probe    func(context.Context, string) (*probe.Result, error)
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

// Usable checks the actual source and artifact, not just a completed database status. A source
// replacement can precede its next scan, and the artifact may have been removed independently.
func Usable(o *store.Optimized, f *store.MediaFile) bool {
	// Existing profile-5 copies were made without a verified DV-aware colour transform.
	for _, stream := range f.Streams {
		if stream.Kind == "video" && stream.DolbyVision && stream.DolbyVisionProfile == 5 {
			return false
		}
	}

	if o.Status != "done" || o.AudioStream < -1 || !o.MatchesSource(f) {
		return false
	}
	info, err := sourceInfo(o, f)
	if err != nil || info.ModTime().UnixNano() != o.SourceMTimeNS {
		return false
	}
	copy, err := os.Stat(o.Path)
	return err == nil && copy.Mode().IsRegular() && copy.Size() > 0 && copy.Size() == o.Size
}

func sourceInfo(o *store.Optimized, f *store.MediaFile) (os.FileInfo, error) {
	if !o.MatchesSource(f) {
		return nil, errors.New("the source changed; optimize it again after scanning")
	}
	info, err := os.Stat(f.Path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != o.SourceSize || info.ModTime().Unix() != o.SourceMTime {
		return nil, errors.New("the source changed; optimize it again after scanning")
	}
	return info, nil
}

var artifactName = regexp.MustCompile(`^\d+-(720|1080)(-[A-Za-z0-9]+)?\.mp4(\.part)?$`)

func (m *Manager) removeArtifact(path string) {
	if filepath.Dir(filepath.Clean(path)) == filepath.Clean(m.dir) && artifactName.MatchString(filepath.Base(path)) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			m.log.Warn("removing unused optimized copy failed", "err", err)
		}
	}
}

// Prune only Arc's own generated filenames, retaining every registered artifact (including
// older failed copies the owner can remove). Interrupted attempt output is never reused.
func (m *Manager) prune(ctx context.Context) error {
	list, err := m.store.OptimizedVersions(ctx)
	if err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, o := range list {
		if o.Path != "" {
			keep[filepath.Clean(o.Path)] = true
		}
	}
	entries, err := os.ReadDir(m.dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(m.dir, e.Name())
		if !e.IsDir() && artifactName.MatchString(e.Name()) && !keep[filepath.Clean(path)] {
			m.removeArtifact(path)
		}
	}
	return nil
}

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
		m.removeArtifact(path)
	}
	m.Events.Publish(events.Event{Topic: "optimized"})
	return nil
}

// Run makes queued copies until ctx ends.
func (m *Manager) Run(ctx context.Context) {
	if err := m.store.ResetConverting(ctx); err != nil {
		m.log.Warn("resuming optimized versions failed", "err", err)
	} else if err := m.prune(ctx); err != nil {
		m.log.Warn("cleaning interrupted optimized versions failed", "err", err)
	}
	for {
		for ctx.Err() == nil {
			release, ok := m.store.BeginCatalogWork()
			if !ok {
				break
			}
			o, err := m.store.NextOptimize(ctx)
			if err != nil {
				m.log.Warn("listing optimized versions failed", "err", err)
				release()
				break
			}
			if o == nil {
				release()
				break
			}
			m.make(ctx, o)
			release()
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
	defer m.Events.Publish(events.Event{Topic: "optimized"})
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
	files, err := m.store.ItemFiles(cctx, o.ItemID)
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
		_, saveErr := m.store.FailOptimize(context.WithoutCancel(ctx), o, err.Error())
		return errors.Join(err, saveErr)
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
	for _, stream := range f.Streams {
		if stream.Kind == "video" && stream.DolbyVision && stream.DolbyVisionProfile == 5 {
			return fail(fmt.Errorf("Dolby Vision profile 5 cannot be safely re-encoded into an optimized copy; use the original on a compatible player"))
		}
	}
	before, err := sourceInfo(o, f)
	if err != nil {
		return fail(err)
	}
	claimed, err := m.store.ClaimOptimize(cctx, o, before.ModTime().UnixNano())
	if err != nil {
		return err
	}
	if !claimed {
		return nil // deleted, already claimed, or superseded
	}
	m.Events.Publish(events.Event{Topic: "optimized", LibraryID: f.LibraryID})

	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fail(err)
	}
	// Attempt-specific paths keep a superseded worker from overwriting a newer artifact.
	out := filepath.Join(m.dir, key(o.FileID, o.Quality)+"-"+o.JobID+".mp4")
	tmp := out + ".part"
	defer m.removeArtifact(tmp)
	args := []string{"-hide_banner", "-loglevel", "error", "-nostats", "-progress", "pipe:1", "-y", "-i", f.Path, "-map", "0:v:0"}
	audio := -1
	o.AudioStream, o.AudioLanguage, o.AudioTitle = -1, "", ""
	for _, s := range f.Streams {
		if s.Kind == "audio" && (audio < 0 || s.IsDefault) {
			audio = s.Index
			o.AudioStream, o.AudioLanguage, o.AudioTitle = s.Index, s.Language, s.Title
			if s.IsDefault {
				break
			}
		}
	}
	if audio >= 0 {
		args = append(args, "-map", "0:"+strconv.Itoa(audio),
			"-metadata:s:a:0", "language="+o.AudioLanguage, "-metadata:s:a:0", "title="+o.AudioTitle,
			"-disposition:a:0", "default")
	}
	scale := transcode.FitScaleFilter(o.Quality)
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
	if cctx.Err() != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return nil
	}
	after, err := sourceInfo(o, f)
	if err != nil {
		return fail(err)
	}
	if !os.SameFile(before, after) || after.ModTime().UnixNano() != o.SourceMTimeNS {
		return fail(errors.New("the source changed while making this copy; optimize it again"))
	}
	if err := os.Rename(tmp, out); err != nil {
		return fail(err)
	}
	info, err := os.Stat(out)
	if err != nil || info.Size() == 0 {
		m.removeArtifact(out)
		return fail(errors.New("the converted copy is unavailable"))
	}
	o.DurationMS, o.Bitrate = 0, 0
	if m.Probe != nil {
		measured, err := m.Probe(cctx, out)
		if err != nil {
			m.log.Warn("measuring optimized copy failed; limited playback will convert instead", "err", err)
		} else if measured != nil && measured.DurationMS > 0 {
			o.DurationMS = measured.DurationMS
			// Include audio and container overhead; never substitute source bitrate or nominal preset.
			o.Bitrate = max(measured.Bitrate, int64(math.Ceil(float64(info.Size())*8000/float64(o.DurationMS))))
		}
	}
	finished, err := m.store.FinishOptimize(cctx, o, out, info.Size())
	if err != nil || !finished {
		m.removeArtifact(out)
		if ctx.Err() != nil {
			return ctx.Err() // leave this interrupted attempt resumable
		}
		if cctx.Err() != nil {
			return nil // deleted while finishing
		}
		if err != nil {
			return fail(err)
		}
		// A missing catalog source can invalidate completion without changing the attempt ID.
		// Mark that attempt failed, but never replace a newer queued/completed generation.
		if _, err := m.store.FailOptimize(context.WithoutCancel(ctx), o, "The source or request changed. Optimize it again."); err != nil {
			return err
		}
		return nil // an older completion never changes the newer request
	}
	if o.Path != "" && o.Path != out {
		m.removeArtifact(o.Path)
	}
	return nil
}
