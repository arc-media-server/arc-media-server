package artwork

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
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
