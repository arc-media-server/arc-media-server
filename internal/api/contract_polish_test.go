package api

import (
	"ams/internal/auth"
	"ams/internal/events"
	"ams/internal/store"
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestErrorCorrelationConcurrentRequests(t *testing.T) {
	s, _ := securityServer(t)
	original := map[string]any{"error": "choose profile", "needs": "pin", "remaining": 2}
	h := s.correlateRequests(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 403, original) }))
	ids := make(chan string, 40)
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("GET", "http://arc/api/v1/identity", nil)
			r.Header.Set(requestIDHeader, "caller-controlled")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			var got map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Error(err)
				return
			}
			id := w.Header().Get(requestIDHeader)
			if n, err := strconv.ParseUint(id, 10, 64); err != nil || n == 0 {
				t.Errorf("invalid ID %q", id)
			}
			if got["request_id"] != id || got["code"] != "forbidden" || got["error"] != original["error"] || got["needs"] != "pin" || got["remaining"] != float64(2) {
				t.Errorf("lost error fields: %v", got)
			}
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)
	seen := map[string]bool{}
	for id := range ids {
		if seen[id] {
			t.Fatal("duplicate ID", id)
		}
		seen[id] = true
	}
	if _, ok := original["code"]; ok {
		t.Fatal("mutated handler map")
	}
	w := httptest.NewRecorder()
	w.Header().Set(requestIDHeader, "42")
	writeJSON(w, 200, map[string]string{"error": "success data"})
	if strings.Contains(w.Body.String(), "request_id") || strings.Contains(w.Body.String(), "code") {
		t.Fatal("changed success payload")
	}
	w = httptest.NewRecorder()
	writeJSON(w, 409, map[string]string{"error": "changed", "code": "revision_conflict"})
	if !strings.Contains(w.Body.String(), "revision_conflict") {
		t.Fatal("replaced domain code")
	}
}

func TestAppCORSEventReconnectHeaders(t *testing.T) {
	h, _ := appCORSHandler(t)
	r := httptest.NewRequest("OPTIONS", "http://arc/api/v1/events", nil)
	r.Header.Set("Origin", "https://client.example")
	r.Header.Set("Access-Control-Request-Method", "GET")
	r.Header.Set("Access-Control-Request-Headers", "authorization,last-event-id")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 || !strings.Contains(w.Header().Get("Access-Control-Allow-Headers"), "Last-Event-ID") || !strings.Contains(w.Header().Get("Access-Control-Expose-Headers"), requestIDHeader) || w.Header().Get(requestIDHeader) == "" {
		t.Fatal(w.Code, w.Header())
	}
}

func TestCrossOriginEventReplayRetainsPermissions(t *testing.T) {
	s, u := securityServer(t)
	if err := s.store.CreateSession(t.Context(), auth.HashToken("replay-token"), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	allowed := []int64{3}
	if err := s.store.UpdateUser(t.Context(), u.ID, store.UserUpdate{LibraryIDs: &allowed}); err != nil {
		t.Fatal(err)
	}
	s.events = events.New()
	_, cursor, _ := s.events.Read("")
	s.events.Publish(events.Event{Topic: "private", UserID: "other"})
	s.events.Publish(events.Event{Topic: "admin", Admin: true})
	s.events.Publish(events.Event{Topic: "library", LibraryID: 999})
	s.events.Publish(events.Event{Topic: "profile", UserID: u.ID})
	server := httptest.NewServer(s.correlateRequests(appCORS(s.signedIn(s.eventStream))))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/v1/events", nil)
	req.Header.Set("Origin", "https://client.example")
	req.Header.Set("Authorization", "Bearer replay-token")
	req.Header.Set("Last-Event-ID", cursor)
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Access-Control-Allow-Origin") != "*" || resp.Header.Get(requestIDHeader) == "" {
		t.Fatal(resp.Status, resp.Header)
	}
	reader := bufio.NewReader(resp.Body)
	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		frame.WriteString(line)
		if line == "\n" {
			break
		}
	}
	text := frame.String()
	if !strings.Contains(text, `"topic":"profile"`) || strings.Contains(text, "resync") || strings.Contains(text, "private") || strings.Contains(text, "admin") || strings.Contains(text, "library_id") || strings.Contains(text, "UserID") || strings.Contains(text, u.ID) {
		t.Fatal("unexpected replay", text)
	}
}

// Exercise the guide's authenticated read-only requests at the real router boundary.
func TestClientGuideReadOnlyRequests(t *testing.T) {
	h, _ := appCORSHandler(t)
	for _, path := range []string{"/identity", "/capabilities", "/items?limit=20&offset=0&sort=added&reverse=1"} {
		r := httptest.NewRequest("GET", "http://arc/api/v1"+path, nil)
		r.Header.Set("Authorization", "Bearer tv-token")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || !json.Valid(w.Body.Bytes()) || w.Header().Get(requestIDHeader) == "" {
			t.Fatal(path, w.Code, w.Body.String())
		}
	}
	r := httptest.NewRequest("GET", "http://arc/api/v1/identity", nil)
	r.Header.Set("Origin", "https://client.example") // no local login bypass
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if w.Code != 401 || got["code"] != "authentication_required" || got["request_id"] != w.Header().Get(requestIDHeader) {
		t.Fatal(w.Code, got)
	}
}
