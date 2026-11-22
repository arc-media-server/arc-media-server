package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/auth"
)

// A profile sets, changes and removes its own PIN; changing an existing one needs it first.
func TestOwnPIN(t *testing.T) {
	s, u := securityServer(t)
	put := func(body string) int {
		fresh, err := s.store.GetUser(context.Background(), u.ID)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("PUT", "http://arc/api/me/pin", strings.NewReader(body))
		r = r.WithContext(context.WithValue(r.Context(), userKey, fresh))
		w := httptest.NewRecorder()
		s.setOwnPIN(w, r)
		return w.Code
	}
	pinIs := func(pin string) bool {
		got, _ := s.store.GetUser(context.Background(), u.ID)
		if pin == "" {
			return !got.HasPIN
		}
		return got.HasPIN && auth.Check(got.PINHash, pin)
	}
	if c := put(`{"pin":"12a4"}`); c != 400 {
		t.Fatalf("bad PIN: %d", c)
	}
	if c := put(`{"pin":"1234"}`); c != 200 || !pinIs("1234") {
		t.Fatalf("set: %d", c)
	}
	if c := put(`{"current":"0000","pin":"5678"}`); c != 401 || !pinIs("1234") {
		t.Fatalf("wrong current PIN: %d", c)
	}
	if c := put(`{"current":"1234","pin":"5678"}`); c != 200 || !pinIs("5678") {
		t.Fatalf("change: %d", c)
	}
	if c := put(`{"current":"5678","pin":""}`); c != 200 || !pinIs("") {
		t.Fatalf("remove: %d", c)
	}
}
