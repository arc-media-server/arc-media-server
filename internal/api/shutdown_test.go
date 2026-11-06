package api

import (
	"ams/internal/auth"
	"ams/internal/events"
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// An open events stream (a browser tab) must not hold a restart: http.Server.Shutdown waits for
// active connections, and Docker kills a container ten seconds after asking it to stop.
func TestShutdownEndsEventStreams(t *testing.T) {
	s, u := securityServer(t)
	token := "shutdown-fixture"
	if err := s.store.CreateSession(t.Context(), auth.HashToken(token), u.ID, "password", ""); err != nil {
		t.Fatal(err)
	}
	s.events = events.New()
	closing := make(chan struct{})
	server := httptest.NewUnstartedServer(s.signedIn(s.eventStream))
	server.Config.BaseContext = func(net.Listener) context.Context { return ShutdownContext(closing) }
	server.Config.RegisterOnShutdown(func() { close(closing) })
	server.Start()
	defer server.Close()

	req, _ := http.NewRequestWithContext(t.Context(), "GET", server.URL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if _, err = bufio.NewReader(resp.Body).ReadString('\n'); err != nil { // the stream is open
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started := time.Now()
	if err := server.Config.Shutdown(ctx); err != nil {
		t.Fatal("shutdown waited for the open stream:", err)
	}
	if took := time.Since(started); took > 2*time.Second {
		t.Fatalf("shutdown took %v with a stream open", took)
	}
}
