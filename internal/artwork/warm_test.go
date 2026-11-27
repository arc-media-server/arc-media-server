package artwork

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"ams/internal/events"
)

// plainJPEG is a quick single-colour JPEG of any size (jpegOf's gradient is slow at 4K).
func plainJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// warmProvider serves a large full-size backdrop, a full-size backdrop no wider than 1920, a
// card backdrop and a large poster.
func warmProvider(t *testing.T) (*httptest.Server, map[string]string) {
	t.Helper()
	files := map[string][]byte{
		"/original/big.jpg":   plainJPEG(t, 3840, 2160),
		"/original/small.jpg": plainJPEG(t, 1920, 1080),
		"/w1280/card.jpg":     plainJPEG(t, 1280, 720),
		"/w500/poster.jpg":    plainJPEG(t, 2000, 3000),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(files[r.URL.Path])
	}))
	t.Cleanup(srv.Close)
	return srv, map[string]string{
		"big":    srv.URL + "/original/big.jpg",
		"small":  srv.URL + "/original/small.jpg",
		"card":   srv.URL + "/w1280/card.jpg",
		"poster": srv.URL + "/w500/poster.jpg",
	}
}

func newWarmCache(t *testing.T) *Cache {
	t.Helper()
	c := New(t.TempDir(), slog.New(slog.DiscardHandler))
	c.Events = events.New()
	return c
}

func TestKeeperMakesTheHeroCopyOfFullSizeBackdropsOnly(t *testing.T) {
	_, src := warmProvider(t)
	c := newWarmCache(t)
	passes := make(chan struct{}, 4)
	k := c.Keep(t.Context(), func(context.Context) (Images, error) {
		passes <- struct{}{}
		return Images{
			Sources: []string{src["poster"], src["card"], src["big"], src["small"]},
			Warm:    []string{src["big"], src["small"]},
		}, nil
	})
	k.Trigger()
	<-passes
	hero := c.sizedFile(Key(src["big"]), HeroWidth)
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(hero); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the large backdrop's 1920 copy wasn't made")
		}
		time.Sleep(20 * time.Millisecond)
	}
	k.Close() // joins the pass

	f, err := os.Open(hero)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil || cfg.Width != HeroWidth || cfg.Height != 1080 {
		t.Fatalf("hero copy %dx%d (%v), want 1920x1080", cfg.Width, cfg.Height, err)
	}
	for _, name := range []string{"small", "card", "poster"} {
		for _, w := range Widths {
			if _, err := os.Stat(c.sizedFile(Key(src[name]), w)); err == nil {
				t.Errorf("%s got a %d copy; only large full-size backdrops are warmed, at 1920", name, w)
			}
		}
	}
	for _, w := range Widths {
		if _, err := os.Stat(c.sizedFile(Key(src["big"]), w)); err == nil && w != HeroWidth {
			t.Errorf("the backdrop got a %d copy; only 1920 is warmed", w)
		}
	}

	// A second pass has nothing to do.
	before, _ := os.Stat(hero)
	if resized, failed := c.Warm(t.Context(), []string{src["big"], src["small"]}); resized+failed != 0 {
		t.Fatalf("second pass resized %d, failed %d; want no work", resized, failed)
	}
	after, _ := os.Stat(hero)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("the hero copy was made again")
	}
}

func TestWarmSkipsOriginalsNotYetCached(t *testing.T) {
	_, src := warmProvider(t)
	c := newWarmCache(t)
	if resized, failed := c.Warm(t.Context(), []string{src["big"]}); resized+failed != 0 {
		t.Fatalf("resized %d, failed %d: warming must never download", resized, failed)
	}
}

func TestWarmStopsOnCancellation(t *testing.T) {
	_, src := warmProvider(t)
	c := newWarmCache(t)
	if _, failed := c.Prefetch(t.Context(), []string{src["big"]}); failed != 0 {
		t.Fatal("prefetch failed")
	}
	// Every slot busy, as if four downloads were running: the warm-up waits for one, then is
	// cancelled while waiting.
	for range cap(c.downloadSlots) {
		c.downloadSlots <- struct{}{}
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan [2]int)
	go func() {
		r, f := c.Warm(ctx, []string{src["big"]})
		done <- [2]int{r, f}
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case got := <-done:
		if got != [2]int{0, 0} {
			t.Fatalf("cancelled warm-up reported resized %d, failed %d", got[0], got[1])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the warm-up didn't stop when cancelled")
	}
	for range cap(c.downloadSlots) {
		<-c.downloadSlots
	}
	time.Sleep(50 * time.Millisecond) // the cancelled job must not go on to make the copy
	if _, err := os.Stat(c.sizedFile(Key(src["big"]), HeroWidth)); err == nil {
		t.Fatal("a cancelled warm-up still made the copy")
	}

	// Already cancelled: nothing starts.
	if r, f := c.Warm(ctx, []string{src["big"]}); r+f != 0 {
		t.Fatalf("resized %d, failed %d after cancellation", r, f)
	}
}

func TestFitWidth(t *testing.T) {
	for _, tc := range []struct{ w, h, srcW, srcH, want int }{
		{400, 0, 1600, 900, 480},   // w alone, as before
		{400, 400, 1600, 900, 780}, // a 16:9 image in a square: 712 wide to be 400 tall
		{1920, 1080, 3000, 2000, 1920},
		{0, 400, 1600, 900, 780},   // h alone
		{0, 5000, 1600, 900, 3840}, // capped at the largest width
		{0, 0, 1600, 900, 0},
	} {
		if got := fitWidth(tc.w, tc.h, tc.srcW, tc.srcH); got != tc.want {
			t.Errorf("fitWidth(w=%d, h=%d, %dx%d) = %d, want %d", tc.w, tc.h, tc.srcW, tc.srcH, got, tc.want)
		}
	}
}
