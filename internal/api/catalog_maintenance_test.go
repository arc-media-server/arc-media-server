package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalogReservationRejectsNewMutationRequests(t *testing.T) {
	s, _ := securityServer(t)
	release, ok := s.store.ReserveCatalogMutation()
	if !ok {
		t.Fatal("reservation unavailable")
	}
	defer release()
	reached := false
	h := s.logRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true; w.WriteHeader(204) }))
	r := httptest.NewRequest("PATCH", "http://arc/api/settings", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 503 || w.Header().Get("Retry-After") == "" || reached {
		t.Fatalf("mutation admitted: status=%d reached=%v", w.Code, reached)
	}
}
func TestCatalogReservationDoesNotAbortAlreadyActiveWork(t *testing.T) {
	s, _ := securityServer(t)
	release, ok := s.store.BeginCatalogWork()
	if !ok {
		t.Fatal("work unavailable")
	}
	if reserved, ok := s.store.ReserveCatalogMutation(); ok {
		reserved()
		t.Error("exclusive mutation admitted during active work")
	}
	release()
	reserved, ok := s.store.ReserveCatalogMutation()
	if !ok {
		t.Fatal("work did not release")
	}
	reserved()
}
