package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/settings"
	"ams/internal/store"
)

func TestIdentityPersistsAcrossRenameRestartAndRestore(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "arc.db")
	st, err := store.Open(ctx, path, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	original, err := st.Setting(ctx, "server_id")
	if err != nil || len(original) != 32 {
		t.Fatalf("id=%q %v", original, err)
	}
	if err = st.SetSetting(ctx, "server_name", "My Arc"); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "safety.db")
	if err = st.BackupTo(ctx, backup); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	st, err = store.Open(ctx, path, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	id, err := st.Setting(ctx, "server_id")
	if err != nil || id != original {
		t.Fatal("restart changed identity", err)
	}
	cfg, err := settings.Load(ctx, st)
	if err != nil || cfg.ServerName != "My Arc" {
		t.Fatalf("name not loaded %+v %v", cfg, err)
	}
	cfg.ServerName = "Renamed"
	if err = settings.Save(ctx, st, cfg); err != nil {
		t.Fatal(err)
	}
	if err = st.RestoreFrom(ctx, backup); err != nil {
		t.Fatal(err)
	}
	id, err = st.Setting(ctx, "server_id")
	if err != nil || id != original {
		t.Fatal("restore changed identity", err)
	}
	cfg, err = settings.Load(ctx, st)
	if err != nil || cfg.ServerName != "My Arc" {
		t.Fatal("restore lost name", err)
	}
	other, err := store.Open(ctx, filepath.Join(t.TempDir(), "other.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	second, _ := other.Setting(ctx, "server_id")
	if second == original {
		t.Fatal("different servers have same identity")
	}
}

func TestIdentitySettingsValidationAndCapabilityPermissions(t *testing.T) {
	s, u := securityServer(t)
	s.ffprobeStatus = func() any { return nil }
	cfg := settings.Settings{ServerName: "Original"}
	s.settings = func() settings.Settings { return cfg }
	s.applySettings = func(_ context.Context, next settings.Settings) error { cfg = next; return nil }
	for _, body := range []string{`{"server_name":""}`, `{"server_name":null}`, `{"server_name":"bad\nname"}`, `{"server_name":false}`, `{"server_name":"` + strings.Repeat("x", 81) + `"}`} {
		w := httptest.NewRecorder()
		s.updateSettings(w, httptest.NewRequest("PATCH", "/", strings.NewReader(body)))
		if w.Code != 400 || cfg.ServerName != "Original" {
			t.Fatalf("invalid name accepted: %s %d", body, w.Code)
		}
	}
	w := httptest.NewRecorder()
	s.updateSettings(w, httptest.NewRequest("PATCH", "/", strings.NewReader(`{"server_name":"  Living Room  "}`)))
	if w.Code != 200 || cfg.ServerName != "Living Room" {
		t.Fatalf("rename %d %s", w.Code, w.Body.String())
	}
	var logs bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&logs, nil))
	for _, admin := range []bool{false, true} {
		u.IsAdmin = admin
		r := httptest.NewRequest("GET", "/", nil).WithContext(context.WithValue(t.Context(), userKey, u))
		w = httptest.NewRecorder()
		s.capabilities(w, r)
		var got serverCapabilities
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Features["metadata_edit"].Allowed != admin || !got.Features["browse"].Allowed || got.Features["transcoding"].Available {
			t.Fatalf("capabilities %+v", got)
		}
		w = httptest.NewRecorder()
		s.identity(w, r)
		var identity serverIdentity
		if err := json.Unmarshal(w.Body.Bytes(), &identity); err != nil {
			t.Fatal(err)
		}
		if len(identity.ID) != 32 || identity.Name != "Living Room" {
			t.Fatalf("identity %+v", identity)
		}
	}
	if logs.Len() != 0 {
		t.Fatal("polling produced logs")
	}
	// Real router wrappers reject unauthenticated callers (health remains public).
	s.settings = func() settings.Settings { return settings.Settings{} }
	for _, fn := range []http.HandlerFunc{s.identity, s.capabilities} {
		w = httptest.NewRecorder()
		s.signedIn(fn)(w, httptest.NewRequest("GET", "http://arc/api/identity", nil))
		if w.Code != 401 {
			t.Fatalf("identity access %d", w.Code)
		}
	}
}
