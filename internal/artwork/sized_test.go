package artwork

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"ams/internal/events"
)

func TestSnapWidth(t *testing.T) {
	for in, want := range map[string]int{"": 0, "0": 0, "-5": 0, "abc": 0, "1": 320, "320": 320, "321": 480, "1080": 1280, "1920": 1920, "2000": 3840, "9999": 3840} {
		if got := SnapWidth(in); got != want {
			t.Errorf("SnapWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

func jpegOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestResizeKeepsShapeAndTransparency(t *testing.T) {
	out, err := Resize(jpegOf(t, 1000, 500), 320)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
	if err != nil || format != "jpeg" || cfg.Width != 320 || cfg.Height != 160 {
		t.Fatalf("got %s %dx%d (%v), want jpeg 320x160", format, cfg.Width, cfg.Height, err)
	}

	logo := image.NewNRGBA(image.Rect(0, 0, 800, 200))
	logo.Set(10, 10, color.NRGBA{255, 255, 255, 255})
	var buf bytes.Buffer
	png.Encode(&buf, logo)
	out, err = Resize(buf.Bytes(), 320)
	if err != nil {
		t.Fatal(err)
	}
	img, format, err := image.Decode(bytes.NewReader(out))
	if err != nil || format != "png" {
		t.Fatalf("a transparent logo came back as %s (%v), want png", format, err)
	}
	if _, _, _, a := img.At(300, 100).RGBA(); a != 0 {
		t.Fatalf("a transparent pixel became opaque (alpha %d)", a)
	}
}

func TestOpenSizedKeepsEachCopyAndNeverEnlarges(t *testing.T) {
	var hits atomic.Int32
	big, small := jpegOf(t, 2000, 1000), jpegOf(t, 600, 300)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/jpeg")
		if r.URL.Path == "/small.jpg" {
			w.Write(small)
			return
		}
		w.Write(big)
	}))
	defer srv.Close()
	dir := t.TempDir()
	c := New(dir, slog.New(slog.DiscardHandler))
	c.Events = events.New()
	ctx := context.Background()
	src := srv.URL + "/big.jpg"

	width := func(f *os.File) int {
		t.Helper()
		defer f.Close()
		cfg, _, err := image.DecodeConfig(f)
		if err != nil {
			t.Fatal(err)
		}
		return cfg.Width
	}

	f, resized, err := c.OpenSized(ctx, src, 1280)
	if err != nil || !resized {
		t.Fatalf("first request: resized %v, err %v", resized, err)
	}
	if got := width(f); got != 1280 {
		t.Fatalf("resized to %d, want 1280", got)
	}
	copyPath := c.sizedFile(Key(src), 1280)
	first, err := os.Stat(copyPath)
	if err != nil {
		t.Fatalf("the resized copy wasn't kept: %v", err)
	}

	// A later request, from any app, is served from the kept copy: no resize and no download.
	time.Sleep(10 * time.Millisecond)
	f, resized, err = c.OpenSized(ctx, src, 1280)
	if err != nil || !resized || width(f) != 1280 {
		t.Fatalf("second request: resized %v, err %v", resized, err)
	}
	again, _ := os.Stat(copyPath)
	if !again.ModTime().Equal(first.ModTime()) {
		t.Fatal("the copy was made again instead of reused")
	}
	if hits.Load() != 1 {
		t.Fatalf("provider asked %d times, want once", hits.Load())
	}

	// Never enlarged: a 600px original asked for at 1920 is the original.
	f, resized, err = c.OpenSized(ctx, srv.URL+"/small.jpg", 1920)
	if err != nil || resized || width(f) != 600 {
		t.Fatalf("small original: resized %v, err %v", resized, err)
	}
	// No width: the original.
	f, resized, err = c.OpenSized(ctx, src, 0)
	if err != nil || resized || width(f) != 2000 {
		t.Fatalf("no width: resized %v, err %v", resized, err)
	}

	// Copies go with their original when it's no longer used.
	if _, err := c.Prune([]string{srv.URL + "/small.jpg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(copyPath); !os.IsNotExist(err) {
		t.Fatalf("the resized copy outlived its original: %v", err)
	}
}

func TestOpenSizedServesTheOriginalWhenItCannotResize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		io.WriteString(w, "not really an image")
	}))
	defer srv.Close()
	c := New(t.TempDir(), slog.New(slog.DiscardHandler))
	c.Events = events.New()
	f, resized, err := c.OpenSized(context.Background(), srv.URL+"/odd.jpg", 780)
	if err != nil || resized {
		t.Fatalf("resized %v, err %v", resized, err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "not really an image" {
		t.Fatalf("served %q", data)
	}
	if entries, _ := os.ReadDir(filepath.Join(c.dir, "sized")); len(entries) != 0 {
		t.Fatal("a copy was kept for an image that couldn't be resized")
	}
}
