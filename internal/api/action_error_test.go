package api

import (
	"ams/internal/auth"
	"ams/internal/background"
	"ams/internal/backup"
	"ams/internal/logbuf"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/schedule"
	"ams/internal/settings"
	"ams/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type failStoreBody struct {
	io.Reader
	once  sync.Once
	close func()
}

func (b *failStoreBody) Read(p []byte) (int, error) { b.once.Do(b.close); return b.Reader.Read(p) }

func TestActionInternalFailuresAreNotClientErrorsOrLeakedAndLogOnce(t *testing.T) {
	for _, kind := range []string{"playlist create", "playlist add", "collection add", "library create", "match", "maintenance", "backup restore", "backup delete"} {
		t.Run(kind, func(t *testing.T) {
			s, _ := securityServer(t)
			ctx := t.Context()
			owner, err := s.store.CreateUser(ctx, "Owner", true, "blue", "", "")
			if err != nil {
				t.Fatal(err)
			}
			logs := logbuf.New(64)
			s.logs = logs
			s.log = slog.New(logs.Wrap(slog.NewTextHandler(io.Discard, nil)))
			root := t.TempDir()
			lib, err := s.store.SyncLibrary(ctx, "Fixture", "movies", "tmdb_aired", []string{root})
			if err != nil {
				t.Fatal(err)
			}
			item, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "movie", Title: "Fixture"}, "error-fixture")
			if err != nil {
				t.Fatal(err)
			}
			file, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: filepath.Join(root, "fixture.mp4"), Size: 1, ModTime: 1}, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.store.SetFileItems(ctx, file, []string{item}, 0); err != nil {
				t.Fatal(err)
			}
			playlist, err := s.store.CreatePlaylist(ctx, owner.ID, "Fixture")
			if err != nil {
				t.Fatal(err)
			}
			collection := store.Collection{LibraryID: lib, Kind: "manual", Title: "Fixture"}
			if err = s.store.CreateCollection(ctx, &collection); err != nil {
				t.Fatal(err)
			}
			s.scanner = scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
			s.background, err = background.New(ctx, s.store, schedule.New(0, 24), nil)
			if err != nil {
				t.Fatal(err)
			}
			s.background.Configure(settings.Settings{AnalysisEnabled: true})
			s.backups = backup.New(filepath.Join(t.TempDir(), "backups"), s.store, s.log)
			var b backup.Backup
			if strings.HasPrefix(kind, "backup") {
				b, err = s.backups.Create(ctx, backup.ReasonManual)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err = s.store.CreateSession(ctx, auth.HashToken("action-error-fixture"), owner.ID, "password", ""); err != nil {
				t.Fatal(err)
			}
			h := New(Deps{Store: s.store, Scanner: s.scanner, Background: s.background, Backups: s.backups, Settings: s.settings, Logs: logs, Log: s.log})
			method, path, body := "POST", "", ""
			direct := false
			switch kind {
			case "playlist create":
				path = "/api/v1/playlists"
				body = fmt.Sprintf(`{"title":"New list","item_ids":[%q]}`, item)
			case "playlist add":
				path = "/api/v1/playlists/" + playlist.ID + "/items"
				body = fmt.Sprintf(`{"item_ids":[%q]}`, item)
			case "collection add":
				path = "/api/v1/collections/" + collection.ID + "/items"
				body = fmt.Sprintf(`{"item_ids":[%q]}`, item)
			case "library create":
				path = "/api/v1/libraries"
				body = fmt.Sprintf(`{"name":"New library","kind":"movies","paths":[%q]}`, root)
			case "match":
				method = "PUT"
				path = "/api/v1/items/" + item + "/match"
				body = `{"provider":"tmdb","id":123}`
			case "maintenance":
				path = "/api/v1/maintenance/analysis/run"
				direct = true
			case "backup restore":
				path = "/api/v1/backups/" + b.Name + "/restore"
				direct = true
			case "backup delete":
				method = "DELETE"
				path = "/api/v1/backups/" + b.Name
				if err = os.RemoveAll(s.backups.Dir()); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(s.backups.Dir(), []byte("not a directory"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var reader io.Reader = strings.NewReader(body)
			if body != "" {
				reader = &failStoreBody{Reader: reader, close: func() {
					if err := s.store.Close(); err != nil {
						t.Error(err)
					}
				}}
			}
			r := httptest.NewRequest(method, path, reader).WithContext(context.WithValue(ctx, userKey, owner))
			r.Header.Set("Authorization", "Bearer action-error-fixture")
			w := httptest.NewRecorder()
			if direct {
				// The failure occurs after authentication in production; exercise the already-authenticated
				// handler directly so the unrelated session-read failure does not hide this boundary.
				if err = s.store.Close(); err != nil {
					t.Fatal(err)
				}
				if kind == "maintenance" {
					r.SetPathValue("kind", "analysis")
					s.runMaintenance(w, r)
				} else {
					r.SetPathValue("name", b.Name)
					s.restoreBackup(w, r)
				}
			} else {
				h.ServeHTTP(w, r)
			}
			if w.Code != http.StatusInternalServerError {
				t.Errorf("internal failure returned %d: %s", w.Code, w.Body.String())
			}
			var response map[string]any
			if err = json.Unmarshal(w.Body.Bytes(), &response); err != nil || response["error"] != "internal error" {
				t.Errorf("internal text leaked or response not generic: %s (%v)", w.Body.String(), err)
			}
			lines := logs.Lines(slog.LevelWarn, 0)
			want := 1
			if kind == "backup restore" {
				// Catalog maintenance must also report failure to resume queued scans.
				// This is a distinct recovery operation, not a second request summary.
				want = 2
				if len(lines) == 2 && (lines[1].Level != "WARN" || lines[1].Message != "resuming queued scans failed") {
					t.Errorf("unexpected recovery diagnostic: %+v", lines[1])
				}
			}
			if len(lines) != want || len(lines) == 0 || lines[0].Level != "ERROR" || lines[0].Message != "request failed" {
				t.Errorf("expected one request failure and %d recovery diagnostic(s), got %+v", want-1, lines)
			}
		})
	}
}

func TestRestoreRuntimeFailureStatesRestoredWithoutLeakingDetail(t *testing.T) {
	s, _ := securityServer(t)
	logs := logbuf.New(32)
	s.log = slog.New(logs.Wrap(slog.NewTextHandler(io.Discard, nil)))
	s.scanner = scanner.New(t.Context(), s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
	s.backups = backup.New(filepath.Join(t.TempDir(), "backups"), s.store, s.log)
	b, err := s.backups.Create(t.Context(), backup.ReasonManual)
	if err != nil {
		t.Fatal(err)
	}
	const marker = "private-runtime-failure-fixture"
	s.afterRestore = func(context.Context) error { return fmt.Errorf("%s", marker) }
	r := httptest.NewRequest("POST", "/", nil)
	r.SetPathValue("name", b.Name)
	w := &statusRecorder{ResponseWriter: httptest.NewRecorder()}
	s.restoreBackup(w, r)
	record := w.ResponseWriter.(*httptest.ResponseRecorder)
	if record.Code != 500 || !strings.Contains(record.Body.String(), "restored") || !strings.Contains(record.Body.String(), "Restart Arc") {
		t.Fatal(record.Code, record.Body.String())
	}
	if strings.Contains(record.Body.String(), marker) {
		t.Error("runtime reload detail leaked in response")
	}
	if !w.failureLogged || len(logs.Lines(slog.LevelError, 0)) != 1 {
		t.Error("runtime reload failure not marked as logged once")
	}
}

func TestExpectedActionValidationKeepsClientStatuses(t *testing.T) {
	for _, kind := range []string{"playlist", "smart rules", "collection", "library", "maintenance", "backup invalid", "backup missing", "backup corrupt", "delete invalid", "delete missing"} {
		t.Run(kind, func(t *testing.T) {
			s, u := securityServer(t)
			ctx := t.Context()
			logs := logbuf.New(32)
			s.log = slog.New(logs.Wrap(slog.NewTextHandler(io.Discard, nil)))
			lib, err := s.store.SyncLibrary(ctx, "Fixture", "movies", "tmdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			c := store.Collection{LibraryID: lib, Kind: "manual", Title: "Fixture"}
			if err = s.store.CreateCollection(ctx, &c); err != nil {
				t.Fatal(err)
			}
			s.scanner = scanner.New(ctx, s.store, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), s.log)
			s.background, err = background.New(ctx, s.store, schedule.New(0, 24), nil)
			if err != nil {
				t.Fatal(err)
			}
			s.backups = backup.New(filepath.Join(t.TempDir(), "backups"), s.store, s.log)
			var body string
			want := 400
			switch kind {
			case "playlist":
				body = `{"title":"Fixture","item_ids":["missing"]}`
			case "smart rules":
				body = `{"title":"Fixture","filter":{"library_id":1,"video_codec":"invalid codec"}}`
			case "collection":
				body = `{"item_ids":["missing"]}`
			case "library":
				body = `{"name":"Fixture","kind":"invalid"}`
			}
			r := httptest.NewRequest("POST", "/", strings.NewReader(body)).WithContext(context.WithValue(ctx, userKey, u))
			w := httptest.NewRecorder()
			switch kind {
			case "playlist", "smart rules":
				s.createPlaylist(w, r)
			case "collection":
				r.SetPathValue("id", c.ID)
				s.collectionAddItems(w, r)
			case "library":
				s.createLibrary(w, r)
			case "maintenance":
				r.SetPathValue("kind", "unknown")
				s.runMaintenance(w, r)
			case "backup invalid":
				r.SetPathValue("name", "../not-a-backup")
				s.restoreBackup(w, r)
			case "delete invalid":
				r.SetPathValue("name", "../not-a-backup")
				s.deleteBackup(w, r)
			default:
				b, err := s.backups.Create(ctx, backup.ReasonManual)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "backup corrupt" {
					err = os.WriteFile(filepath.Join(s.backups.Dir(), b.Name), []byte("not sqlite"), 0600)
				} else {
					err = os.Remove(filepath.Join(s.backups.Dir(), b.Name))
					want = 404
				}
				if err != nil {
					t.Fatal(err)
				}
				r.SetPathValue("name", b.Name)
				if kind == "delete missing" {
					s.deleteBackup(w, r)
				} else {
					s.restoreBackup(w, r)
				}
			}
			if w.Code != want {
				t.Errorf("status=%d want %d: %s", w.Code, want, w.Body.String())
			}
			if len(logs.Lines(slog.LevelWarn, 0)) != 0 {
				t.Error("ordinary validation logged as a server failure")
			}
		})
	}
}

func TestMatchProviderFailureKeeps502WithoutEchoOrDuplicateDiagnostic(t *testing.T) {
	s, u, _, item := optimizedAudioFixture(t, 0, 0, 720)
	const marker = "fixture-private-provider-message"
	fixture := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(401)
		fmt.Fprintf(w, `{"message":%q}`, marker)
	}))
	t.Cleanup(fixture.Close)
	previous := http.DefaultTransport
	transport := fixture.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "127.0.0.1"
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", fixture.Listener.Addr().String())
	}
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })
	logs := logbuf.New(64)
	s.log = slog.New(logs.Wrap(slog.NewTextHandler(io.Discard, nil)))
	s.scanner = scanner.New(t.Context(), s.store, probe.New(""), metadata.NewTMDB("fixture-not-a-real-key"), metadata.NewTVDB(""), s.log)
	r := httptest.NewRequest("PUT", "/", strings.NewReader(`{"provider":"tmdb","id":123}`)).WithContext(context.WithValue(t.Context(), userKey, u))
	r.SetPathValue("id", item)
	recorded := httptest.NewRecorder()
	w := &statusRecorder{ResponseWriter: recorded}
	s.setMatch(w, r)
	if recorded.Code != 502 || !strings.Contains(recorded.Body.String(), "401") {
		t.Fatal(recorded.Code, recorded.Body.String())
	}
	if strings.Contains(recorded.Body.String(), marker) || strings.Contains(fmt.Sprint(logs.Lines(slog.LevelWarn, 0)), marker) {
		t.Error("untrusted provider text echoed")
	}
	if !w.failureLogged || len(logs.Lines(slog.LevelWarn, 0)) != 1 {
		t.Error("provider failure not diagnosed once")
	}
}
