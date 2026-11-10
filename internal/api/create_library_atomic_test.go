package api

import (
	"ams/internal/auth"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/settings"
	"ams/internal/store"
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestLibraryCreateRejectsInvalidOptionsBeforeSaving(t *testing.T) {
	for _, option := range []map[string]any{{"collection_min": -1}, {"collection_min": 11}, {"metadata_language": "not a language"}, {"collection_grid": "invalid"}} {
		t.Run(strings.TrimSpace(strings.ReplaceAll(strings.TrimSpace(string(mustLibraryJSON(t, option))), "\"", "")), func(t *testing.T) {
			s, _ := securityServer(t)
			ctx, cancel := context.WithCancel(t.Context())
			sc := scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
			defer func() { cancel(); sc.Wait() }()
			owner, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.store.CreateSession(t.Context(), auth.HashToken("owner"), owner.ID, "password", ""); err != nil {
				t.Fatal(err)
			}
			body := map[string]any{"name": "Fixture", "kind": "movies", "paths": []string{t.TempDir()}}
			for key, value := range option {
				body[key] = value
			}
			notifications := 0
			h := New(Deps{Store: s.store, Scanner: sc, Settings: s.settings, Log: s.log, LibrariesChanged: func() { notifications++ }})
			r := httptest.NewRequest("POST", "http://arc/api/v1/libraries", strings.NewReader(string(mustLibraryJSON(t, body))))
			r.Header.Set("Authorization", "Bearer owner")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 400 {
				t.Errorf("invalid option accepted: %d %s", w.Code, w.Body.String())
			}
			libraries, err := s.store.ListLibraries(t.Context())
			if err != nil || len(libraries) != 0 || notifications != 0 {
				t.Errorf("invalid create saved/notified: libraries=%d notifications=%d (%v)", len(libraries), notifications, err)
			}
		})
	}
}

func TestLibraryCreateDefaultsAreCompleteBeforeNotification(t *testing.T) {
	for _, kind := range []string{"movies", "shows", "videos"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := securityServer(t)
			ctx, cancel := context.WithCancel(t.Context())
			sc := scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
			defer func() { cancel(); sc.Wait() }()
			owner, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if err = s.store.CreateSession(t.Context(), auth.HashToken("owner"), owner.ID, "password", ""); err != nil {
				t.Fatal(err)
			}
			notifications := 0
			h := New(Deps{Store: s.store, Scanner: sc, Settings: func() settings.Settings { return settings.Settings{Region: "GB"} }, Log: s.log, LibrariesChanged: func() {
				notifications++
				libraries, err := s.store.ListLibraries(t.Context())
				if err != nil || len(libraries) != 1 {
					t.Error("notification before persistence", libraries, err)
					return
				}
				lib := libraries[0]
				if lib.SkipIntros != (kind == "shows") || lib.SkipCredits != (kind != "videos") || lib.RatingRegion != "GB" || lib.CollectionMin != 2 {
					t.Error("notification saw partial options", lib)
				}
			}})
			payload := mustLibraryJSON(t, map[string]any{"name": "Complete", "kind": kind, "paths": []string{t.TempDir()}})
			r := httptest.NewRequest("POST", "http://arc/api/v1/libraries", strings.NewReader(string(payload)))
			r.Header.Set("Authorization", "Bearer owner")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != 201 {
				t.Fatal(w.Code, w.Body.String())
			}
			var lib store.Library
			if err = json.Unmarshal(w.Body.Bytes(), &lib); err != nil || lib.Kind != kind || lib.SourceOrdering != "tmdb_aired" || notifications != 1 {
				t.Fatal("defaults or response changed", lib, notifications, err)
			}
		})
	}
}

func TestLibraryConfigurationWriteFailureLeavesNoLibraryOrNotification(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, _ := securityServerAt(t, path)
	ctx, cancel := context.WithCancel(t.Context())
	sc := scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
	defer func() { cancel(); sc.Wait() }()
	owner, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(t.Context(), auth.HashToken("owner"), owner.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Reject the same configuration at either write boundary: the old follow-up
	// UPDATE, or a complete atomic INSERT. This is a temporary fixture database.
	for _, statement := range []string{
		`CREATE TRIGGER fail_provider_update BEFORE UPDATE OF details_provider ON libraries WHEN NEW.details_provider='tvdb' BEGIN SELECT RAISE(ABORT,'fixture configuration write failure'); END`,
		`CREATE TRIGGER fail_provider_insert BEFORE INSERT ON libraries WHEN NEW.details_provider='tvdb' BEGIN SELECT RAISE(ABORT,'fixture configuration write failure'); END`,
	} {
		if _, err = db.ExecContext(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	notifications := 0
	h := New(Deps{Store: s.store, Scanner: sc, Settings: s.settings, Log: s.log, LibrariesChanged: func() { notifications++ }})
	body := map[string]any{"name": "Must not exist", "kind": "shows", "paths": []string{t.TempDir()}, "details_provider": "tvdb"}
	r := httptest.NewRequest("POST", "http://arc/api/v1/libraries", strings.NewReader(string(mustLibraryJSON(t, body))))
	r.Header.Set("Authorization", "Bearer owner")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 500 {
		t.Fatal("fixture configuration failure not reached", w.Code, w.Body.String())
	}
	libraries, err := s.store.ListLibraries(t.Context())
	if err != nil || len(libraries) != 0 || notifications != 0 {
		t.Fatalf("configuration failure left partial library: count=%d notifications=%d (%v)", len(libraries), notifications, err)
	}
}

func mustLibraryJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
