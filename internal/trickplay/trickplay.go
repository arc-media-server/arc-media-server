// Package trickplay makes the small pictures shown while dragging along the player's timeline:
// a thumbnail every 10 seconds, packed into sheets of 10×10. A background worker makes them one
// file at a time, newest titles first, and a title someone starts playing jumps the queue. Only
// keyframes are decoded, on one thread, so it's quick and stays out of the way. It's turned on per
// library (off by default).
package trickplay

import (
	"ams/internal/activity"
	"ams/internal/background"
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/store"
	"ams/internal/transcode"
)

const (
	IntervalMS        = 10_000
	ThumbWidth        = 320
	Columns           = 10
	Rows              = 10
	maxUrgentRequests = 256
)

// Info describes a file's thumbnails.
type Info struct {
	IntervalMS int64 `json:"interval_ms"`
	Width      int   `json:"width"`
	Height     int   `json:"height"`
	Columns    int   `json:"columns"`
	Rows       int   `json:"rows"`
	// How many thumbnails, and sheets of them (files 0.jpg, 1.jpg, ...).
	Count  int `json:"count"`
	Sheets int `json:"sheets"`
}

type Manager struct {
	dir    string
	store  *store.Store
	ffmpeg func() string
	log    *slog.Logger

	wake   chan struct{}
	mu     sync.Mutex
	urgent []int64
	failed map[int64]bool
	// One BIF file is cut at a time (they're made on request and kept).
	bifMu sync.Mutex

	// Activity, if set, is told while previews are being made.
	Activity   *activity.Tracker
	Background *background.Manager
	// Allowed, if set, says whether previews may be made now for files nobody asked for yet.
	Allowed func() bool
}

func New(dir string, st *store.Store, ffmpeg func() string, log *slog.Logger) *Manager {
	// Half-made thumbnails from a run that was interrupted.
	if tmps, err := filepath.Glob(filepath.Join(dir, "*.tmp")); err == nil {
		for _, t := range tmps {
			os.RemoveAll(t)
		}
	}
	return &Manager{dir: dir, store: st, ffmpeg: ffmpeg, log: log,
		wake: make(chan struct{}, 1), failed: map[int64]bool{}}
}

func (m *Manager) fileDir(fileID int64) string {
	return filepath.Join(m.dir, strconv.FormatInt(fileID, 10))
}

// Info returns a file's thumbnails, or nil if they haven't been made.
func (m *Manager) Info(fileID int64) *Info {
	b, err := os.ReadFile(filepath.Join(m.fileDir(fileID), "info.json"))
	if err != nil {
		return nil
	}
	var info Info
	if json.Unmarshal(b, &info) != nil {
		return nil
	}
	return &info
}

// Sheet returns the path of one sheet of a file's thumbnails.
func (m *Manager) Sheet(fileID int64, n int) (string, bool) {
	info := m.Info(fileID)
	if info == nil || n < 0 || n >= info.Sheets {
		return "", false
	}
	return filepath.Join(m.fileDir(fileID), strconv.Itoa(n)+".jpg"), true
}

// Request moves a file to the front of the queue (someone is watching it).
func (m *Manager) Request(fileID int64) {
	if fileID <= 0 || m.Info(fileID) != nil {
		return
	}
	m.mu.Lock()
	index := slices.Index(m.urgent, fileID)
	if index < 0 {
		// Urgency is a hint, not durable work: a normal pass still discovers all eligible files.
		// Favor current viewers and forget the oldest hint when generation is disabled/unavailable.
		if len(m.urgent) == maxUrgentRequests {
			m.urgent = m.urgent[:maxUrgentRequests-1]
		}
		index = len(m.urgent)
		m.urgent = append(m.urgent, 0)
	}
	copy(m.urgent[1:index+1], m.urgent[:index])
	m.urgent[0] = fileID
	m.mu.Unlock()
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Wake starts a pass over the library (e.g. after a scan found new files).
func (m *Manager) Wake() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Run makes thumbnails until ctx ends.
func (m *Manager) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-m.wake:
	case <-time.After(30 * time.Second): // let start-up scans go first
	}
	for {
		if m.ffmpeg() != "" {
			m.pass(ctx)
		} else {
			m.Background.AbandonTurn("previews")
		}
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
		case <-time.After(30 * time.Minute):
		}
	}
}

// pass makes thumbnails for urgent files, then everything still without them.
func (m *Manager) pass(ctx context.Context) {
	if m.Background != nil && m.Background.Mode("previews") == "off" {
		return
	}
	files, err := m.store.VideoFiles(ctx)
	if err != nil {
		m.log.Warn("listing files for scrubbing previews failed", "err", err)
		return
	}
	byID := make(map[int64]store.VideoFile, len(files))
	for _, f := range files {
		byID[f.ID] = f
	}
	visited := map[int64]bool{}
	next := func() (store.VideoFile, bool) {
		m.mu.Lock()
		defer m.mu.Unlock()
		for len(m.urgent) > 0 {
			id := m.urgent[0]
			m.urgent = m.urgent[1:]
			if f, ok := byID[id]; ok && m.Background.CanStart("previews", f.ID, f.ProbedAt) && !m.failed[id] && !visited[id] && m.Info(id) == nil {
				return f, true
			}
		}
		if m.Background == nil && m.Allowed != nil && !m.Allowed() {
			return store.VideoFile{}, false
		}
		for _, f := range files {
			if m.Background.CanStart("previews", f.ID, f.ProbedAt) && !m.failed[f.ID] && !visited[f.ID] && m.Info(f.ID) == nil {
				return f, true
			}
		}
		return store.VideoFile{}, false
	}
	var task *activity.Handle
	done, todo, failed := 0, 0, 0
	defer func() {
		if task != nil {
			m.log.Info("scrubbing previews", "made", done-failed, "failed", failed)
		}
		task.Finish(nil)
	}()
	for ctx.Err() == nil {
		f, ok := next()
		if !ok {
			if m.Background != nil {
				m.Background.CompletePass("previews")
			}
			return
		}
		visited[f.ID] = true
		work, finish, allowed := m.Background.BeginPreview(ctx, f.ID)
		if !allowed {
			if m.Background.Deferred("previews") {
				return
			}
			continue
		}
		if task == nil {
			for _, f := range files {
				if m.Background.CanStart("previews", f.ID, f.ProbedAt) && !m.failed[f.ID] && !visited[f.ID] && m.Info(f.ID) == nil {
					todo++
				}
			}
			task = m.Activity.Start("previews", "Making scrubbing previews")
		}
		done++
		task.Progress(done, max(todo, done))
		task.Detail(filepath.Base(f.Path))
		name, _ := m.store.FileActivityLabel(ctx, f.ID)
		if name == "" {
			name = filepath.Base(f.Path)
		}
		op := task.Work("Making previews", name, filepath.Base(f.Path))
		start := time.Now()
		err := m.make(work, f)
		op.Finish()
		paused := work.Err() != nil
		finish(err)
		if paused {
			return
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.mu.Lock()
			if m.Background == nil {
				m.failed[f.ID] = true
			}
			m.mu.Unlock()
			failed++
			m.log.Warn("couldn't make scrubbing previews", "title", name, "file", filepath.Base(f.Path), "err", err)
			continue
		}
		m.log.Debug("made scrubbing previews", "file", filepath.Base(f.Path), "took", time.Since(start).Round(time.Millisecond))
	}
}

func (m *Manager) make(ctx context.Context, f store.VideoFile) error {
	if f.Width <= 0 || f.Height <= 0 {
		return fmt.Errorf("no picture size")
	}
	w := ThumbWidth
	h := int(float64(w)*float64(f.Height)/float64(f.Width)/2+0.5) * 2
	hdr := f.ColorTransfer == "smpte2084" || f.ColorTransfer == "arib-std-b67"
	frames, err := m.keyframes(ctx, f.Path, w, h, hdr)
	if err != nil && hdr {
		// No tone mapping in this FFmpeg: washed-out previews are better than none.
		frames, err = m.keyframes(ctx, f.Path, w, h, false)
	}
	if err != nil {
		return err
	}
	if len(frames) == 0 {
		return fmt.Errorf("FFmpeg found no pictures")
	}
	slices.SortStableFunc(frames, func(a, b frame) int { return cmp.Compare(a.ms, b.ms) })

	tmp := m.fileDir(f.ID) + ".tmp"
	os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Each slot gets the keyframe nearest its time.
	count := max(1, int((f.DurationMS+IntervalMS-1)/IntervalMS))
	perSheet := Columns * Rows
	sheets := (count + perSheet - 1) / perSheet
	decoded := map[int]image.Image{}
	for s := 0; s < sheets; s++ {
		sheet := image.NewRGBA(image.Rect(0, 0, Columns*w, Rows*h))
		for slot := s * perSheet; slot < min(count, (s+1)*perSheet); slot++ {
			i := nearest(frames, int64(slot)*IntervalMS)
			img, ok := decoded[i]
			if !ok {
				if img, err = jpeg.Decode(bytes.NewReader(frames[i].jpeg)); err != nil {
					return err
				}
				decoded = map[int]image.Image{i: img} // only the last is reused (slots go in order)
			}
			pos := slot - s*perSheet
			at := image.Pt((pos%Columns)*w, (pos/Columns)*h)
			draw.Draw(sheet, image.Rectangle{Min: at, Max: at.Add(image.Pt(w, h))}, img, image.Point{}, draw.Src)
		}
		out, err := os.Create(filepath.Join(tmp, strconv.Itoa(s)+".jpg"))
		if err != nil {
			return err
		}
		err = jpeg.Encode(out, sheet, &jpeg.Options{Quality: 72})
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	info := Info{IntervalMS: IntervalMS, Width: w, Height: h, Columns: Columns, Rows: Rows, Count: count, Sheets: sheets}
	b, _ := json.Marshal(info)
	if err := os.WriteFile(filepath.Join(tmp, "info.json"), b, 0o644); err != nil {
		return err
	}
	final := m.fileDir(f.ID)
	os.RemoveAll(final)
	return os.Rename(tmp, final)
}

// frame is one decoded keyframe, small and JPEG-compressed to keep memory down.
type frame struct {
	ms   int64
	jpeg []byte
}

var showinfoRe = regexp.MustCompile(`\bn:\s*\d+\s+pts:\s*-?\d+\s+pts_time:(-?[\d.]+)`)

// keyframes decodes only a file's keyframes, at w×h. Decoders can hand them back out of order
// (HEVC does), so each comes with its own time, read from FFmpeg's showinfo log.
func (m *Manager) keyframes(ctx context.Context, path string, w, h int, toneMap bool) ([]frame, error) {
	vf := fmt.Sprintf("scale=%d:%d,showinfo", w, h)
	if toneMap {
		vf = transcode.ToneMapFilter + "," + vf
	}
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.ffmpeg(), "-hide_banner", "-nostdin", "-v", "info",
		"-threads", "1", "-skip_frame", "nokey", "-i", path,
		"-map", "0:v:0", "-an", "-sn", "-dn", "-filter_threads", "1",
		"-vf", vf, "-fps_mode", "passthrough",
		"-f", "rawvideo", "-pix_fmt", "rgb24", "pipe:1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// Each picture's time is logged before the picture is written.
	times := make(chan int64, 64)
	var lastErr string
	go func() {
		defer close(times)
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if m := showinfoRe.FindStringSubmatch(line); m != nil {
				sec, _ := strconv.ParseFloat(m[1], 64)
				times <- int64(sec * 1000)
			} else if strings.Contains(strings.ToLower(line), "error") {
				lastErr = line
			}
		}
	}()

	var frames []frame
	buf := make([]byte, w*h*3)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	var enc bytes.Buffer
	for {
		if _, err := io.ReadFull(stdout, buf); err != nil {
			break
		}
		ms, ok := <-times
		if !ok {
			break
		}
		for i, j := 0, 0; i < len(buf); i, j = i+3, j+4 {
			img.Pix[j], img.Pix[j+1], img.Pix[j+2], img.Pix[j+3] = buf[i], buf[i+1], buf[i+2], 255
		}
		enc.Reset()
		if err := jpeg.Encode(&enc, img, &jpeg.Options{Quality: 80}); err != nil {
			return nil, err
		}
		frames = append(frames, frame{ms: ms, jpeg: bytes.Clone(enc.Bytes())})
	}
	io.Copy(io.Discard, stdout)
	for range times {
	}
	if err := cmd.Wait(); err != nil && len(frames) == 0 {
		return nil, fmt.Errorf("%w: %s", err, lastErr)
	}
	return frames, nil
}

// nearest is the index of the frame closest to ms (frames are sorted by time).
func nearest(frames []frame, ms int64) int {
	i, _ := slices.BinarySearchFunc(frames, ms, func(f frame, t int64) int { return cmp.Compare(f.ms, t) })
	if i == len(frames) {
		return i - 1
	}
	if i > 0 && ms-frames[i-1].ms <= frames[i].ms-ms {
		return i - 1
	}
	return i
}

// Forget deletes a file's thumbnails (e.g. when the file changed).
func (m *Manager) Forget(fileID int64) {
	os.RemoveAll(m.fileDir(fileID))
}
