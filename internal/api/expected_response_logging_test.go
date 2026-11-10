package api

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/auth"
	"ams/internal/opensubtitles"
	"ams/internal/store"
	"path/filepath"
)

func TestMissingSubtitleProviderIsNotAServerFailureLog(t *testing.T) {
	s, _ := securityServer(t)
	u, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	if err := s.store.CreateSession(t.Context(), auth.HashToken("fixture"), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	handler := New(Deps{Store: s.store, Settings: s.settings, Log: s.log})
	request := httptest.NewRequest("GET", "http://arc/api/v1/items/fixture/subtitles/search", nil)
	request.Header.Set("Authorization", "Bearer fixture")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 503 {
		t.Fatalf("want configuration-unavailable: %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(logs.String(), "level=WARN") || strings.Contains(logs.String(), "level=ERROR") {
		t.Fatalf("ordinary missing configuration logged as server failure: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "status=503") {
		t.Fatal("detailed request status was lost")
	}
}

func TestCanceledRequestFailureIsQuietButUnexpectedFailuresRemainVisible(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		cancel      bool
		fail        bool
		warn, error int
	}{
		{"canceled", 500, true, true, 0, 0},
		{"internal", 500, false, true, 0, 1},
		{"unclassified unavailable", 503, false, false, 1, 0},
		{"unclassified upstream", 502, false, false, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _ := securityServer(t)
			var logs bytes.Buffer
			s.log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
			handler := s.logRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.fail {
					err := errors.New("fixture internal failure")
					if tc.cancel {
						err = context.Canceled
					}
					s.fail(w, err)
				} else {
					writeError(w, tc.status, "fixture unavailable")
				}
			}))
			request := httptest.NewRequest("GET", "/api/fixture", nil)
			if tc.cancel {
				ctx, cancel := context.WithCancel(request.Context())
				cancel()
				request = request.WithContext(ctx)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatal(response.Code)
			}
			if got := strings.Count(logs.String(), "level=WARN"); got != tc.warn {
				t.Errorf("warnings=%d want=%d: %s", got, tc.warn, logs.String())
			}
			if got := strings.Count(logs.String(), "level=ERROR"); got != tc.error {
				t.Errorf("errors=%d want=%d: %s", got, tc.error, logs.String())
			}
		})
	}
}

func TestSubtitleProviderFailureHasOneSafeProviderDiagnostic(t *testing.T) {
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
		_, _ = w.Write([]byte("fixture provider unavailable"))
	}))
	defer fixture.Close()
	t.Setenv("AMS_OPENSUBTITLES_URL", fixture.URL)
	s, u := securityServer(t)
	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s.subsOnline = opensubtitles.New("fixture")
	s.subsOnline.SetCredentials("fake-key", "", "")
	lib, err := s.store.SyncLibrary(t.Context(), "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	fid, err := s.store.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: filepath.Join(t.TempDir(), "fixture.mkv")}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.store.SetFileItems(t.Context(), fid, []string{id}, 0); err != nil {
		t.Fatal(err)
	}
	handler := s.logRequests(http.HandlerFunc(s.searchSubtitles))
	request := httptest.NewRequest("GET", "/api/items/"+id+"/subtitles/search?languages=en", nil)
	request.SetPathValue("id", id)
	request = request.WithContext(context.WithValue(request.Context(), userKey, u))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 502 {
		t.Fatalf("provider failure: %d %s", response.Code, response.Body.String())
	}
	if strings.Count(logs.String(), "level=WARN") != 1 || !strings.Contains(logs.String(), "provider=opensubtitles") || !strings.Contains(logs.String(), "operation=search") || strings.Contains(logs.String(), "msg=\"request failed\"") {
		t.Fatalf("missing/duplicate provider classification: %s", logs.String())
	}
	if strings.Contains(logs.String(), fixture.URL) || strings.Contains(logs.String(), "fake-key") {
		t.Fatalf("private provider context logged: %s", logs.String())
	}
}
