package palette

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func quadrantPNG(t *testing.T, colors [4]color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			i := 0
			if x >= 32 {
				i++
			}
			if y >= 32 {
				i += 2
			}
			img.SetNRGBA(x, y, colors[i])
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var quadrants = [4]color.NRGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 128, 0, 255}}

func TestRegionalColorsAndTransparency(t *testing.T) {
	data := quadrantPNG(t, quadrants)
	got, err := Extract(data)
	if err != nil {
		t.Fatal(err)
	}
	want := Colors{"#ff0000", "#00ff00", "#0000ff", "#ff8000"}
	if got.Colors != want {
		t.Fatalf("colors: %+v", got.Colors)
	}
	again, err := Extract(data)
	if err != nil || again != got {
		t.Fatalf("not stable: %+v %v", again, err)
	}
	changed := quadrants
	changed[0] = color.NRGBA{0, 128, 255, 255}
	next, err := Extract(quadrantPNG(t, changed))
	if err != nil || next.Version == got.Version {
		t.Fatalf("image change not versioned: %+v %v", next, err)
	}
	var transparent [4]color.NRGBA
	if _, err := Extract(quadrantPNG(t, transparent)); !errors.Is(err, ErrNoColors) {
		t.Fatalf("transparent: %v", err)
	}
	transparent[0] = color.NRGBA{255, 0, 0, 128}
	partial, err := Extract(quadrantPNG(t, transparent))
	if err != nil || partial.Colors != (Colors{"#ff0000", "#ff0000", "#ff0000", "#ff0000"}) {
		t.Fatalf("transparent regions added black: %+v %v", partial, err)
	}
}
func TestDecodeBoundsAndMalformedInput(t *testing.T) {
	if _, err := Extract([]byte("not an image")); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("corrupt: %v", err)
	}
	if _, err := Extract(make([]byte, MaxBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("byte cap: %v", err)
	}
	huge := quadrantPNG(t, quadrants)
	binary.BigEndian.PutUint32(huge[16:20], 100000)
	binary.BigEndian.PutUint32(huge[20:24], 100000)
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	if _, err := Extract(huge); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("pixel cap: %v", err)
	}
}
func TestCacheReuseRestartAndFileReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fanart.png")
	data := quadrantPNG(t, quadrants)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cache := New(dir)
	read := func(c *Cache, flag int) Result {
		t.Helper()
		f, err := os.OpenFile(path, flag, 0600)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r, err := c.Get(t.Context(), f)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	first := read(cache, os.O_RDONLY)
	// Write-only handles deliberately cannot be decoded: success demonstrates actual cache reuse.
	if got := read(cache, os.O_WRONLY); got != first {
		t.Fatalf("memory cache: %+v", got)
	}
	if got := read(New(dir), os.O_WRONLY); got != first {
		t.Fatalf("persistent cache: %+v", got)
	}
	data = quadrantPNG(t, [4]color.NRGBA{{0, 0, 0, 255}, {0, 0, 0, 255}, {0, 0, 0, 255}, {0, 0, 0, 255}})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	next := read(cache, os.O_RDONLY)
	if next.Version == first.Version || next.Colors.TopLeft != "#000000" {
		t.Fatalf("stale replacement: %+v", next)
	}
}
func TestCacheConcurrentAndCancelledWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "art.png")
	if err := os.WriteFile(path, quadrantPNG(t, quadrants), 0600); err != nil {
		t.Fatal(err)
	}
	cache := New(t.TempDir())
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			f, err := os.Open(path)
			if err != nil {
				t.Error(err)
				return
			}
			defer f.Close()
			r, err := cache.Get(t.Context(), f)
			if err != nil || r.Colors.TopLeft != "#ff0000" {
				t.Errorf("concurrent: %+v %v", r, err)
			}
		})
	}
	wg.Wait()
	entries, err := os.ReadDir(cache.dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache not coalesced: %d %v", len(entries), err)
	}
	other := New(t.TempDir())
	other.slots <- struct{}{}
	other.slots <- struct{}{}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := other.Get(ctx, f); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}
func TestDiskBudget(t *testing.T) {
	dir := t.TempDir()
	cache := New(dir)
	for i := 0; i < diskEntries+3; i++ {
		name := filepath.Join(dir, time.Unix(int64(i), 0).Format("150405")+".json")
		if err := os.WriteFile(name, []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cache.prune()
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != diskEntries {
		t.Fatalf("disk bound: %d %v", len(entries), err)
	}
}

func TestWebPDecoder(t *testing.T) {
	data, err := os.ReadFile("testdata/blue.webp")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Extract(data)
	if err != nil || got.Colors.TopLeft != "#0000ff" {
		t.Fatalf("generated WebP: %+v %v", got, err)
	}
}

func TestJPEGDecoder(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.SetNRGBA(x, y, color.NRGBA{255, 0, 0, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	got, err := Extract(buf.Bytes())
	if err != nil || got.Colors.TopLeft != "#fe0000" {
		t.Fatalf("JPEG: %+v %v", got, err)
	}
}
