package artwork

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCacheDownloadsOnceAndPrunes(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/missing.jpg" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		io.WriteString(w, "jpeg bytes for "+r.URL.Path)
	}))
	defer srv.Close()

	c := New(t.TempDir(), slog.New(slog.DiscardHandler))
	ctx := context.Background()
	a, b := srv.URL+"/a.jpg", srv.URL+"/b.jpg"

	for range 3 {
		f, err := c.Open(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(f)
		f.Close()
		if string(body) != "jpeg bytes for /a.jpg" {
			t.Fatalf("got %q", body)
		}
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("downloaded %d times, want 1", n)
	}

	if _, err := c.Open(ctx, srv.URL+"/missing.jpg"); err == nil {
		t.Fatal("expected an error for a missing image")
	}

	downloaded, failed := c.Prefetch(ctx, []string{a, b, ""})
	if downloaded != 1 || failed != 0 {
		t.Fatalf("prefetch downloaded %d, failed %d; want 1, 0", downloaded, failed)
	}

	removed, err := c.Prune([]string{b})
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 || c.Has(a) || !c.Has(b) {
		t.Fatalf("prune removed %d (a cached: %v, b cached: %v); want a removed and b kept", removed, c.Has(a), c.Has(b))
	}
}

func TestSourceSizes(t *testing.T) {
	tests := []struct{ kind, typ, path, want string }{
		{"movie", "primary", "/p.jpg", "https://image.tmdb.org/t/p/w500/p.jpg"},
		{"episode", "primary", "/s.jpg", "https://image.tmdb.org/t/p/w780/s.jpg"},
		{"series", "backdrop", "/b.jpg", "https://image.tmdb.org/t/p/w1280/b.jpg"},
		{"series", "primary", "https://artworks.thetvdb.com/banners/x.jpg", "https://artworks.thetvdb.com/banners/x.jpg"},
		{"movie", "primary", "", ""},
	}
	for _, tt := range tests {
		if got := Source(tt.kind, tt.typ, tt.path); got != tt.want {
			t.Errorf("Source(%q, %q, %q) = %q, want %q", tt.kind, tt.typ, tt.path, got, tt.want)
		}
	}
}

func TestFullSourceForBanners(t *testing.T) {
	tests := []struct{ path, want string }{
		{"/b.jpg", "https://image.tmdb.org/t/p/original/b.jpg"},
		{"https://artworks.thetvdb.com/banners/fanart/x.jpg", ""}, // TheTVDB has one size only
		{"upload:abc.jpg", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := FullSource("movie", tt.path); got != tt.want {
			t.Errorf("FullSource(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
	got := Sources("movie", "backdrop", "/b.jpg")
	if len(got) != 2 || got[0] != "https://image.tmdb.org/t/p/w1280/b.jpg" || got[1] != "https://image.tmdb.org/t/p/original/b.jpg" {
		t.Errorf("backdrop sources = %v, want the card size then the full size", got)
	}
	if got := Sources("movie", "primary", "/p.jpg"); len(got) != 1 {
		t.Errorf("poster sources = %v, want one", got)
	}
}

func TestCancelledDownloadWaiterDoesNotCancelSharedFetch(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("fixture"))
	}))
	defer server.Close()
	cache := New(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		f, err := cache.Open(ctx, server.URL+"/art.png")
		if f != nil {
			f.Close()
		}
		done <- err
	}()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("waiter: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled request kept waiting")
	}
	close(release)
	f, err := cache.Open(t.Context(), server.URL+"/art.png")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if !cache.Has(server.URL + "/art.png") {
		t.Fatal("shared cache was not filled")
	}
}
