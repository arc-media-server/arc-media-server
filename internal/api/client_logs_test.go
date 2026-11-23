package api

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ams/internal/auth"
)

func TestClientLogPrivacyAndPermissions(t *testing.T) {
	s, viewer := securityServer(t)
	admin, err := s.store.CreateUser(t.Context(), "Owner", true, "slate", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"app-fixture", "admin-fixture"} {
		user := viewer
		if token == "admin-fixture" {
			user = admin
		}
		if err := s.store.CreateSession(t.Context(), auth.HashToken(token), user.ID, "password", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.SetSessionDevice(t.Context(), auth.HashToken("app-fixture"), "Test TV"); err != nil {
		t.Fatal(err)
	}
	if err := s.store.CreateToolSession(t.Context(), auth.HashToken("tool-fixture"), admin.ID, "Test Tool"); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	h := New(Deps{Store: s.store, Settings: s.settings, Log: slog.New(slog.NewTextHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))})
	call := func(method, path, token, body string, want int) string {
		t.Helper()
		req := httptest.NewRequest(method, "http://arc/api/v1/client-logs"+path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: got %d, want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w.Body.String()
	}
	batch := fmt.Sprintf(`{"platform":"roku","entries":[{"time_ms":%d,"level":"error","event":"playback_failed"}]}`, time.Now().UnixMilli())
	call("POST", "", "", batch, 401)
	call("POST", "", "admin-fixture", batch, 403)
	call("POST", "", "tool-fixture", batch, 403)
	call("POST", "", "app-fixture", batch+strings.Repeat(" ", clientLogMaxBytes), 413)
	for _, value := range []string{"Bearer secret-fixture", "https://private.test/cast?token=secret-fixture", "10.0.0.1:8099", "pin=1234", "password=secret-fixture"} {
		call("POST", "", "app-fixture", strings.Replace(batch, "playback_failed", value, 1), 400)
	}
	call("POST", "", "app-fixture", strings.Replace(batch, `"time_ms"`, `"authorization":"secret-fixture","time_ms"`, 1), 400)
	call("POST", "", "app-fixture", batch+" {}", 400)
	if got := call("GET", "", "admin-fixture", "", 200); got != "[]\n" {
		t.Fatal("an invalid upload was stored")
	}
	call("POST", "", "app-fixture", batch, 204)
	call("GET", "", "app-fixture", "", 403)
	call("GET", "/1", "app-fixture", "", 403)
	if got := call("GET", "/1", "admin-fixture", "", 200); !strings.Contains(got, "playback_failed") {
		t.Fatal("valid event missing")
	}
	if strings.Contains(log.String(), "secret-fixture") || strings.Contains(log.String(), "app-fixture") {
		t.Fatal("a secret appeared in logs")
	}
	if err := s.store.DeleteSession(t.Context(), auth.HashToken("app-fixture")); err != nil {
		t.Fatal(err)
	}
	call("GET", "/1", "admin-fixture", "", 404)
	call("POST", "", "app-fixture", batch, 401)
}
