package api

import (
	"ams/internal/auth"
	"ams/internal/events"
	"ams/internal/watchtogether"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDeviceSessionKeepsUTF8AtByteLimit(t *testing.T) {
	s, u := securityServer(t)
	request := httptest.NewRequest("POST", "http://arc/api/auth/login", nil)
	for _, name := range []string{strings.Repeat("x", 79) + "é", strings.Repeat("x", 78) + "😀", strings.Repeat("x", 80), "短い名前"} {
		token, err := s.deviceSession(request, u.ID, "password", name)
		if err != nil {
			t.Fatal(err)
		}
		session, err := s.store.GetSession(t.Context(), auth.HashToken(token))
		if err != nil || session == nil {
			t.Fatal(err)
		}
		if !utf8.ValidString(session.Device) || len(session.Device) > 80 || !strings.HasPrefix(name, session.Device) {
			t.Fatalf("invalid bounded device name %q", session.Device)
		}
	}
}

func TestStopPlaybackKeepsUTF8AtByteLimit(t *testing.T) {
	s, u := securityServer(t)
	s.playing = newNowPlaying()
	request := httptest.NewRequest("GET", "http://arc/api/items/item/file", nil)
	request = request.WithContext(context.WithValue(request.Context(), userKey, u))
	s.playing.mu.Lock()
	playing := s.playing.touch(request, "item", "")
	s.playing.mu.Unlock()
	message := strings.Repeat("x", 299) + "é"
	body, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		t.Fatal(err)
	}
	stop := httptest.NewRequest("POST", "http://arc/api/playing/id/stop", strings.NewReader(string(body)))
	stop.SetPathValue("id", playing.id)
	stop = stop.WithContext(context.WithValue(stop.Context(), userKey, u))
	response := httptest.NewRecorder()
	s.stopPlaying(response, stop)
	if response.Code != 204 {
		t.Fatal(response.Code, response.Body.String())
	}
	s.playing.mu.Lock()
	reason := s.playing.refusal(playing.key)
	s.playing.mu.Unlock()
	if !utf8.ValidString(reason) || len(reason) > 300 || !strings.HasPrefix(message, reason) {
		t.Fatalf("invalid bounded stop message %q", reason)
	}
}

func TestDefaultEventHubWiresWatchTogetherNotifications(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, supplied := range []*events.Hub{nil, events.New()} {
		manager := watchtogether.New(log)
		New(Deps{Log: log, WatchTogether: manager, Events: supplied})
		if manager.Notify == nil {
			t.Fatal("Watch Together has no notification callback with default event hub")
		}
		manager.Notify([]string{"profile"})
		if supplied != nil {
			_, cursor, _ := supplied.Read("")
			manager.Notify([]string{"other"})
			hints, _, _ := supplied.Read(cursor)
			if len(hints) != 1 || hints[0].UserID != "other" || hints[0].Topic != "watch_together" {
				t.Fatal("wrong personal hint", hints)
			}
		}
	}
}
