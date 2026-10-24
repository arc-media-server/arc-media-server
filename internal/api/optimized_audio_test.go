package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/store"
)

func optimizedAudioFixture(t *testing.T, durationMS, bitrate int64, quality int) (*Server, *store.User, *store.MediaFile, string) {
	t.Helper()
	s, viewer := securityServer(t)
	ctx := t.Context()
	root := t.TempDir()
	lib, err := s.store.SyncLibrary(ctx, "Audio", "movies", "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Audio fixture"}, "audio-fixture")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "source.mkv")
	if err := os.WriteFile(path, []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	f := &store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix(), Container: "matroska", Bitrate: 10000000, Streams: []store.Stream{
		{Index: 0, Kind: "video", Codec: "h264", Height: 90},
		{Index: 1, Kind: "audio", Codec: "ac3", Channels: 6, Language: "eng", IsDefault: true},
		{Index: 2, Kind: "audio", Codec: "aac", Channels: 2, Language: "fra"},
		{Index: 3, Kind: "audio", Codec: "aac", Channels: 2, Language: "eng", Title: "Commentary"},
	}}
	f.ID, err = s.store.SaveFile(ctx, f, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetFileItems(ctx, f.ID, []string{item}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err = s.store.QueueOptimize(ctx, []int64{f.ID}, quality); err != nil {
		t.Fatal(err)
	}
	o, err := s.store.NextOptimize(ctx)
	if err != nil || o == nil {
		t.Fatalf("next: %+v %v", o, err)
	}
	if ok, err := s.store.ClaimOptimize(ctx, o, info.ModTime().UnixNano()); err != nil || !ok {
		t.Fatalf("claim %v %v", ok, err)
	}
	copyPath := filepath.Join(root, "copy.mp4")
	if err = os.WriteFile(copyPath, []byte("English copy"), 0600); err != nil {
		t.Fatal(err)
	}
	o.AudioStream, o.AudioLanguage = 1, "eng"
	o.DurationMS, o.Bitrate = durationMS, bitrate
	if ok, err := s.store.FinishOptimize(ctx, o, copyPath, 12); err != nil || !ok {
		t.Fatalf("finish %v %v", ok, err)
	}

	return s, viewer, f, item
}

func TestOptimizedPlaybackRespectsPreferredAudio(t *testing.T) {
	s, viewer, f, item := optimizedAudioFixture(t, 0, 0, 720)
	ctx := t.Context()
	var err error

	for _, tc := range []struct {
		name, language, extra string
		want, selected        int
	}{
		{"profile French", "fra", "", 503, 2},
		{"two-letter French", "fr", "", 503, 2},
		{"default English", "", "", 200, 1},
		{"profile English", "en", "", 200, 1},
		{"unavailable preferred language uses default", "jpn", "", 200, 1},
		{"explicit default overrides French profile", "fra", `,"audio_stream":1`, 200, 1},
		{"explicit alternate overrides English profile", "eng", `,"audio_stream":2`, 503, 2},
		{"same-language commentary is a different track", "eng", `,"audio_stream":3`, 503, 3},
		{"explicit no audio must not serve audible copy", "eng", `,"audio_stream":-1`, 503, -1},
		{"limited quality rejects unknown copy bitrate", "eng", `,"max_kbps":2000`, 503, 1},
		{"invalid selected track", "", `,"audio_stream":99`, 400, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err = s.store.SetUserPrefs(ctx, viewer.ID, fmt.Sprintf(`{"audio_language":%q}`, tc.language)); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(fmt.Sprintf(`{"file_id":%d,"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"]}%s}`, f.ID, tc.extra)))
			r.SetPathValue("id", item)
			r = r.WithContext(context.WithValue(r.Context(), userKey, viewer))
			w := httptest.NewRecorder()
			s.startPlayback(w, r)
			// No transcoder here: safe fallback refuses instead of silently changing the track.
			if w.Code != tc.want {
				t.Fatalf("status %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
			if w.Code != 200 {
				return
			}
			var plan playbackPlan
			if err := json.Unmarshal(w.Body.Bytes(), &plan); err != nil {
				t.Fatal(err)
			}
			if plan.AudioStream != tc.selected || plan.Optimized == nil || plan.Optimized.SourceAudioStream != tc.selected || !strings.Contains(plan.URL, "optimized=") {
				t.Fatalf("wrong copy/selection: %+v", plan)
			}
			a := plan.Optimized.Audio
			if a == nil || a.Index != 1 || a.Codec != "aac" || a.Channels != 2 || a.Language != "eng" || !a.Default {
				t.Fatalf("incorrect served audio: %+v", a)
			}
			if plan.AudioTracks[0].Codec != "ac3" || plan.AudioTracks[0].Channels != 6 {
				t.Fatal("output details replaced source track choices")
			}
		})
	}
}
