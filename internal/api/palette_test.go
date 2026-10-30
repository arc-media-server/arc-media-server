package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ams/internal/artwork"
	"ams/internal/auth"
	"ams/internal/metadata"
	"ams/internal/store"
)

func palettePNG(t *testing.T, c color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
func TestArtworkPaletteAPISelectionCacheAndAccess(t *testing.T) {
	s, u := securityServer(t)
	var logged bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logged, nil))
	ctx := t.Context()
	lib, err := s.store.SyncLibrary(ctx, "Fixtures", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "palette-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(ctx, auth.HashToken("palette-fixture"), u.ID, "password", "palette tests"); err != nil {
		t.Fatal(err)
	}
	uploads := t.TempDir()
	art := artwork.New(t.TempDir(), s.log)
	handler := New(Deps{Store: s.store, Artwork: art, UploadsDir: uploads, Settings: s.settings, Log: s.log})
	request := func(query, etag string, authenticated bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1/items/"+id+"/artwork/palette"+query, nil)
		if authenticated {
			r.Header.Set("Authorization", "Bearer palette-fixture")
		}
		if etag != "" {
			r.Header.Set("If-None-Match", etag)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	decode := func(w *httptest.ResponseRecorder) paletteResponse {
		t.Helper()
		if w.Code != 200 {
			t.Fatalf("status: %d %s", w.Code, w.Body.String())
		}
		var out paletteResponse
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := request("", "", false).Code; got != 401 {
		t.Fatalf("anonymous: %d", got)
	}
	if got := request("?type=logo", "", true).Code; got != 400 {
		t.Fatalf("invalid type: %d", got)
	}
	if out := decode(request("", "", true)); out.Available || out.Reason != "no_artwork" {
		t.Fatalf("no art: %+v", out)
	}
	local := filepath.Join(t.TempDir(), "fanart.png")
	if err = os.WriteFile(local, palettePNG(t, color.NRGBA{255, 0, 0, 255}), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetLocalArt(ctx, id, local, "", ""); err != nil {
		t.Fatal(err)
	}
	w := request("", "", true)
	first := decode(w)
	if !first.Available || first.ArtworkType != "primary" || first.Colors.TopLeft != "#ff0000" || !strings.HasPrefix(first.ArtworkURL, "/api/v1/") || !strings.Contains(first.ArtworkURL, "palette_version=") {
		t.Fatalf("poster fallback: %+v", first)
	}
	if strings.Contains(w.Body.String(), filepath.Dir(local)) {
		t.Fatal("local path leaked")
	}
	if w.Header().Get("Cache-Control") != "private, no-cache" {
		t.Fatal("palette is not private")
	}

	imageRequest := httptest.NewRequest("GET", first.ArtworkURL, nil)
	imageRequest.Header.Set("Authorization", "Bearer palette-fixture")
	imageResponse := httptest.NewRecorder()
	handler.ServeHTTP(imageResponse, imageRequest)
	if imageResponse.Code != 200 || imageResponse.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("returned artwork URL: %d %s", imageResponse.Code, imageResponse.Header().Get("Content-Type"))
	}
	etag := w.Header().Get("ETag")
	if got := request("", etag, true).Code; got != 304 {
		t.Fatalf("revalidation: %d", got)
	}
	if got := request("", `W/`+etag, true).Code; got != 304 {
		t.Fatalf("weak revalidation: %d", got)
	}

	// Identical bytes selected from a different artwork source still change the response ETag/URL.
	if err = os.WriteFile(filepath.Join(uploads, "same.png"), palettePNG(t, color.NRGBA{255, 0, 0, 255}), 0600); err != nil {
		t.Fatal(err)
	}
	samePath := metadata.UploadPrefix + "same.png"
	if err = s.store.EditItem(ctx, id, store.ItemEdit{Poster: &samePath}); err != nil {
		t.Fatal(err)
	}
	same := decode(request("", etag, true))
	if same.ArtworkVersion != first.ArtworkVersion || same.ArtworkURL == first.ArtworkURL {
		t.Fatalf("source change was not represented: %+v", same)
	}
	empty := ""
	if err = s.store.EditItem(ctx, id, store.ItemEdit{Poster: &empty}); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(local, palettePNG(t, color.NRGBA{0, 0, 255, 255}), 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Second)
	os.Chtimes(local, now, now)
	next := decode(request("", etag, true))
	if next.ArtworkVersion == first.ArtworkVersion || next.Colors.TopLeft != "#0000ff" {
		t.Fatalf("local replacement: %+v", next)
	}
	// A locked uploaded choice wins over local art, matching the existing image endpoint.
	if err = os.WriteFile(filepath.Join(uploads, "choice.png"), palettePNG(t, color.NRGBA{0, 255, 0, 255}), 0600); err != nil {
		t.Fatal(err)
	}
	path := metadata.UploadPrefix + "choice.png"
	if err = s.store.EditItem(ctx, id, store.ItemEdit{Poster: &path}); err != nil {
		t.Fatal(err)
	}
	chosen := decode(request("?type=primary", "", true))
	if chosen.Colors.TopLeft != "#00ff00" {
		t.Fatalf("upload precedence: %+v", chosen)
	}
	if err = os.WriteFile(filepath.Join(uploads, "choice.png"), []byte("invalid image"), 0600); err != nil {
		t.Fatal(err)
	}
	bad := decode(request("?type=primary", "", true))
	if bad.Available || bad.Reason != "unsupported_image" {
		t.Fatalf("corrupt: %+v", bad)
	}
	rating := "18"
	age := 12
	if err = s.store.EditItem(ctx, id, store.ItemEdit{ContentRating: &rating}); err != nil {
		t.Fatal(err)
	}
	if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{MaxAge: &age}); err != nil {
		t.Fatal(err)
	}
	if got := request("", etag, true).Code; got != 404 {
		t.Fatalf("age-restricted palette: %d", got)
	}
	age = 0
	if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{MaxAge: &age}); err != nil {
		t.Fatal(err)
	}

	_ = decode(request("?type=primary", "", true))
	if count := strings.Count(logged.String(), "artwork palette unavailable"); count != 1 {
		t.Fatalf("unbounded warnings: %d %s", count, logged.String())
	}
	if strings.Contains(logged.String(), filepath.Dir(local)) || strings.Contains(logged.String(), "palette-fixture") {
		t.Fatalf("sensitive details in log: %s", logged.String())
	}
	// Denied users cannot use an ETag to obtain a cached response for hidden artwork.
	none := []int64{}
	if err = s.store.UpdateUser(ctx, u.ID, store.UserUpdate{LibraryIDs: &none}); err != nil {
		t.Fatal(err)
	}
	if got := request("", etag, true).Code; got != 404 {
		t.Fatalf("restricted conditional: %d", got)
	}
}

func TestArtworkPaletteProviderAndRequestURLIsolation(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	hits := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "image/png")
		w.Write(palettePNG(t, color.NRGBA{128, 64, 255, 255}))
	}))
	defer provider.Close()
	lib, err := s.store.SyncLibrary(ctx, "Provider", "shows", "tvdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Provider fixture", BackdropPath: provider.URL + "/image.png"}, "provider-palette")
	if err != nil {
		t.Fatal(err)
	}
	source := provider.URL + "/image.png"
	if err := s.store.EditItem(ctx, id, store.ItemEdit{Backdrop: &source}); err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(ctx, auth.HashToken("provider-fixture"), u.ID, "password", "provider test"); err != nil {
		t.Fatal(err)
	}
	handler := New(Deps{Store: s.store, Artwork: artwork.New(t.TempDir(), s.log), Settings: s.settings, Log: s.log})
	for range 2 {
		r := httptest.NewRequest("GET", "/api/v1/items/"+id+"/artwork/palette?url=http://invalid.local/never-fetch", nil)
		r.Header.Set("Authorization", "Bearer provider-fixture")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("provider: %d %s", w.Code, w.Body.String())
		}
		var out paletteResponse
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if !out.Available || out.ArtworkType != "backdrop" || out.Colors.TopLeft != "#8040ff" {
			t.Fatalf("provider palette: %+v", out)
		}
		if strings.Contains(w.Body.String(), provider.URL) || strings.Contains(w.Body.String(), "invalid.local") {
			t.Fatal("source URL leaked")
		}
	}
	if hits != 1 {
		t.Fatalf("provider fetched %d times", hits)
	}
}
