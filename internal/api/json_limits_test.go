package api

import (
	"ams/internal/auth"
	"ams/internal/store"
	"bytes"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type countedRequestBody struct {
	io.Reader
	read int64
}

func (b *countedRequestBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += int64(n)
	return n, err
}
func (*countedRequestBody) Close() error { return nil }

func TestAuthenticatedJSONBodyHasBoundedReadBudget(t *testing.T) {
	s, u := securityServer(t)
	if err := s.store.CreateSession(t.Context(), auth.HashToken("fixture"), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	handler := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	payload := `{"title":"` + strings.Repeat("x", 3<<20) + `"}`
	for _, prefix := range []string{"/api", "/api/v1"} {
		for _, chunked := range []bool{false, true} {
			body := &countedRequestBody{Reader: strings.NewReader(payload)}
			r := httptest.NewRequest("POST", "http://arc"+prefix+"/playlists", body)
			r.Header.Set("Authorization", "Bearer fixture")
			// A spoofed content type must not exempt a JSON endpoint.
			r.Header.Set("Content-Type", "image/png")
			r.ContentLength = int64(len(payload))
			if chunked {
				r.ContentLength = -1
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if body.read > (2<<20)+1 {
				t.Errorf("unbounded JSON body read %d bytes (chunked=%v)", body.read, chunked)
			}
			if !chunked && w.Code != http.StatusRequestEntityTooLarge {
				t.Errorf("known large body status %d want 413", w.Code)
			}
			if chunked && w.Code != 400 && w.Code != 413 {
				t.Errorf("chunked large body status %d", w.Code)
			}
		}
	}
}

func TestLargeBodyStillRequiresAuthenticationFirst(t *testing.T) {
	s, _ := securityServer(t)
	handler := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	body := &countedRequestBody{Reader: strings.NewReader(`{"title":"` + strings.Repeat("x", 3<<20) + `"}`)}
	r := httptest.NewRequest("POST", "http://arc/api/v1/playlists", body)
	r.ContentLength = 3 << 20
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 || body.read != 0 {
		t.Fatal("body processed before authentication", w.Code, body.read)
	}
}

func TestArtworkUploadKeepsSeparateTwentyMiBBudget(t *testing.T) {
	s, _ := securityServer(t)
	owner, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(t.Context(), auth.HashToken("owner"), owner.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	handler := New(Deps{Store: s.store, Settings: s.settings, Log: s.log, UploadsDir: t.TempDir()})
	var imageBytes bytes.Buffer
	if err = png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	data := append(imageBytes.Bytes(), make([]byte, 3<<20)...)
	for _, prefix := range []string{"/api", "/api/v1"} {
		r := httptest.NewRequest("POST", "http://arc"+prefix+"/items/"+item+"/artwork/poster", bytes.NewReader(data))
		r.Header.Set("Authorization", "Bearer owner")
		r.Header.Set("Content-Type", "image/png")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatal("image upload inherited JSON budget", w.Code, w.Body.String())
		}
	}
}
