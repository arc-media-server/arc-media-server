package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaybackOptionsResolveWithoutStartingPlayback(t *testing.T) {
	s, viewer, f, item := optimizedAudioFixture(t, 120000, 500000, 720)
	for _, tc := range []struct {
		language string
		want     int
	}{{"fra", 2}, {"fr", 2}, {"jpn", 1}, {"", 1}} {
		t.Run("preferred-"+tc.language, func(t *testing.T) {
			prefs := fmt.Sprintf(`{"audio_language":%q,"subtitle_language":"fra"}`, tc.language)
			if err := s.store.SetUserPrefs(t.Context(), viewer.ID, prefs); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(`{"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"]}}`))
			r.SetPathValue("id", item)
			r = r.WithContext(context.WithValue(r.Context(), userKey, viewer))
			w := httptest.NewRecorder()
			// This fixture has no converter. Preview still succeeds for original audio requiring it.
			s.playbackOptions(w, r)
			if w.Code != 200 {
				t.Fatalf("options %d: %s", w.Code, w.Body.String())
			}
			var out playbackPlan
			if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
				t.Fatal(err)
			}
			if out.FileID != f.ID || out.AudioStream != tc.want || len(out.AudioTracks) != 3 || len(out.Versions) != 1 {
				t.Fatalf("choices %+v", out)
			}
			if out.URL != "" || out.SessionID != "" || out.Optimized != nil {
				t.Fatalf("preview started playback: %+v", out)
			}
			if !strings.Contains(w.Body.String(), "Commentary") {
				t.Fatal("source commentary not exposed")
			}
			// The snapshot is read-only with respect to profile choices.
			got, err := s.loadPrefs(r)
			if err != nil || got.AudioLanguage != tc.language || got.SubtitleLanguage != "fra" {
				t.Fatalf("preferences changed: %+v %v", got, err)
			}
		})
	}
	for _, tc := range []struct {
		name, body string
		denied     bool
		want       int
	}{
		{"invalid JSON", "{", false, 400},
		{"unrelated file", `{"file_id":999999}`, false, 404},
		{"restricted library", `{}`, true, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := *viewer
			if tc.denied {
				u.LibraryIDs = []int64{}
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			r.SetPathValue("id", item)
			r = r.WithContext(context.WithValue(r.Context(), userKey, &u))
			w := httptest.NewRecorder()
			s.playbackOptions(w, r)
			if w.Code != tc.want {
				t.Fatalf("status %d want %d: %s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
