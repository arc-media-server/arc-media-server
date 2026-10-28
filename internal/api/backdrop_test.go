package api

import (
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/artwork"
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
