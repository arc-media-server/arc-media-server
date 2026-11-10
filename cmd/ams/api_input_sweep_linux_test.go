package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type sweepSchema struct {
	Type string
	Ref  string
}

func (s *sweepSchema) UnmarshalJSON(b []byte) error {
	var raw struct {
		Type string
		Ref  string `json:"$ref"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	s.Type, s.Ref = raw.Type, raw.Ref
	return nil
}

type sweepContent struct{ Schema sweepSchema }
type sweepOperation struct {
	OperationID string                                       `json:"operationId"`
	RequestBody struct{ Content map[string]json.RawMessage } `json:"requestBody"`
	Responses   map[string]struct{ Content map[string]sweepContent }
}
type sweepOutput struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *sweepOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}
func (b *sweepOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.data.String() }

// TestAPIInvalidInputSweep is intentionally opt-in: it builds and starts the real
// server with race detection, a disposable store and only loopback provider fixtures.
// Bodyless commands and live event streams have no malformed input to exercise.
func TestAPIInvalidInputSweep(t *testing.T) {
	if os.Getenv("ARC_RUN_API_INPUT_SWEEP") != "1" {
		t.Skip("opt-in real-server bad-input sweep")
	}
	sweepCtx, stopSweep := context.WithTimeout(t.Context(), 2*time.Minute)
	defer stopSweep()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths      map[string]map[string]sweepOperation
		Components struct{ Schemas map[string]sweepSchema }
	}
	raw, err := os.ReadFile(filepath.Join(root, "docs", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "arc-sweep")
	build := exec.CommandContext(sweepCtx, filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-race", "-o", binary, "./cmd/ams")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real server: %v %s", err, out)
	}
	var providerCalls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls.Add(1)
		w.WriteHeader(503)
		_, _ = io.WriteString(w, "local unavailable fixture")
	}))
	defer provider.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "AMS_") {
			env = append(env, entry)
		}
	}
	env = append(env, "AMS_LISTEN="+address)
	for _, name := range []string{"AMS_TMDB_URL", "AMS_TVDB_URL", "AMS_OMDB_URL", "AMS_PLEX_TV_URL", "AMS_OPENSUBTITLES_URL", "AMS_PUBLIC_IP_URL", "AMS_UPNP_URL", "AMS_ACME_URL"} {
		env = append(env, name+"="+provider.URL)
	}
	var output sweepOutput
	// Keep the creating thread alive: Linux sends the child a death signal if
	// this test process is interrupted, so an aborted run cannot leave Arc serving.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	server := exec.CommandContext(sweepCtx, binary, "-data", t.TempDir(), "-config", "none")
	server.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
	server.Dir = root
	server.Env = env
	server.Stdout = &output
	server.Stderr = &output
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- server.Wait() }()
	defer func() {
		_ = server.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-exited:
			if err != nil {
				t.Errorf("fixture server exit: %v", err)
			}
		case <-time.After(10 * time.Second):
			_ = server.Process.Kill()
			<-exited
			t.Error("fixture server exceeded shutdown deadline")
		}
		if strings.Contains(output.String(), "WARNING: DATA RACE") {
			t.Error("real fixture server reported a race")
		}
	}()
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	base := "http://" + address
	deadline := time.Now().Add(20 * time.Second)
	for {
		response, err := client.Get(base + "/api/health")
		if err == nil {
			io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("temporary server did not become healthy: %s", output.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
	setup, err := client.Post(base+"/api/auth/setup", "application/json", strings.NewReader("{\"name\":\"Sweep fixture\",\"password\":\"fixture-password-only\",\"local_bypass\":false}"))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, setup.Body)
	setup.Body.Close()
	if setup.StatusCode != 200 && setup.StatusCode != 201 {
		t.Fatalf("fixture setup status=%d", setup.StatusCode)
	}
	token := ""
	for _, cookie := range setup.Cookies() {
		if cookie.Name == "ams_session" {
			token = cookie.Value
		}
	}
	if token == "" {
		t.Fatal("fixture setup did not issue a session")
	}
	type request struct {
		method, path string
		operation    sweepOperation
	}
	requests := []request{}
	for path, methods := range spec.Paths {
		for method, op := range methods {
			switch method {
			case "get", "post", "put", "patch", "delete":
				requests = append(requests, request{strings.ToUpper(method), path, op})
			}
		}
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].path+requests[i].method < requests[j].path+requests[j].method })
	params := regexp.MustCompile(`\{[^}]+\}`)
	passed, skipped := 0, 0
	statuses := map[int]int{}
	for _, entry := range requests {
		_, jsonBody := entry.operation.RequestBody.Content["application/json"]
		hasParams := params.MatchString(entry.path)
		eventStream := false
		for _, r := range entry.operation.Responses {
			for ct := range r.Content {
				if ct == "text/event-stream" {
					eventStream = true
				}
			}
		}
		if (entry.method != "GET" && !jsonBody && !hasParams) || (eventStream && !hasParams) {
			skipped++
			t.Logf("skip %s %s: bodyless command or live stream; positive behavior has separate tests", entry.method, entry.path)
			continue
		}
		for _, prefix := range []string{"/api", "/api/v1"} {
			path := params.ReplaceAllString(entry.path, "not-found")
			query := url.Values{"limit": {"not-a-limit"}, "offset": {"-1"}, "sort": {"not-a-sort"}, "cursor": {"not-a-cursor"}, "path": {filepath.Join(t.TempDir(), "absent")}}
			target := base + prefix + path
			if entry.method == "GET" {
				target += "?" + query.Encode()
			}
			body := io.Reader(nil)
			if jsonBody && entry.method != "GET" {
				body = strings.NewReader("{\"invalid\":")
			}
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			req, err := http.NewRequestWithContext(ctx, entry.method, target, body)
			if err != nil {
				cancel()
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			response, err := client.Do(req)
			if err != nil {
				cancel()
				t.Errorf("%s %s%s transport failure: %v", entry.method, prefix, entry.path, err)
				continue
			}
			data, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
			response.Body.Close()
			cancel()
			if readErr != nil {
				t.Errorf("%s %s%s response read: %v", entry.method, prefix, entry.path, readErr)
				continue
			}
			status := response.StatusCode
			statuses[status]++
			if status == 500 || status == 501 || status >= 505 {
				t.Errorf("%s %s%s unexpected server failure: %d %.200s", entry.method, prefix, entry.path, status, data)
				continue
			}
			if status >= 200 && status < 300 {
				content := entry.operation.Responses[strconv.Itoa(status)].Content["application/json"]
				schema := content.Schema
				if schema.Ref != "" {
					schema = spec.Components.Schemas[strings.TrimPrefix(schema.Ref, "#/components/schemas/")]
				}
				if schema.Type == "array" && bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
					t.Errorf("%s %s%s returned null for a top-level array", entry.method, prefix, entry.path)
					continue
				}
			}
			// 502/503/504 are permitted for unavailable fixture providers/features, not
			// counted as successful feature execution. An unavailable service is not panic.
			passed++
		}
	}
	// Invalid authentication inputs may legitimately return 401. Verify the
	// fixture session itself still works, rather than forbidding that status.
	probe, err := http.NewRequestWithContext(sweepCtx, "GET", base+"/api/profiles", nil)
	if err != nil {
		t.Fatal(err)
	}
	probe.Header.Set("Authorization", "Bearer "+token)
	verified, err := client.Do(probe)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, verified.Body)
	verified.Body.Close()
	if verified.StatusCode != 200 {
		t.Fatalf("fixture authentication was lost: %d", verified.StatusCode)
	}
	t.Logf("sweep_result passed_requests=%d failed_requests=%d skipped_operations=%d total_operations=%d fixture_provider_calls=%d statuses=%v", passed, len(requests)*2-skipped*2-passed, skipped, len(requests), providerCalls.Load(), statuses)
	if passed < 200 {
		t.Fatal("route coverage unexpectedly small")
	}
}
