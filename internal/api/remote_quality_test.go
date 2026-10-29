package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/settings"
	"ams/internal/store"
)

// setRemoteQuality saves a profile's internet streaming quality through PATCH /api/me/prefs.
func setRemoteQuality(t *testing.T, s *Server, u *store.User, body string) int {
	t.Helper()
	r := httptest.NewRequest("PATCH", "/api/me/prefs", strings.NewReader(body))
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	w := httptest.NewRecorder()
	s.updatePrefs(w, r)
	return w.Code
}

func TestRemoteQualityPrefValidation(t *testing.T) {
	s, u := securityServer(t)
	for _, bad := range []string{`{"remote_quality_kbps":5000}`, `{"remote_quality_kbps":-1}`, `{"remote_quality_kbps":100000}`} {
		if code := setRemoteQuality(t, s, u, bad); code != 400 {
			t.Errorf("%s: got %d, want 400", bad, code)
		}
	}
	if code := setRemoteQuality(t, s, u, `{"remote_quality_kbps":4000}`); code != 200 {
		t.Fatalf("4000: got %d", code)
	}
	p, err := s.loadPrefsFor(t.Context(), u.ID)
	if err != nil || p.RemoteQualityKbps != 4000 {
		t.Fatalf("saved %d %v, want 4000", p.RemoteQualityKbps, err)
	}
	// Other preferences leave it alone; 0 goes back to Maximum.
	if code := setRemoteQuality(t, s, u, `{"hide_spoilers":true}`); code != 200 {
		t.Fatal(code)
	}
	if p, _ = s.loadPrefsFor(t.Context(), u.ID); p.RemoteQualityKbps != 4000 {
		t.Fatalf("lost by another change: %d", p.RemoteQualityKbps)
	}
	if code := setRemoteQuality(t, s, u, `{"remote_quality_kbps":0}`); code != 200 {
		t.Fatal(code)
	}
	if p, _ = s.loadPrefsFor(t.Context(), u.ID); p.RemoteQualityKbps != 0 {
		t.Fatalf("not reset: %d", p.RemoteQualityKbps)
	}
}

func TestRemoteLimitUsesProfileQuality(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		server, profile, want int
		home                  bool
	}{
		{name: "no limits", want: 0},
		{name: "profile only", profile: 3000, want: 3000},
		{name: "server only", server: 8000, want: 8000},
		{name: "profile lower than server", server: 8000, profile: 4000, want: 4000},
		{name: "server lower than profile", server: 8000, profile: 12000, want: 8000},
		{name: "home ignores both", server: 8000, profile: 2000, home: true, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, u := securityServer(t)
			s.settings = func() settings.Settings { return settings.Settings{RemoteKbps: tc.server} }
			if code := setRemoteQuality(t, s, u, fmt.Sprintf(`{"remote_quality_kbps":%d}`, tc.profile)); code != 200 {
				t.Fatal(code)
			}
			r := httptest.NewRequest("POST", "/", nil)
			r.RemoteAddr = "203.0.113.5:5000"
			if tc.home {
				r.RemoteAddr = "192.168.1.20:5000"
			}
			r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			if got := s.remoteLimit(r, "x"); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

// The playback decision honours the profile's internet quality away from home, plays a file that's
// already within it untouched (Plex's "Play smaller videos at original quality", which Arc does by
// itself), and ignores it at home.
func TestPlaybackHonoursRemoteQuality(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	root := t.TempDir()
	lib, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	add := func(name string, bitrate int64, height int) string {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: name}, name)
		if err != nil {
			t.Fatal(err)
		}
		f := &store.MediaFile{LibraryID: lib, Path: filepath.Join(root, name+".mp4"), Size: 1, ModTime: 1, Container: "mp4",
			Bitrate: bitrate, DurationMS: 60000, Streams: []store.Stream{
				{Index: 0, Kind: "video", Codec: "h264", Height: height, Width: height * 16 / 9},
				{Index: 1, Kind: "audio", Codec: "aac", Channels: 2, IsDefault: true},
			}}
		if f.ID, err = s.store.SaveFile(ctx, f, 1, 1); err != nil {
			t.Fatal(err)
		}
		if err = s.store.SetFileItems(ctx, f.ID, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		return id
	}
	small := add("small", 2500000, 720) // 2.5 Mbps 720p
	big := add("big", 15000000, 1080)   // 15 Mbps 1080p
	if code := setRemoteQuality(t, s, u, `{"remote_quality_kbps":4000}`); code != 200 {
		t.Fatal(code)
	}
	play := func(item, address string) (int, playbackPlan, string) {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"]}}`))
		r.RemoteAddr = address
		r.SetPathValue("id", item)
		r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		w := httptest.NewRecorder()
		s.startPlayback(w, r)
		var plan playbackPlan
		_ = json.Unmarshal(w.Body.Bytes(), &plan)
		return w.Code, plan, w.Body.String()
	}
	const away, home = "203.0.113.5:5000", "192.168.1.20:5000"

	// Under the limit: played as it is, with the limit reported to the player.
	code, plan, body := play(small, away)
	if code != 200 || plan.Method != "direct" || plan.ServerLimitKbps != 4000 {
		t.Fatalf("small file away: %d %s", code, body)
	}
	// Over it: converted down (this test server has no converter, so it says so).
	if code, _, body = play(big, away); code != 503 || !strings.Contains(body, "convert") {
		t.Fatalf("big file away: %d %s", code, body)
	}
	// At home the profile's internet quality doesn't apply.
	code, plan, body = play(big, home)
	if code != 200 || plan.Method != "direct" || plan.ServerLimitKbps != 0 {
		t.Fatalf("big file at home: %d %s", code, body)
	}
}
