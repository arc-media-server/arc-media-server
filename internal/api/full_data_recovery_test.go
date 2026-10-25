package api

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/backup"
	"ams/internal/settings"
	"ams/internal/store"
)

// Rehearse the documented stopped-folder copy in an independent location, including
// an actual API artwork upload and authenticated serving. No owner paths are accepted.
func TestFullDataCopyRestoresUploadedArtworkAndDatabase(t *testing.T) {
	ctx := context.Background()
	data, snapshot, recovered := t.TempDir(), filepath.Join(t.TempDir(), "snapshot"), filepath.Join(t.TempDir(), "recovered")
	opened := map[*store.Store]bool{}
	open := func(dir string) *Server {
		t.Helper()
		st, err := store.Open(ctx, filepath.Join(dir, "ams.db"), store.Options{})
		if err != nil {
			t.Fatal(err)
		}
		opened[st] = true
		return &Server{store: st, uploadsDir: filepath.Join(dir, "uploads"), log: slog.New(slog.DiscardHandler), settings: func() settings.Settings { return settings.Settings{} }}
	}
	closeStore := func(s *Server) {
		t.Helper()
		if err := s.store.Close(); err != nil {
			t.Fatal(err)
		}
		opened[s.store] = false
	}
	t.Cleanup(func() {
		for st, live := range opened {
			if live {
				st.Close()
			}
		}
	})
	s := open(data)
	u, err := s.store.CreateUser(ctx, "Recovery owner", true, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.CreateSession(ctx, auth.HashToken("recovery-fixture"), u.ID, "password", "test"); err != nil {
		t.Fatal(err)
	}
	mediaRoot := t.TempDir()
	media := filepath.Join(mediaRoot, "fixture.mkv")
	if err := os.WriteFile(media, []byte("source remains untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.SyncLibrary(ctx, "Recovery library", "movies", "tmdb_aired", []string{mediaRoot})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Original"}, "recovery")
	if err != nil {
		t.Fatal(err)
	}
	fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: media}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
		t.Fatal(err)
	}
	title := "My locked title"
	if err := s.store.EditItem(ctx, id, store.ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetSetting(ctx, "server_name", "Recovery server"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.ReportProgress(ctx, u.ID, id, 120000, 7200000); err != nil {
		t.Fatal(err)
	}
	play, err := s.store.AddPlay(ctx, store.Play{UserID: u.ID, ItemID: id, Kind: "movie", LibraryID: lib, Title: title, StartedAt: 1, EndedAt: 2, WatchedMS: 120000, PositionMS: 120000, DurationMS: 7200000})
	if err != nil {
		t.Fatal(err)
	}
	request := func(method, path, typ string, body []byte) *http.Request {
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.SetPathValue("id", id)
		r.SetPathValue("type", typ)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "recovery-fixture"})
		return r
	}
	upload := func(c color.RGBA) []byte {
		t.Helper()
		im := image.NewRGBA(image.Rect(0, 0, 2, 2))
		im.Set(0, 0, c)
		var b bytes.Buffer
		if err := png.Encode(&b, im); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		s.signedIn(s.uploadArtwork)(w, request("POST", "/api/items/"+id+"/artwork/poster", "poster", b.Bytes()))
		if w.Code != 200 {
			t.Fatalf("upload: %d %s", w.Code, w.Body.String())
		}
		return b.Bytes()
	}
	originalImage := upload(color.RGBA{R: 255, A: 255})
	originalItem, err := s.store.GetItem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	// Opaque fixture bytes prove file preservation, not live TLS/certificate validity.
	cert := []byte("test-only certificate material, not a real private key")
	if err := os.MkdirAll(filepath.Join(data, "certs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "certs", "fixture.pem"), cert, 0600); err != nil {
		t.Fatal(err)
	}
	m := backup.New(filepath.Join(data, "backups"), s.store, slog.New(slog.DiscardHandler))
	b, err := m.Create(ctx, backup.ReasonManual)
	if err != nil {
		t.Fatal(err)
	}
	closeStore(s)
	if err := os.CopyFS(snapshot, os.DirFS(data)); err != nil {
		t.Fatal(err)
	}
	// Replacement artwork removes the old upload. DB-only restore cannot recreate it.
	s = open(data)
	upload(color.RGBA{B: 255, A: 255})
	m = backup.New(filepath.Join(data, "backups"), s.store, slog.New(slog.DiscardHandler))
	if err := m.Restore(ctx, b.Name); err != nil {
		t.Fatal(err)
	}
	oldName := strings.TrimPrefix(originalItem.PosterPath, "upload:")
	if _, err := os.Stat(filepath.Join(data, "uploads", oldName)); !os.IsNotExist(err) {
		t.Fatalf("DB restore unexpectedly restored upload: %v", err)
	}
	closeStore(s)
	if err := os.CopyFS(recovered, os.DirFS(snapshot)); err != nil {
		t.Fatal(err)
	}
	s = open(recovered)
	it, err := s.store.GetItem(ctx, id)
	if err != nil || it.Title != title || it.PosterPath != originalItem.PosterPath || !slices.Contains(it.LockedFields, "title") || !slices.Contains(it.LockedFields, "poster") {
		t.Fatalf("item/locks lost: %+v %v", it, err)
	}
	name, err := s.store.Setting(ctx, "server_name")
	if err != nil || name != "Recovery server" {
		t.Fatalf("settings lost: %s %v", name, err)
	}
	progress, err := s.store.UserDataFor(ctx, u.ID, []string{id})
	if err != nil || progress[id].PositionMS != 120000 {
		t.Fatalf("resume lost: %+v %v", progress, err)
	}
	history, err := s.store.Plays(ctx, store.PlayFilter{UserID: u.ID})
	if err != nil || len(history) != 1 || history[0].ID != play || history[0].ItemID != id {
		t.Fatalf("history lost: %+v %v", history, err)
	}
	w := httptest.NewRecorder()
	s.signedIn(s.image)(w, request("GET", "/api/items/"+id+"/images/primary", "primary", nil))
	if w.Code != 200 || !bytes.Equal(w.Body.Bytes(), originalImage) {
		t.Fatalf("restored upload not served: %d", w.Code)
	}
	gotCert, err := os.ReadFile(filepath.Join(recovered, "certs", "fixture.pem"))
	if err != nil || !bytes.Equal(gotCert, cert) {
		t.Fatalf("certificate fixture lost: %v", err)
	}
	gotMedia, err := os.ReadFile(media)
	if err != nil || string(gotMedia) != "source remains untouched" {
		t.Fatalf("media changed: %v", err)
	}
}
