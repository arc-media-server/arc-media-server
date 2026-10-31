package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// A track that takes a while to read (the first time, out of a large video) doesn't hold the
// player up: the WebVTT header comes at once, blank lines keep the connection alive, and the cues
// follow. A player that gives up doesn't cancel the reading, so the next request is instant.
func TestSlowSubtitleTrackAnswersAtOnce(t *testing.T) {
	dir := t.TempDir()
	runs := filepath.Join(dir, "runs")
	fake := filepath.Join(dir, "ffmpeg")
	// Stands in for FFmpeg reading a whole large file: slow, then the track as WebVTT.
	script := "#!/bin/sh\ncat >/dev/null\necho run >> " + runs + "\nsleep 2\nprintf 'WEBVTT\\n\\n00:00:10.000 --> 00:00:12.000\\nHello\\n'\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	oldWait, oldAlive := vttWaitBeforeHeader, vttKeepAlive
	vttWaitBeforeHeader, vttKeepAlive = 50*time.Millisecond, 300*time.Millisecond
	defer func() { vttWaitBeforeHeader, vttKeepAlive = oldWait, oldAlive }()

	s, u := securityServer(t)
	s.subtitles = subtitles.NewCache(t.TempDir(), func() string { return fake })
	ctx := context.Background()
	media := t.TempDir()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{media})
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Film"}, "film")
	if err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(media, "Film.mp4")
	os.WriteFile(video, []byte("clip"), 0o600)
	os.WriteFile(filepath.Join(media, "Film.en.srt"), []byte("1\n00:00:10,000 --> 00:00:12,000\nHello\n"), 0o600)
	fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: video, Size: 4, ModTime: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
		t.Fatal(err)
	}
	track := subtitles.Tracks(&store.MediaFile{Path: video})[0]
	_, hash := auth.NewToken()
	if err := s.store.CreateSession(ctx, hash, u.ID, "password", "review"); err != nil {
		t.Fatal(err)
	}
	token := s.cast.add(&castGrant{userID: u.ID, session: hash, itemID: id})
	rest := "api/items/" + id + "/subtitles/" + track.ID + "/vtt"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("token", token)
		r.SetPathValue("rest", rest)
		s.castStream(w, r)
	}))
	defer srv.Close()
	url := srv.URL + "/api/cast/" + token + "/" + rest

	// A player that gives up after a moment still gets the header, well before the track is read.
	quick, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	req, _ := http.NewRequestWithContext(quick, "GET", url, nil)
	started := time.Now()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("no answer while the track is read: %v", err)
	}
	head := make([]byte, 7)
	if _, err := io.ReadFull(res.Body, head); err != nil || string(head) != "WEBVTT\n" || time.Since(started) > time.Second {
		t.Fatalf("header %q after %v: %v", head, time.Since(started), err)
	}
	res.Body.Close()
	cancel()

	// The reading carried on: once it's done, the track is served whole and FFmpeg isn't run again.
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if strings.Contains(string(body), "00:00:10.000 --> 00:00:12.000\nHello") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("cues never arrived: %q", body)
		}
	}
	if b, _ := os.ReadFile(runs); strings.Count(string(b), "run") != 1 {
		t.Fatalf("FFmpeg ran %d times, want once (the abandoned request shouldn't cancel it)", strings.Count(string(b), "run"))
	}
}
