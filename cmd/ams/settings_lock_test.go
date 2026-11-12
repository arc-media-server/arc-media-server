package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSettingsLockProcess(t *testing.T) {
	root := os.Getenv("ARC_SETTINGS_LOCK_TEST_DATA")
	if root == "" {
		return
	}
	if err := run(root, "none", slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestSlowProviderValidationDoesNotBlockReadsOrOverwriteNewSettings(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var arrive, unblock sync.Once
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrive.Do(func() { close(entered) })
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{}`)
	}))
	defer provider.Close()
	defer unblock.Do(func() { close(release) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	process := exec.Command(binary, "-test.run=^TestSettingsLockProcess$")
	// Only isolated data, local provider and prepared executable paths: never inherit owner overrides.
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "AMS_") && !strings.HasPrefix(entry, "ARC_SETTINGS_LOCK_TEST_DATA=") {
			process.Env = append(process.Env, entry)
		}
	}
	process.Env = append(process.Env, "ARC_SETTINGS_LOCK_TEST_DATA="+t.TempDir(), "AMS_LISTEN="+address, "AMS_TMDB_URL="+provider.URL)
	var diagnostics strings.Builder
	process.Stdout, process.Stderr = io.Discard, &diagnostics
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan error, 1)
	go func() { stopped <- process.Wait() }()
	defer func() {
		unblock.Do(func() { close(release) })
		signalErr := process.Process.Signal(os.Interrupt)
		select {
		case err := <-stopped:
			if signalErr == nil && err != nil {
				t.Errorf("isolated server shutdown failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			process.Process.Kill()
			<-stopped
		}
		if strings.Contains(diagnostics.String(), "WARNING: DATA RACE") {
			t.Error("isolated server reported a data race")
		}
	}()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	base := "http://" + address
	ready := false
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(base + "/api/auth/status")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("isolated server not ready")
	}
	request := func(client *http.Client, method, path, body string) (int, []byte, error) {
		req, err := http.NewRequestWithContext(context.Background(), method, base+path, strings.NewReader(body))
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		return response.StatusCode, data, err
	}
	if status, data, err := request(client, "POST", "/api/auth/setup", `{"name":"Fixture owner","password":"Arc-fixture-password-123","local_bypass":false}`); err != nil || status != http.StatusCreated {
		t.Fatalf("setup failed: %d %s %v", status, data, err)
	}
	type result struct {
		status int
		err    error
	}
	saved := make(chan result, 1)
	slow := &http.Client{Jar: jar, Timeout: 20 * time.Second}
	go func() {
		status, _, err := request(slow, "PATCH", "/api/settings", `{"tmdb_api_key":"fixture-key-not-a-secret"}`)
		saved <- result{status, err}
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("provider validation did not start")
	}
	fast := &http.Client{Jar: jar, Timeout: 2 * time.Second}
	if status, _, err := request(fast, "GET", "/api/settings", ""); err != nil || status != 200 {
		t.Errorf("settings reads blocked by provider validation: status=%d err=%v", status, err)
	}
	if status, _, err := request(fast, "GET", "/api/libraries", ""); err != nil || status != 200 {
		t.Errorf("library requests blocked by provider validation: status=%d err=%v", status, err)
	}
	if status, _, err := request(fast, "PATCH", "/api/settings", `{"server_name":"Concurrent saved name"}`); err != nil || status != 200 {
		t.Errorf("unrelated save blocked: status=%d err=%v", status, err)
	}
	unblock.Do(func() { close(release) })
	select {
	case outcome := <-saved:
		if outcome.err != nil || outcome.status != 409 {
			t.Errorf("stale save was not rejected: %+v", outcome)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("validation did not finish")
	}
	status, data, err := request(client, "GET", "/api/settings", "")
	var config map[string]any
	json.Unmarshal(data, &config)
	if err != nil || status != 200 || config["server_name"] != "Concurrent saved name" {
		t.Errorf("concurrent settings lost: status=%d err=%v name=%v", status, err, config["server_name"])
	}
}
