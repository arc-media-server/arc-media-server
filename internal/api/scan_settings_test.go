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
	s.applySettings = func(_ context.Context, _ settings.Settings, next settings.Settings) error { current = next; return nil }
	for _, body := range []string{
		`{"scan_interval_hours":3}`, `{"scan_interval_hours":-1}`, `{"scan_interval_hours":1.5}`,
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

func TestBackgroundSettingsStrictValidationAndMinutePatch(t *testing.T) {
	s, _ := securityServer(t)
	s.ffprobeStatus = func() any { return nil }
	current := settings.Settings{ScanIntervalHours: 1, PreviewMode: "scheduled", DetectionMode: "scheduled"}
	s.settings = func() settings.Settings { return current }
	s.applySettings = func(_ context.Context, _ settings.Settings, next settings.Settings) error { current = next; return nil }
	for _, body := range []string{`{"database_upkeep_enabled":null}`, `{"cache_cleanup_enabled":1}`, `{"database_upkeep_enabled":"true"}`, `{"empty_trash_enabled":null}`, `{"empty_trash_enabled":"true"}`, `{"empty_trash_enabled":1}`, `{"preview_mode":null}`, `{"preview_mode":"always"}`, `{"detection_mode":false}`, `{"analysis_enabled":null}`, `{"metadata_refresh_enabled":1}`, `{"auto_backups_enabled":"false"}`, `{"scan_interval_minutes":1}`, `{"scan_interval_minutes":15.5}`, `{"scan_interval_minutes":null}`} {
		w := httptest.NewRecorder()
		s.updateSettings(w, httptest.NewRequest("PATCH", "/api/settings", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("accepted %s: %d", body, w.Code)
		}
	}
	for _, body := range []string{`{"scan_interval_minutes":15,"preview_mode":"off","detection_mode":"added"}`, `{"analysis_enabled":false,"metadata_refresh_enabled":false,"auto_backups_enabled":false,"empty_trash_enabled":true,"database_upkeep_enabled":true,"cache_cleanup_enabled":true}`} {
		w := httptest.NewRecorder()
		s.updateSettings(w, httptest.NewRequest("PATCH", "/api/settings", strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatalf("%s: %s", body, w.Body.String())
		}
	}
	if current.ScanMinutes() != 15 || current.PreviewMode != "off" || current.DetectionMode != "added" || current.DetectIntros != true || current.AnalysisEnabled || current.AutoBackupsEnabled || !current.EmptyTrashEnabled || !current.DatabaseUpkeepEnabled || !current.CacheCleanupEnabled {
		t.Fatalf("bad canonical patch: %+v", current)
	}
}

func TestFractionalLegacyHoursRoundTripForMinuteIntervals(t *testing.T) {
	s, _ := securityServer(t)
	s.ffprobeStatus = func() any { return nil }
	current := settings.Settings{}
	s.settings = func() settings.Settings { return current }
	s.applySettings = func(_ context.Context, _ settings.Settings, next settings.Settings) error { current = next; return nil }
	for _, hours := range []string{"0.25", "0.5", "2"} {
		w := httptest.NewRecorder()
		s.updateSettings(w, httptest.NewRequest("PATCH", "/api/settings", strings.NewReader(`{"scan_interval_hours":`+hours+`}`)))
		if w.Code != 200 {
			t.Fatalf("%s: %s", hours, w.Body.String())
		}
	}
}
