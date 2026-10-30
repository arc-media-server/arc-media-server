package api

import (
	"ams/internal/auth"
	"ams/internal/events"
	"ams/internal/store"
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEventPermissions(t *testing.T) {
	u := &store.User{ID: "me", LibraryIDs: []int64{3}}
	for _, e := range []events.Event{{Topic: "profile", UserID: "other"}, {Topic: "activity", Admin: true}, {Topic: "library", LibraryID: 4}} {
		if eventAllowed(e, u) {
			t.Fatal("private hint allowed", e)
		}
	}
	for _, e := range []events.Event{{Topic: "profile", UserID: "me"}, {Topic: "library", LibraryID: 3}, {Topic: "resync"}} {
		if !eventAllowed(e, u) {
			t.Fatal("own hint denied", e)
		}
	}
}
func TestEventStreamReplayMutationAndRevocation(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	token := "event-fixture"
	if err := s.store.CreateSession(ctx, auth.HashToken(token), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	s.events = events.New()
	s.eventHeartbeat = 10 * time.Millisecond
	server := httptest.NewServer(s.signedIn(s.eventStream))
	defer server.Close()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal(resp.Status)
	}
	reader := bufio.NewReader(resp.Body)
	frame := func() string {
		var b strings.Builder
		for {
			line, e := reader.ReadString('\n')
			if e != nil {
				t.Fatal(e)
			}
			b.WriteString(line)
			if line == "\n" {
				return b.String()
			}
		}
	}
	if f := frame(); !strings.Contains(f, "event: resync") {
		t.Fatal(f)
	}
	// The normal request boundary publishes only after a successful mutation.
	r := httptest.NewRequest("PUT", "http://arc/api/items/fixture/watchlist", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	s.publishMutation(r, "")
	s.events.Publish(events.Event{Topic: "private-test", UserID: "someone-else"})
	s.events.Publish(events.Event{Topic: "admin-test", Admin: true})
	for {
		f := frame()
		if strings.Contains(f, "private-test") || strings.Contains(f, "admin-test") {
			t.Fatal("leaked", f)
		}
		if strings.Contains(f, `"topic":"profile"`) {
			break
		}
	}
	if err := s.store.DeleteSession(ctx, auth.HashToken(token)); err != nil {
		t.Fatal(err)
	}
	for {
		_, err = reader.ReadString('\n')
		if err != nil {
			break
		}
	}
}
