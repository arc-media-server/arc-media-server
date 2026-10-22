package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/settings"
)

func TestScanSettingsPatchValidation(t *testing.T) {
	s, _ := securityServer(t)
	s.ffprobeStatus = func() any { return nil }
	current := settings.Settings{ScanAutomatic: true, ScanPartial: true, ScanOnStart: true, ScanIntervalHours: 1}
	s.settings = func() settings.Settings { return current }
	s.applySettings = func(_ context.Context, next settings.Settings) error { current = next; return nil }
	for _, body := range []string{
		`{"scan_interval_hours":2}`, `{"scan_interval_hours":-1}`, `{"scan_interval_hours":1.5}`,
		`{"scan_interval_hours":"6"}`, `{"scan_automatic":"false"}`, `{"scan_partial":1}`,
		`{"scan_on_start":null}`, `{"scan_automatic":null}`, `{"scan_interval_hours":null}`,
	} {
		w := httptest.NewRecorder()
		s.updateSettings(w, httptest.NewRequest("PATCH", "/api/settings", strings.NewReader(body)))
		if w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d %s", body, w.Code, w.Body.String())
		}
		if !current.ScanAutomatic || current.ScanIntervalHours != 1 {
			t.Fatal("invalid patch changed settings")
		}
	}
	for _, body := range []string{`{"scan_automatic":false}`, `{"scan_interval_hours":0}`, `{"scan_on_start":false}`} {
		w := httptest.NewRecorder()
		s.updateSettings(w, httptest.NewRequest("PATCH", "/api/settings", strings.NewReader(body)))
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", body, w.Code, w.Body.String())
		}
	}
	if current.ScanAutomatic || current.ScanOnStart || current.ScanIntervalHours != 0 || !current.ScanPartial {
		t.Fatalf("patch semantics: %+v", current)
	}
}
