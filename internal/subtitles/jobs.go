package subtitles

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/store"
)

const (
	subtitleFailureDelay = time.Minute
	maxSubtitleFailures  = 256
	maxSubtitleJobs      = 8
	maxBatchTracks       = 32
	maxCueBytes          = 16 << 20
)

type cueResult struct {
	cues []Cue
	err  error
}
type extraction struct {
	done    chan struct{}
	results map[string]cueResult
	err     error
}
type subtitleFailure struct {
	until time.Time
	err   error
}

// Cache owns bounded extraction jobs. Call Close before closing the server's dependencies.
type Cache struct {
	dir     string
	ffmpeg  func() string
	log     *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	closed  bool
	running map[string]*extraction
	failed  map[string]subtitleFailure
	slots   chan struct{}
	wg      sync.WaitGroup
	now     func() time.Time
}

func NewCache(dir string, ffmpeg func() string) *Cache {
	return NewCacheWithContext(context.Background(), dir, ffmpeg, slog.Default())
}

// NewCacheWithContext binds extraction lifetime to the process, not an HTTP request.
func NewCacheWithContext(ctx context.Context, dir string, ffmpeg func() string, log *slog.Logger) *Cache {
	ctx, cancel := context.WithCancel(ctx)
	if log == nil {
		log = slog.Default()
	}
	return &Cache{dir: dir, ffmpeg: ffmpeg, log: log, ctx: ctx, cancel: cancel, running: map[string]*extraction{}, failed: map[string]subtitleFailure{}, slots: make(chan struct{}, 2), now: time.Now}
}

// Close stops admission, cancels FFmpeg, and waits until every admitted extraction is reaped.
func (c *Cache) Close() { c.mu.Lock(); c.closed = true; c.cancel(); c.mu.Unlock(); c.wg.Wait() }

func cueKey(src string, info os.FileInfo, track string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%s", src, info.Size(), info.ModTime().Unix(), track)))
	return hex.EncodeToString(sum[:12])
}
func cuePath(dir string, fid int64, src string, info os.FileInfo, track string) string {
	return filepath.Join(dir, strconv.FormatInt(fid, 10), cueKey(src, info, track)+".json")
}

// Cues reads a cached track or joins a shared media pass. Caller cancellation only
// ends its wait; process cancellation stops the underlying job and prevents publication.
func (c *Cache) Cues(ctx context.Context, f *store.MediaFile, t Track) ([]Cue, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	if t.Image {
		return nil, errors.New("picture subtitles are shown by burning them into the video")
	}
	src := f.Path
	if t.External {
		src = t.Path
	}
	info, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	path := cuePath(c.dir, f.ID, src, info, t.ID)
	// Keep identifiable legacy-cache adoption; cache entries are scoped by file id.
	legacy := filepath.Join(c.dir, cueKey(src, info, t.ID)+".json")
	if b, err := os.ReadFile(legacy); err == nil {
		var cues []Cue
		if json.Unmarshal(b, &cues) == nil && writeCues(path, cues) == nil {
			_ = os.Remove(legacy)
		}
	}
	if b, err := os.ReadFile(path); err == nil {
		var cues []Cue
		if json.Unmarshal(b, &cues) == nil {
			return cues, nil
		}
	}
	tracks := batchTracks(f, t)
	ids := make([]string, len(tracks))
	for i, tr := range tracks {
		ids[i] = tr.ID
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%d|%d|%s", f.ID, src, info.Size(), info.ModTime().UnixNano(), strings.Join(ids, ","))))
	key := hex.EncodeToString(sum[:])
	c.mu.Lock()
	if c.closed || c.ctx.Err() != nil {
		c.mu.Unlock()
		return nil, context.Canceled
	}
	now := c.now()
	if failure, ok := c.failed[key]; ok && now.Before(failure.until) {
		c.mu.Unlock()
		return nil, failure.err
	}
	job := c.running[key]
	if job == nil {
		// A prior extraction may have published between our first cache read
		// and admission. Recheck before starting another whole-file pass.
		if b, err := os.ReadFile(path); err == nil {
			var cues []Cue
			if json.Unmarshal(b, &cues) == nil {
				c.mu.Unlock()
				return cues, nil
			}
		}
		if len(c.running) >= maxSubtitleJobs {
			c.mu.Unlock()
			return nil, errors.New("subtitle extraction is busy; try again shortly")
		}
		job = &extraction{done: make(chan struct{})}
		c.running[key] = job
		c.wg.Add(1)
		go c.runExtraction(key, job, f.ID, src, info, tracks)
	}
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-job.done:
		if job.err != nil {
			return nil, job.err
		}
		result, ok := job.results[t.ID]
		if !ok {
			return nil, errors.New("subtitle track was not extracted")
		}
		return result.cues, result.err
	}
}

func batchTracks(f *store.MediaFile, requested Track) []Track {
	if requested.External {
		return []Track{requested}
	}
	tracks := []Track{}
	for _, st := range f.Streams {
		if st.Kind == "subtitle" && textCodecs[st.Codec] {
			tracks = append(tracks, Track{ID: "s" + strconv.Itoa(st.Index), StreamIndex: st.Index, Codec: st.Codec})
		}
	}
	slices.SortFunc(tracks, func(a, b Track) int { return a.StreamIndex - b.StreamIndex })
	at := slices.IndexFunc(tracks, func(t Track) bool { return t.ID == requested.ID })
	if at < 0 {
		return []Track{requested}
	}
	start := at / maxBatchTracks * maxBatchTracks
	return tracks[start:min(start+maxBatchTracks, len(tracks))]
}

func (c *Cache) runExtraction(key string, job *extraction, fileID int64, src string, info os.FileInfo, tracks []Track) {
	defer c.wg.Done()
	started := time.Now()
	work, cancel := context.WithTimeout(c.ctx, 10*time.Minute)
	defer cancel()
	reason := "extract_failed"
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-work.Done():
		job.err = work.Err()
	}
	if job.err == nil {
		if tracks[0].External {
			var cues []Cue
			cues, job.err = c.extract(work, src, tracks[0])
			job.results = map[string]cueResult{tracks[0].ID: {cues: cues}}
		} else {
			job.results, job.err = c.extractBatch(work, src, tracks)
		}
	}
	if work.Err() != nil {
		job.err = work.Err()
	}
	// A changing source must never populate cache entries for its old revision.
	if job.err == nil {
		after, err := os.Stat(src)
		if err != nil || after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			job.err = errors.New("source changed while reading subtitles")
			reason = "source_changed"
		}
	}
	if job.err == nil {
		for _, t := range tracks {
			if err := work.Err(); err != nil {
				job.err = err
				break
			}
			result := job.results[t.ID]
			if err := writeCues(cuePath(c.dir, fileID, src, info, t.ID), result.cues); err != nil {
				job.err = errors.New("subtitle cache could not be saved")
				reason = "cache_write_failed"
				break
			}
		}
	}
	if job.err != nil {
		if errors.Is(job.err, context.DeadlineExceeded) {
			reason = "timeout"
		}
		if !errors.Is(job.err, context.Canceled) {
			c.log.Warn("subtitle extraction failed", "file_id", fileID, "tracks", len(tracks), "reason", reason, "took", time.Since(started).Round(time.Millisecond))
		}
		// Keep request errors safe; FFmpeg's stderr can contain paths and source URLs.
		if !errors.Is(job.err, context.Canceled) && !errors.Is(job.err, context.DeadlineExceeded) {
			job.err = errors.New("subtitles could not be extracted; try again in a minute")
		}
	} else {
		c.log.Debug("subtitles extracted", "file_id", fileID, "tracks", len(tracks), "took", time.Since(started).Round(time.Millisecond))
	}
	c.mu.Lock()
	delete(c.running, key)
	if job.err != nil && !errors.Is(job.err, context.Canceled) {
		if len(c.failed) >= maxSubtitleFailures {
			var oldest string
			var until time.Time
			for k, v := range c.failed {
				if oldest == "" || v.until.Before(until) {
					oldest, until = k, v.until
				}
			}
			delete(c.failed, oldest)
		}
		c.failed[key] = subtitleFailure{until: c.now().Add(subtitleFailureDelay), err: job.err}
	} else {
		delete(c.failed, key)
	}
	close(job.done)
	c.mu.Unlock()
}

func (c *Cache) extractBatch(ctx context.Context, src string, tracks []Track) (map[string]cueResult, error) {
	bin := c.ffmpeg()
	if bin == "" {
		return nil, errors.New("FFmpeg was not found")
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(c.dir, "extract-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	args := []string{"-hide_banner", "-nostdin", "-v", "error", "-i", src}
	paths := make([]string, len(tracks))
	for i, tr := range tracks {
		paths[i] = filepath.Join(dir, strconv.Itoa(i)+".vtt")
		args = append(args, "-map", "0:"+strconv.Itoa(tr.StreamIndex), "-c:s", "webvtt", "-fs", strconv.Itoa(maxCueBytes+1), "-f", "webvtt", paths[i])
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	results := make(map[string]cueResult, len(tracks))
	for i, t := range tracks {
		file, err := os.Open(paths[i])
		if err != nil {
			return nil, err
		}
		out, err := io.ReadAll(io.LimitReader(file, maxCueBytes+1))
		file.Close()
		if err != nil {
			return nil, err
		}
		if len(out) > maxCueBytes || !bytesHasVTTPrefix(out) {
			return nil, errors.New("subtitle output was incomplete or too large")
		}
		cues := ParseVTT(out)
		if cues == nil {
			cues = []Cue{}
		}
		results[t.ID] = cueResult{cues: cues}
	}
	return results, nil
}
func bytesHasVTTPrefix(out []byte) bool {
	return strings.HasPrefix(string(out[:min(len(out), 6)]), "WEBVTT")
}

func writeCues(path string, cues []Cue) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.Marshal(cues)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cues-")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
