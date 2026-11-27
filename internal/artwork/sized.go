package artwork

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	_ "image/gif"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Widths are the sizes artwork is resized to on request (?w=). A request is rounded up to the
// next one, so each image has at most a handful of resized copies however many screens ask:
// 1920 and 3840 are a 1080p and a 4K screen's width, the others suit cards and lists.
var Widths = []int{320, 480, 780, 1280, 1920, 3840}

// Images with more pixels than this are served as they are rather than decoded to resize.
const maxResizePixels = 50_000_000

// HeroWidth is the width of a full-screen backdrop on a TV: TV apps draw their interface at
// 1920x1080, even on 4K screens. The Keeper makes this copy of each full-size backdrop ahead of
// time (Warm); every other width is made on request.
const HeroWidth = 1920

// SnapWidth rounds a requested width up to one of Widths; 0 (no resize) for none or nonsense.
// A request wider than the largest is the largest.
func SnapWidth(requested string) int { return snap(Dimension(requested)) }

// Dimension reads a ?w= or ?h= value: a positive number of pixels, or 0 for none or nonsense.
func Dimension(requested string) int {
	n, err := strconv.Atoi(requested)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

func snap(n int) int {
	if n <= 0 {
		return 0
	}
	for _, w := range Widths {
		if n <= w {
			return w
		}
	}
	return Widths[len(Widths)-1]
}

// fitWidth is the copy width for a request of at least w wide and h tall, for an image
// srcW x srcH: wide enough for both, keeping the image's shape, rounded up to one of Widths.
// 0 means the original.
func fitWidth(w, h, srcW, srcH int) int {
	target := w
	if h > 0 && srcW > 0 && srcH > 0 {
		target = max(target, (h*srcW+srcH-1)/srcH)
	}
	return snap(target)
}

// FitWidth is the width OpenSized would make for w and h, from the image's own shape (its cached
// original, downloaded if needed). Without h it's just w rounded up; if the image can't be read
// it falls back to that too.
func (c *Cache) FitWidth(ctx context.Context, sourceURL string, w, h int) int {
	if h <= 0 {
		return snap(w)
	}
	f, err := c.Open(ctx, sourceURL)
	if err != nil {
		return snap(w)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return snap(w)
	}
	return fitWidth(w, h, cfg.Width, cfg.Height)
}

// sizedFile is where an image's copy at width lives: beside the originals, under the same key,
// so Prune removes it with its original.
func (c *Cache) sizedFile(key string, width int) string {
	return filepath.Join(c.dir, "sized", strconv.Itoa(width), key[:2], key)
}

// OpenSized returns the image at sourceURL at least w wide and h tall (either may be 0): its
// resized copy at the width that needs (see fitWidth), made the first time it's asked for and
// kept, or the original when that's already narrow enough (images are never enlarged or cropped)
// or can't be resized. Copies are stored by width only, so h never makes a new kind of copy.
// served is the copy's width, 0 for the original: a copy's type (JPEG, or PNG with
// transparency) can differ from the original's. Concurrent requests for the same copy share one
// resize.
func (c *Cache) OpenSized(ctx context.Context, sourceURL string, w, h int) (f *os.File, served int, err error) {
	original, err := c.Open(ctx, sourceURL)
	if err != nil || (w <= 0 && h <= 0) {
		return original, 0, err
	}
	key := Key(sourceURL)
	width := 0
	if h <= 0 {
		// The fast path: the width is known without reading the image.
		width = snap(w)
		if f, err := os.Open(c.sizedFile(key, width)); err == nil {
			original.Close()
			return f, width, nil
		}
	}
	cfg, _, err := image.DecodeConfig(original)
	if err == nil && h > 0 {
		width = fitWidth(w, h, cfg.Width, cfg.Height)
		if width < cfg.Width {
			if f, err := os.Open(c.sizedFile(key, width)); err == nil {
				original.Close()
				return f, width, nil
			}
		}
	}
	if err != nil || cfg.Width <= width || cfg.Width*cfg.Height > maxResizePixels {
		// Narrow enough already, too big to decode safely, or a format that can't be decoded:
		// the original it is.
		if _, err := original.Seek(0, io.SeekStart); err != nil {
			original.Close()
			return nil, 0, err
		}
		return original, 0, nil
	}
	original.Close()
	if err := c.resize(ctx, key, width, true); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			c.log.Warn("artwork resize failed; serving the original", "key", key, "width", width, "err", err)
		}
		f, err := c.Open(ctx, sourceURL)
		return f, 0, err
	}
	f, err = os.Open(c.sizedFile(key, width))
	if err != nil {
		return nil, 0, err
	}
	return f, width, nil
}

// Warm makes the HeroWidth copy of each of the given images (full-size backdrops, most important
// first) that is cached, wider than HeroWidth and has no copy yet, so the first visit to a title
// doesn't wait for it. It shares resize's jobs, so a request arriving meanwhile waits for the
// same copy, and the download slots, so it never swamps the machine. It stops when ctx is
// cancelled.
func (c *Cache) Warm(ctx context.Context, sourceURLs []string) (resized, failed int) {
	const workers = 4
	todo := make(chan string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for u := range todo {
				key := Key(u)
				err := c.resize(ctx, key, HeroWidth, false)
				if ctx.Err() != nil {
					continue // stopping, not failing
				}
				mu.Lock()
				if err != nil {
					failed++
					if failed <= 3 {
						c.log.Debug("artwork warm-up resize failed", "key", key, "err", err)
					}
				} else {
					resized++
				}
				mu.Unlock()
			}
		}()
	}
	for _, u := range sourceURLs {
		if ctx.Err() != nil {
			break
		}
		if u == "" || !c.needsHero(u) {
			continue
		}
		select {
		case todo <- u:
		case <-ctx.Done():
		}
	}
	close(todo)
	wg.Wait()
	return resized, failed
}

// needsHero reports whether the image's original is cached, wider than HeroWidth and small
// enough to decode, with no HeroWidth copy yet. It reads only the original's header.
func (c *Cache) needsHero(sourceURL string) bool {
	key := Key(sourceURL)
	if _, err := os.Stat(c.sizedFile(key, HeroWidth)); err == nil {
		return false
	}
	f, err := os.Open(c.file(key))
	if err != nil {
		return false
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	return err == nil && cfg.Width > HeroWidth && cfg.Width*cfg.Height <= maxResizePixels
}

// resize makes the copy of key at width, once: a request arriving while it's being made waits
// for it. Resizes share the download slots, so they never swamp the machine. A detached resize
// (a viewer's request) finishes even if the viewer moves on; the Keeper's warm-up is not
// detached, so it stops with the Keeper.
func (c *Cache) resize(ctx context.Context, key string, width int, detach bool) error {
	job := "sized/" + strconv.Itoa(width) + "/" + key
	c.mu.Lock()
	if f := c.inflight[job]; f != nil {
		c.mu.Unlock()
		select {
		case <-f.done:
			return f.err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	f := &fetch{done: make(chan struct{})}
	c.inflight[job] = f
	c.mu.Unlock()

	// As downloads: not tied to the one request, so a viewer moving on still leaves the copy
	// made for the next view.
	parent := ctx
	if detach {
		parent = context.WithoutCancel(ctx)
	}
	rctx, cancel := context.WithTimeout(parent, time.Minute)
	go func() {
		defer cancel()
		select {
		case c.downloadSlots <- struct{}{}:
			start := time.Now()
			f.err = c.resizeToDisk(rctx, key, width)
			<-c.downloadSlots
			if f.err == nil {
				c.log.Debug("artwork resized", "key", key, "width", width, "duration", time.Since(start))
			}
		case <-rctx.Done():
			f.err = rctx.Err()
		}
		c.mu.Lock()
		delete(c.inflight, job)
		close(f.done)
		c.mu.Unlock()
	}()
	select {
	case <-f.done:
		return f.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Cache) resizeToDisk(ctx context.Context, key string, width int) error {
	data, err := os.ReadFile(c.file(key))
	if err != nil {
		return err
	}
	out, err := Resize(data, width)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dst := c.sizedFile(key, width)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), key+".*.tmp")
	if err != nil {
		return err
	}
	_, err = tmp.Write(out)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), dst)
	}
	if err != nil {
		os.Remove(tmp.Name())
	}
	return err
}

// Resize scales an image down to width, keeping its shape: a JPEG for opaque pictures, a PNG
// when it has transparency (logos), so see-through edges stay see-through.
func Resize(data []byte, width int) ([]byte, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	if b.Dx() <= width {
		return nil, fmt.Errorf("image is %dpx wide, not wider than %d", b.Dx(), width)
	}
	height := max(1, (b.Dy()*width+b.Dx()/2)/b.Dx())
	opaque := isOpaque(src)
	var dst draw.Image
	if opaque {
		dst = image.NewRGBA(image.Rect(0, 0, width, height))
	} else {
		dst = image.NewNRGBA(image.Rect(0, 0, width, height))
	}
	// Catmull-Rom over the source area: sharp, without the shimmer of a cheap downscale.
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	var buf bytes.Buffer
	if opaque {
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85})
	} else {
		err = png.Encode(&buf, dst)
	}
	return buf.Bytes(), err
}

func isOpaque(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return o.Opaque()
	}
	return false
}
