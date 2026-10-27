package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/store"
	"ams/internal/subtitles"
)

func TestWebVTTShiftsAndCleansCues(t *testing.T) {
	got := webVTT([]subtitles.Cue{
		{StartMS: 500, EndMS: 1500, Text: "Gone before the start"},
		{StartMS: 1800, EndMS: 2500, Text: "Starts before, ends after"},
		{StartMS: 3_725_004, EndMS: 3_726_000, Text: "Tom & <i>Jerry</i>\n\n--> next"},
	}, -2000)
	want := "WEBVTT\n\n00:00:00.000 --> 00:00:00.500\nStarts before, ends after\n\n01:02:03.004 --> 01:02:04.000\nTom &amp; <i>Jerry</i>\n-> next\n"
	if got != want {
		t.Fatalf("webVTT:\n%q\nwant\n%q", got, want)
	}
}

// A Chromecast fetches a text subtitle track as WebVTT through the cast link, without a cookie,
// shifted to the converted stream's start; another title's track and revoked sessions are refused.
func TestCastSubtitleTrack(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	s, u := securityServer(t)
	s.subtitles = subtitles.NewCache(t.TempDir(), func() string { return ffmpeg })
	ctx := context.Background()
	dir := t.TempDir()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Film"}, "film")
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Other"}, "other")
	if err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(dir, "Film.mp4")
	os.WriteFile(video, []byte("clip"), 0o600)
	os.WriteFile(filepath.Join(dir, "Film.en.srt"), []byte("1\n00:00:10,000 --> 00:00:12,000\nHello <i>there</i>\n"), 0o600)
	fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: video, Size: 4, ModTime: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
		t.Fatal(err)
	}
	tracks := subtitles.Tracks(&store.MediaFile{Path: video})
	if len(tracks) != 1 {
		t.Fatalf("tracks: %+v", tracks)
	}
	_, hash := auth.NewToken()
	if err := s.store.CreateSession(ctx, hash, u.ID, "password", "review"); err != nil {
		t.Fatal(err)
	}
	token := s.cast.add(&castGrant{userID: u.ID, session: hash, itemID: id})
	fetch := func(item string) *httptest.ResponseRecorder {
		rest := "api/items/" + item + "/subtitles/" + tracks[0].ID + "/vtt"
		r := httptest.NewRequest("GET", "http://arc/api/cast/"+token+"/"+rest+"?shift_ms=-9000", nil)
		r.RemoteAddr = "192.168.1.2:45000"
		r.SetPathValue("token", token)
		r.SetPathValue("rest", rest)
		w := httptest.NewRecorder()
		s.castStream(w, r)
		return w
	}
	w := fetch(id)
	if w.Code != 200 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/vtt") || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("cast subtitles: %d %v %s", w.Code, w.Header(), w.Body.String())
	}
	if want := "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello <i>there</i>\n"; w.Body.String() != want {
		t.Fatalf("body %q, want %q", w.Body.String(), want)
	}
	if w := fetch(other); w.Code != http.StatusNotFound {
		t.Fatalf("another title's track through this link: %d", w.Code)
	}
	if err := s.store.DeleteSession(ctx, hash); err != nil {
		t.Fatal(err)
	}
	if w := fetch(id); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session still served subtitles: %d", w.Code)
	}
}
