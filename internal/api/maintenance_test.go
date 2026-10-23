package api

import (
	"ams/internal/store"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ams/internal/activity"
	"ams/internal/auth"
	"ams/internal/background"
	"ams/internal/schedule"
	"ams/internal/settings"
)

func TestMaintenanceAccessModesAndCoalescedRunNow(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := context.Background()
	admin, err := s.store.CreateUser(ctx, "Owner", true, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	s.background, err = background.New(ctx, s.store, schedule.New(0, 0), activity.New())
	if err != nil {
		t.Fatal(err)
	}
	s.background.Configure(settings.Settings{AnalysisEnabled: true, PreviewMode: "off", DetectionMode: "off"})
	calls := 0
	s.background.Register("analysis", func() { calls++ })
	for token, id := range map[string]string{"viewer": viewer.ID, "owner": admin.ID} {
		if err := s.store.CreateSession(ctx, auth.HashToken(token), id, "password", "fixture"); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		token, kind string
		want        int
	}{{"viewer", "analysis", 403}, {"owner", "unknown", 400}, {"owner", "previews", 400}, {"owner", "analysis", 202}, {"owner", "analysis", 202}} {
		r := httptest.NewRequest("POST", "/api/maintenance/"+tt.kind+"/run", nil)
		r.SetPathValue("kind", tt.kind)
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: tt.token})
		w := httptest.NewRecorder()
		s.adminOnly(s.runMaintenance)(w, r)
		if w.Code != tt.want {
			t.Fatalf("%s/%s: %d %s", tt.token, tt.kind, w.Code, w.Body.String())
		}
	}
	if calls != 1 {
		t.Fatalf("repeated Run now dispatched %d callbacks", calls)
	}
}

// Exercise the real API wiring, not an injected foreground boolean: playback progress
// must cancel an optional lease, while paused playback permits work again.
func TestPlaybackProgressPreemptsBackgroundThroughRealRouter(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := context.Background()
	lib, err := s.store.SyncLibrary(ctx, "Playback", "movies", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	item, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture", MetadataState: "matched"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	f := store.MediaFile{LibraryID: lib, Path: "fixture.mkv", Size: 100, ModTime: 1}
	f.ID, err = s.store.SaveFile(ctx, &f, 99, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.CreateSession(ctx, auth.HashToken("playback"), viewer.ID, "password", "fixture"); err != nil {
		t.Fatal(err)
	}
	tracker := activity.New()
	bg, err := background.New(ctx, s.store, schedule.New(0, 0), tracker)
	if err != nil {
		t.Fatal(err)
	}
	cfg := settings.Settings{AnalysisEnabled: true, PreviewMode: "off", DetectionMode: "off"}
	bg.Configure(cfg)
	handler := New(Deps{Store: s.store, Activity: tracker, Background: bg, Settings: func() settings.Settings { return cfg }, Log: s.log})
	work, finish, ok := bg.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("background lease denied")
	}
	report := func(state string) {
		t.Helper()
		r := httptest.NewRequest("PUT", "/api/items/"+item+"/progress", strings.NewReader(`{"position_ms":1000,"duration_ms":120000,"state":"`+state+`"}`))
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "playback"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code >= 300 {
			t.Fatalf("progress: %d %s", w.Code, w.Body.String())
		}
	}
	report("playing")
	select {
	case <-work.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("real playback did not preempt background work")
	}
	finish(work.Err())
	report("paused")
	_, finish, ok = bg.Begin(ctx, "analysis", "file", f.ID)
	if !ok {
		t.Fatal("paused playback blocked all background work")
	}
	finish(nil)
}
