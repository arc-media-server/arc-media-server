package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDNSRebindingDoesNotGrantHomeAccess(t *testing.T) {
	s, u := securityServer(t)
	for _, prefix := range []string{"/api", "/api/v1"} {
		r := httptest.NewRequest("GET", "http://rebind.attacker.example:8099"+prefix+"/auth/status", nil)
		r.RemoteAddr = "192.168.1.20:1234"
		r.Header.Set("Origin", "http://rebind.attacker.example:8099")
		w := httptest.NewRecorder()
		s.authStatus(w, r)
		var status authStatusDTO
		if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
			t.Fatal(err)
		}
		if status.Bypass || len(status.Profiles) > 0 {
			t.Errorf("%s: unknown Host granted home access: %s", prefix, w.Body.String())
		}
		r = httptest.NewRequest("POST", "http://rebind.attacker.example:8099"+prefix+"/auth/profile", strings.NewReader(`{"id":"`+u.ID+`"}`))
		r.RemoteAddr = "192.168.1.20:1234"
		w = httptest.NewRecorder()
		s.chooseProfile(w, r)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: unknown Host profile selection=%d", prefix, w.Code)
		}
	}
}
func TestDNSRebindingCannotRunSetup(t *testing.T) {
	s, _ := securityServer(t)
	r := httptest.NewRequest("POST", "http://rebind.attacker.example/api/auth/setup", strings.NewReader(`{}`))
	r.RemoteAddr = "192.168.1.20:1234"
	w := httptest.NewRecorder()
	s.setup(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("unknown Host setup reached handler: %d", w.Code)
	}
}
