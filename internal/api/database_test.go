package api

import (
	"ams/internal/auth"
	"ams/internal/databaseconfig"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDatabaseManagementIsAdminOnlyAndRequiresExplicitDowntime(t *testing.T) {
	s, u := securityServer(t)
	s.database = databaseconfig.New(t.TempDir(), nil)
	token := "database-fixture-token"
	if err := s.store.CreateSession(t.Context(), auth.HashToken(token), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{}`, `{"confirm_downtime":false}`, `{"confirm_downtime":true,"unexpected":"private-value"}`} {
		req := httptest.NewRequest("POST", "http://arc/api/database/transfer", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.adminOnly(s.databaseTransfer)(w, req)
		if w.Code != 403 {
			t.Fatalf("ordinary profile admitted: %d %s", w.Code, w.Body.String())
		}
	}
	admin, err := s.store.CreateUser(t.Context(), "Admin", true, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetSessionUser(t.Context(), auth.HashToken(token), admin.ID); err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{}`, `{"confirm_downtime":false}`, `{"confirm_downtime":true,"unexpected":"private-value"}`, `{"confirm_downtime":true} {}`} {
		req := httptest.NewRequest("POST", "http://arc/api/database/transfer", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.adminOnly(s.databaseTransfer)(w, req)
		if w.Code != 400 {
			t.Fatalf("invalid/unconfirmed transfer admitted: %d %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "private-value") {
			t.Fatal("request contents echoed")
		}
	}
	req := httptest.NewRequest("GET", "http://arc/api/database", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.adminOnly(s.databaseStatus)(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"backend":"sqlite"`) || strings.Contains(w.Body.String(), "password") {
		t.Fatal(w.Code, w.Body.String())
	}
}
