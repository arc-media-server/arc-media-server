package api

import "testing"

// Show Missing Episodes is a profile setting: off by default, saved on its own, and left alone by
// other changes.
func TestShowMissingEpisodesPref(t *testing.T) {
	s, u := securityServer(t)
	p, err := s.loadPrefsFor(t.Context(), u.ID)
	if err != nil || p.ShowMissingEpisodes {
		t.Fatalf("default %v %v, want off", p.ShowMissingEpisodes, err)
	}
	if code := setRemoteQuality(t, s, u, `{"show_missing_episodes":true}`); code != 200 {
		t.Fatalf("on: got %d", code)
	}
	if p, _ = s.loadPrefsFor(t.Context(), u.ID); !p.ShowMissingEpisodes {
		t.Fatal("not saved")
	}
	if code := setRemoteQuality(t, s, u, `{"auto_skip":true}`); code != 200 {
		t.Fatal(code)
	}
	if p, _ = s.loadPrefsFor(t.Context(), u.ID); !p.ShowMissingEpisodes {
		t.Fatal("lost by another change")
	}
	if code := setRemoteQuality(t, s, u, `{"show_missing_episodes":false}`); code != 200 {
		t.Fatal(code)
	}
	if p, _ = s.loadPrefsFor(t.Context(), u.ID); p.ShowMissingEpisodes {
		t.Fatal("not turned off")
	}
}
