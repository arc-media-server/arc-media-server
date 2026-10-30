// Package palette derives small, versioned colour palettes from cached artwork. It never reads media.
package palette

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
)

const Algorithm = "arc-regional-v1"
const MaxBytes = 25 << 20
const maxPixels = 12_000_000
const memoryEntries = 256
const diskEntries = 1024

var ErrTooLarge = errors.New("image exceeds palette size limits")
var ErrUnsupported = errors.New("image cannot be decoded for palette extraction")
var ErrNoColors = errors.New("image has no visible pixels")

// Colors are opaque CSS hex colours in decoded sRGB pixel space, arranged as viewed in the image.
type Colors struct {
	TopLeft     string `json:"top_left"`
	TopRight    string `json:"top_right"`
	BottomLeft  string `json:"bottom_left"`
	BottomRight string `json:"bottom_right"`
}
type Result struct {
	Algorithm string `json:"algorithm"`
	Version   string `json:"version"`
	Colors    Colors `json:"colors"`
}
type entry struct {
	result Result
	info   os.FileInfo
}

// Cache is bounded in memory, on disk and in concurrent decodes. A stat fingerprint avoids
// reading/decoding an unchanged file on every view; version is a digest of the actual image bytes.
type flight struct {
	done   chan struct{}
	result Result
	err    error
}

type Cache struct {
	dir      string
	mu       sync.Mutex
	inflight map[string]*flight
	diskMu   sync.Mutex
	entries  map[string]entry
	order    []string
	slots    chan struct{}
}

func New(dir string) *Cache {
	return &Cache{dir: dir, entries: make(map[string]entry), inflight: make(map[string]*flight), slots: make(chan struct{}, 2)}
}

func (c *Cache) Get(ctx context.Context, f *os.File) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	info, err := f.Stat()
	if err != nil {
		return Result{}, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return Result{}, ErrUnsupported
	}
	if info.Size() > MaxBytes {
		return Result{}, ErrTooLarge
	}
	fingerprint := fmt.Sprintf("%s\x00%s\x00%d\x00%d", Algorithm, f.Name(), info.Size(), info.ModTime().UnixNano())
	sum := sha256.Sum256([]byte(fingerprint))
	key := hex.EncodeToString(sum[:])
	lookup := func() (Result, bool) {
		c.mu.Lock()
		defer c.mu.Unlock()
		e, ok := c.entries[key]
		return e.result, ok && os.SameFile(e.info, info)
	}
	if result, ok := lookup(); ok {
		return result, nil
	}
	select {
	case c.slots <- struct{}{}:
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	held := true
	defer func() {
		if held {
			<-c.slots
		}
	}()
	if result, ok := lookup(); ok {
		return result, nil
	}

	c.mu.Lock()
	if e, ok := c.entries[key]; ok && os.SameFile(e.info, info) {
		c.mu.Unlock()
		return e.result, nil
	}
	if pending := c.inflight[key]; pending != nil {
		c.mu.Unlock()
		// Do not hold a decode slot while following the already-running computation.
		<-c.slots
		held = false
		select {
		case <-pending.done:
			return pending.result, pending.err
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
	}
	pending := &flight{done: make(chan struct{})}
	c.inflight[key] = pending
	c.mu.Unlock()
	result, err := c.generate(ctx, f, info, key)
	c.mu.Lock()
	if err == nil {
		c.remember(key, entry{result, info})
	}
	pending.result, pending.err = result, err
	delete(c.inflight, key)
	close(pending.done)
	c.mu.Unlock()
	return result, err
}

func (c *Cache) generate(ctx context.Context, f *os.File, info os.FileInfo, key string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	cacheFile := filepath.Join(c.dir, key+".json")
	var result Result
	var err error
	if data, err := readCache(cacheFile); err == nil && len(data) < 2048 && json.Unmarshal(data, &result) == nil && valid(result) {
		return result, nil
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return Result{}, err
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return Result{}, err
	}
	if len(data) > MaxBytes {
		return Result{}, ErrTooLarge
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	result, err = Extract(data)
	if err != nil {
		return Result{}, err
	}
	after, err := f.Stat()
	if err != nil {
		return Result{}, err
	}
	if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
		return Result{}, errors.New("artwork changed during palette extraction")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	c.diskMu.Lock()
	defer c.diskMu.Unlock()
	if err = os.MkdirAll(c.dir, 0o755); err != nil {
		return Result{}, err
	}
	raw, _ := json.Marshal(result)
	tmp, err := os.CreateTemp(c.dir, "palette-*.tmp")
	if err != nil {
		return Result{}, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(raw); err != nil {
		tmp.Close()
		return Result{}, err
	}
	if err = tmp.Close(); err != nil {
		return Result{}, err
	}
	if err = os.Rename(name, cacheFile); err != nil {
		return Result{}, err
	}
	c.prune()
	return result, nil
}
func (c *Cache) remember(key string, e entry) {
	if _, ok := c.entries[key]; !ok {
		c.order = append(c.order, key)
	}
	c.entries[key] = e
	for len(c.order) > memoryEntries {
		delete(c.entries, c.order[0])
		c.order = c.order[1:]
	}
}
func (c *Cache) prune() {
	files, err := os.ReadDir(c.dir)
	if err != nil || len(files) <= diskEntries {
		return
	}
	type aged struct {
		name string
		at   time.Time
	}
	var list []aged
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if st, err := f.Info(); err == nil {
			list = append(list, aged{f.Name(), st.ModTime()})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].at.Before(list[j].at) })
	for i := 0; i < len(list)-diskEntries; i++ {
		os.Remove(filepath.Join(c.dir, list[i].name))
	}
}
func readCache(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 2049))
}

func valid(r Result) bool {
	if r.Algorithm != Algorithm || len(r.Version) != 64 {
		return false
	}
	if _, err := hex.DecodeString(r.Version); err != nil {
		return false
	}
	for _, s := range []string{r.Colors.TopLeft, r.Colors.TopRight, r.Colors.BottomLeft, r.Colors.BottomRight} {
		if len(s) != 7 || s[0] != '#' {
			return false
		}
		if _, err := hex.DecodeString(s[1:]); err != nil {
			return false
		}
	}
	return true
}

type average struct{ r, g, b, w uint64 }

func (a *average) add(r, g, b, alpha uint32) {
	maxC, minC := max(r, g, b), min(r, g, b)
	weight := uint64(alpha) * uint64(64+maxC-minC)
	a.r += uint64(r) * weight
	a.g += uint64(g) * weight
	a.b += uint64(b) * weight
	a.w += weight
}
func (a average) color() string { return fmt.Sprintf("#%02x%02x%02x", a.r/a.w, a.g/a.w, a.b/a.w) }

// Extract samples at most 4096 points. Alpha and chroma weight regional averages so transparent
// pixels do not add black and small colourful areas are not overwhelmed by neutral backgrounds.
// This is Arc's own algorithm; it does not claim to reproduce Plex's private renderer.
func Extract(data []byte) (Result, error) {
	if len(data) > MaxBytes {
		return Result{}, ErrTooLarge
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return Result{}, ErrUnsupported
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return Result{}, ErrTooLarge
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}, ErrUnsupported
	}
	bounds := img.Bounds()
	var regions [4]average
	var all average
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			r, g, b, a := img.At(bounds.Min.X+(2*x+1)*bounds.Dx()/128, bounds.Min.Y+(2*y+1)*bounds.Dy()/128).RGBA()
			if a < 6554 {
				continue
			}
			r = (r * 65535 / a) >> 8
			g = (g * 65535 / a) >> 8
			b = (b * 65535 / a) >> 8
			quadrant := 0
			if x >= 32 {
				quadrant++
			}
			if y >= 32 {
				quadrant += 2
			}
			regions[quadrant].add(r, g, b, a)
			all.add(r, g, b, a)
		}
	}
	if all.w == 0 {
		return Result{}, ErrNoColors
	}
	for i := range regions {
		if regions[i].w == 0 {
			regions[i] = all
		}
	}
	digest := sha256.New()
	digest.Write([]byte(Algorithm + "\x00"))
	digest.Write(data)
	return Result{Algorithm: Algorithm, Version: hex.EncodeToString(digest.Sum(nil)), Colors: Colors{regions[0].color(), regions[1].color(), regions[2].color(), regions[3].color()}}, nil
}
