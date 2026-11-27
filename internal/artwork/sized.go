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

// SnapWidth rounds a requested width up to one of Widths; 0 (no resize) for none or nonsense.
// A request wider than the largest is the largest.
func SnapWidth(requested string) int {
	n, err := strconv.Atoi(requested)
	if err != nil || n <= 0 {
		return 0
	}
	for _, w := range Widths {
		if n <= w {
			return w
		}
	}
	return Widths[len(Widths)-1]
}

// sizedFile is where an image's copy at width lives: beside the originals, under the same key,
// so Prune removes it with its original.
func (c *Cache) sizedFile(key string, width int) string {
	return filepath.Join(c.dir, "sized", strconv.Itoa(width), key[:2], key)
}

// OpenSized returns the image at sourceURL no wider than width: its resized copy, made the first
// time it's asked for and kept, or the original when that's already narrow enough (images are
// never enlarged) or can't be resized. width 0 is the original. resized says which it is: a
// copy's type (JPEG, or PNG with transparency) can differ from the original's. Concurrent
// requests for the same copy share one resize.
func (c *Cache) OpenSized(ctx context.Context, sourceURL string, width int) (f *os.File, resized bool, err error) {
	original, err := c.Open(ctx, sourceURL)
	if err != nil || width <= 0 {
		return original, false, err
	}
	key := Key(sourceURL)
	dst := c.sizedFile(key, width)
	if f, err := os.Open(dst); err == nil {
		original.Close()
		return f, true, nil
	}
	cfg, _, err := image.DecodeConfig(original)
	if err != nil || cfg.Width <= width || cfg.Width*cfg.Height > maxResizePixels {
		// Narrow enough already, too big to decode safely, or a format that can't be decoded:
		// the original it is.
		if _, err := original.Seek(0, io.SeekStart); err != nil {
			original.Close()
			return nil, false, err
		}
		return original, false, nil
	}
	original.Close()
	if err := c.resize(ctx, sourceURL, key, width); err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			c.log.Warn("artwork resize failed; serving the original", "key", key, "width", width, "err", err)
		}
		f, err := c.Open(ctx, sourceURL)
		return f, false, err
	}
	f, err = os.Open(dst)
	return f, err == nil, err
}

// resize makes the copy of key at width, once: a request arriving while it's being made waits
// for it. Resizes share the download slots, so they never swamp the machine.
func (c *Cache) resize(ctx context.Context, sourceURL, key string, width int) error {
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
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
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
