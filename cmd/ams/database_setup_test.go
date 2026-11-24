package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPostgresDatabaseOnboarding(t *testing.T) {
	dsn := os.Getenv("ARC_TEST_POSTGRES_ONBOARDING")
	if dsn == "" {
		t.Skip("set ARC_TEST_POSTGRES_ONBOARDING to a dedicated empty fixture database")
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "AMS_") {
			t.Setenv(key, "")
		}
	}
	conn, err := url.Parse(dsn)
	if err != nil || conn.Hostname() != "127.0.0.1" || conn.User == nil {
		t.Fatal("expected a loopback PostgreSQL fixture URL")
	}
	port, err := strconv.Atoi(conn.Port())
	if err != nil {
		t.Fatal("fixture requires an explicit PostgreSQL port")
	}
	pass, _ := conn.User.Password()
	address := testDatabaseHTTPAddress(t)
	t.Setenv("AMS_LISTEN", address)
	dir := t.TempDir()
	client, base := testDatabaseClient(t, address)

	start := func(want int) func() {
		t.Helper()
		ctx, cancel := context.WithCancel(t.Context())
		finished := make(chan error, 1)
		go func() {
			finished <- runWithStore(ctx, dir, filepath.Join(dir, "no-legacy-config"), slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
		}()
		stopped := false
		stop := func() {
			if stopped {
				return
			}
			stopped = true
			cancel()
			if err := <-finished; err != nil {
				t.Errorf("stop Arc: %v", err)
			}
		}
		t.Cleanup(stop)
		for until := time.Now().Add(15 * time.Second); time.Now().Before(until); {
			res, err := client.Get(base + "/api/auth/status")
			if err == nil {
				res.Body.Close()
				if res.StatusCode == want {
					return stop
				}
			}
			select {
			case err := <-finished:
				stopped = true
				t.Fatalf("Arc exited early: %v", err)
			case <-time.After(30 * time.Millisecond):
			}
		}
		t.Fatal("Arc did not start")
		return nil
	}
	request := func(method, path string, body any, origin string, want int) []byte {
		t.Helper()
		var payload []byte
		if body != nil {
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, base+path, strings.NewReader(string(payload)))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != want {
			t.Fatalf("%s %s: status %d, want %d (read error: %v)", method, path, res.StatusCode, want, err)
		}
		return data
	}
	noFile := func(name string) {
		t.Helper()
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s unexpectedly exists or could not be checked", name)
		}
	}

	stop := start(http.StatusOK)
	status := request("GET", "/api/auth/status", nil, "", http.StatusOK)
	if !strings.Contains(string(status), `"database_needed":true`) {
		t.Fatal("first run did not ask for database choice")
	}
	noFile("ams.db")
	noFile(databaseChoiceFile)
	input := databaseInput{Engine: "postgres", Host: conn.Hostname(), Port: port,
		Database: strings.TrimPrefix(conn.Path, "/"), User: conn.User.Username(), Password: "fixture-wrong-password", NoTLS: true}
	request("POST", "/api/auth/database", input, "https://outside.invalid", http.StatusForbidden)
	bad := request("POST", "/api/auth/database", input, base, http.StatusBadRequest)
	if strings.Contains(string(bad), input.Password) {
		t.Fatal("setup returned database credentials in an error")
	}
	noFile(databaseChoiceFile)
	noFile("ams.db")
	input.Password = pass
	request("POST", "/api/auth/database", input, base, http.StatusNoContent)
	if !waitForDatabaseAPI(client, base) {
		t.Fatal("Arc did not hand off to the normal API")
	}
	request("POST", "/api/auth/setup", map[string]string{"name": "PostgreSQL fixture", "password": "arc-fixture-profile"}, base, http.StatusCreated)
	b := request("POST", "/api/backups", nil, base, http.StatusCreated)
	if !strings.Contains(string(b), ".pg.zip") {
		t.Fatal("manual backup did not use the PostgreSQL format")
	}
	choice, err := loadDatabaseChoice(dir)
	if err != nil || choice.Engine != "postgres" || choice.Pending {
		t.Fatal("PostgreSQL choice was not finalized", err)
	}
	viewJSON := request("GET", "/api/settings/database", nil, base, http.StatusOK)
	if strings.Contains(string(viewJSON), pass) || strings.Contains(string(viewJSON), "dsn") ||
		!strings.Contains(string(viewJSON), `"can_edit":true`) {
		t.Fatal("database connection details missing or exposed a credential")
	}

	update := input
	update.Host = "localhost"
	update.Password = "fixture-wrong-updated-password"
	rejected := request("PUT", "/api/settings/database", update, base, http.StatusBadRequest)
	if strings.Contains(string(rejected), update.Password) {
		t.Fatal("connection update returned credentials in an error")
	}
	if unchanged, err := loadDatabaseChoice(dir); err != nil || unchanged != choice {
		t.Fatal("invalid update changed the saved database connection", err)
	}
	update.Password = pass
	request("PUT", "/api/settings/database", update, "https://outside.invalid", http.StatusForbidden)
	request("PUT", "/api/settings/database", update, base, http.StatusNoContent)
	if view := request("GET", "/api/settings/database", nil, base, http.StatusOK); !strings.Contains(string(view), `"restart_required":true`) ||
		!strings.Contains(string(view), `"host":"localhost"`) {
		t.Fatal("updated connection was not staged for restart")
	}
	staged, err := loadDatabaseChoice(dir)
	if err != nil || staged.ID != choice.ID || staged.DSN == choice.DSN || staged.Pending {
		t.Fatal("saved connection changed the installation identity or was not updated", err)
	}
	stop()
	stop = start(http.StatusOK)
	if view := request("GET", "/api/settings/database", nil, base, http.StatusOK); !strings.Contains(string(view), `"restart_required":false`) ||
		!strings.Contains(string(view), `"host":"localhost"`) {
		t.Fatal("restart did not use the saved connection")
	}
	choice = staged
	noFile("ams.db")
	stop()

	// An interruption after the PostgreSQL transaction committed but before the
	// local choice was finalized has the same installation identity on restart.
	choice.Pending = true
	if err := saveDatabaseChoice(dir, choice, false); err != nil {
		t.Fatal(err)
	}
	stop = start(http.StatusOK)
	status = request("GET", "/api/auth/status", nil, "", http.StatusOK)
	if !strings.Contains(string(status), `"setup_needed":false`) {
		t.Fatal("Arc reopened on a new database instead of its saved choice")
	}
	stop()
	choice, err = loadDatabaseChoice(dir)
	if err != nil || choice.Pending {
		t.Fatal("Arc did not complete the interrupted choice", err)
	}
	choice.DSN = "postgres://fixture@127.0.0.1:1/unavailable?sslmode=disable"
	if err := saveDatabaseChoice(dir, choice, false); err != nil {
		t.Fatal(err)
	}
	stop = start(http.StatusServiceUnavailable)
	unavailable := request("GET", "/api/auth/status", nil, "", http.StatusServiceUnavailable)
	if !strings.Contains(string(unavailable), `"code":"database_unavailable"`) {
		t.Fatalf("unexpected startup status: %s", unavailable)
	}
	noFile("ams.db")
	stop()
}

func testDatabaseHTTPAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func testDatabaseClient(t *testing.T, address string) (*http.Client, string) {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar, Timeout: 10 * time.Second}, "http://" + address
}

func waitForDatabaseAPI(client *http.Client, base string) bool {
	for until := time.Now().Add(15 * time.Second); time.Now().Before(until); {
		res, err := client.Get(base + "/api/auth/status")
		if err == nil {
			var status struct {
				DatabaseNeeded bool `json:"database_needed"`
			}
			err := json.NewDecoder(res.Body).Decode(&status)
			res.Body.Close()
			if res.StatusCode == http.StatusOK && err == nil && !status.DatabaseNeeded {
				return true
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	return false
}
