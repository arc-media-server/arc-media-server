package api

import (
	"ams/internal/auth"
	"ams/internal/settings"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingsLogsOnlyKnownCommittedChangesWithoutValues(t *testing.T) {
	s, _ := securityServer(t)
	owner, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.CreateSession(t.Context(), auth.HashToken("owner"), owner.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	cfg, err := settings.Load(t.Context(), s.store)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ServerName = "Arc"
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	fail := false
	h := New(Deps{Store: s.store, Log: log, Settings: func() settings.Settings { return cfg }, FFprobeStatus: func() any { return nil }, ApplySettings: func(_ context.Context, _ settings.Settings, next settings.Settings) error {
		if fail {
			return errors.New("fixture refusal")
		}
		cfg = next
		return nil
	}})
	request := func(body string) int {
		r := httptest.NewRequest("PATCH", "http://arc/api/v1/settings", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer owner")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if status := request(`{"server_name":"https://fixture.invalid/private?token=fixture-secret","unknown_value":"fixture-unknown-secret","https://fixture.invalid/key?token=unknown-key":"x"}`); status != 200 {
		t.Fatal(status, logs.String())
	}
	for _, secret := range []string{"https://fixture.invalid", "fixture-secret", "fixture-unknown-secret", "unknown-key", "unknown_value"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("settings log exposed submitted data: %s", secret)
		}
	}
	if !strings.Contains(logs.String(), "settings changed") {
		t.Fatal("changed outcome was not logged", logs.String())
	}
	logs.Reset()
	if status := request(`{"unknown_value":"ignored"}`); status != 200 {
		t.Fatal(status)
	}
	if strings.Contains(logs.String(), "settings changed") {
		t.Error("ignored input claimed a settings change", logs.String())
	}
	logs.Reset()
	if status := request(`{"server_name":"https://fixture.invalid/private?token=fixture-secret"}`); status != 200 {
		t.Fatal(status)
	}
	if strings.Contains(logs.String(), "settings changed") {
		t.Error("unchanged input claimed a settings change", logs.String())
	}
	logs.Reset()
	fail = true
	if status := request(`{"server_name":"Not saved"}`); status != 400 {
		t.Fatal(status)
	}
	if strings.Contains(logs.String(), "settings changed") {
		t.Error("failed save claimed a settings change", logs.String())
	}
}
