package api

import (
	"bytes"
	"image"
	"image/jpeg"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/artwork"
	"ams/internal/events"
	"ams/internal/store"
)

func TestBannerBackdropIsFullSize(t *testing.T) {
	d := toDTO(store.Item{ID: "m1", Kind: "movie", BackdropPath: "/b.jpg"})
	card, full := d.Images["backdrop"], d.Images["backdrop_full"]
	if !strings.Contains(card, "v="+artwork.Key("https://image.tmdb.org/t/p/w1280/b.jpg")) {
		t.Errorf("card backdrop = %q, want the w1280 version", card)
	}
	want := "/api/items/m1/images/backdrop?size=full&v=" + artwork.Key("https://image.tmdb.org/t/p/original/b.jpg")
	if full != want {
		t.Errorf("banner backdrop = %q, want %q", full, want)
	}

	r := httptest.NewRequest("GET", full, nil)
	if got := imageSource(r, "movie", "backdrop", "/b.jpg"); got != "https://image.tmdb.org/t/p/original/b.jpg" {
		t.Errorf("full-size request serves %q", got)
	}
	r = httptest.NewRequest("GET", card, nil)
	if got := imageSource(r, "movie", "backdrop", "/b.jpg"); got != "https://image.tmdb.org/t/p/w1280/b.jpg" {
		t.Errorf("card request serves %q", got)
	}

	// Local fanart and TheTVDB artwork have one size: no separate banner link.
	for _, it := range []store.Item{
		{ID: "m2", Kind: "movie", BackdropPath: "/b.jpg", LocalBackdrop: "/media/Movie/fanart.jpg"},
		{ID: "s1", Kind: "series", BackdropPath: "https://artworks.thetvdb.com/banners/fanart/x.jpg"},
	} {
		if d := toDTO(it); d.Images["backdrop_full"] != "" || d.Images["backdrop"] == "" {
			t.Errorf("%s: images = %v, want only a backdrop link", it.ID, d.Images)
		}
	}
}

func TestWideBackdropRequestUsesTheOriginal(t *testing.T) {
	for w, want := range map[string]string{
		"1920": "https://image.tmdb.org/t/p/original/b.jpg",
		"3840": "https://image.tmdb.org/t/p/original/b.jpg",
		"1280": "https://image.tmdb.org/t/p/w1280/b.jpg",
		"780":  "https://image.tmdb.org/t/p/w1280/b.jpg",
	} {
		r := httptest.NewRequest("GET", "/api/items/m1/images/backdrop?w="+w, nil)
		if got := imageSource(r, "movie", "backdrop", "/b.jpg"); got != want {
			t.Errorf("w=%s serves %q, want %q", w, got, want)
		}
	}
}

func TestServeArtworkResizesAndKeepsItsOwnVersion(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2000, 1000))
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(buf.Bytes())
	}))
	defer provider.Close()
	log := slog.New(slog.DiscardHandler)
	cache := artwork.New(t.TempDir(), log)
	cache.Events = events.New()
	s := &Server{artwork: cache, log: log}
	src := provider.URL + "/b.jpg"
	key := artwork.Key(src)

	rec := httptest.NewRecorder()
	s.serveArtwork(rec, httptest.NewRequest("GET", "/x?w=1000&v="+key, nil), src)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("status %d, type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if got := rec.Header().Get("ETag"); got != `"`+key+`-w1280"` {
		t.Errorf("ETag %s: a resized copy needs its own", got)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("Cache-Control %q: a versioned resized link is as permanent as the original", rec.Header().Get("Cache-Control"))
	}
	cfg, _, err := image.DecodeConfig(rec.Body)
	if err != nil || cfg.Width != 1280 {
		t.Fatalf("served %dpx (%v), want 1280", cfg.Width, err)
	}

	rec = httptest.NewRecorder()
	s.serveArtwork(rec, httptest.NewRequest("GET", "/x", nil), src)
	if cfg, _, _ := image.DecodeConfig(rec.Body); cfg.Width != 2000 || rec.Header().Get("ETag") != `"`+key+`"` {
		t.Fatalf("without w: %dpx, ETag %s; want the original", cfg.Width, rec.Header().Get("ETag"))
	}
}
