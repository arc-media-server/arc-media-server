package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"ams/internal/auth"
	"ams/internal/databaseconfig"
	"ams/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestMain(m *testing.M) {
	if os.Getenv("ARC_TEST_AMS_PROCESS") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestDatabaseTransferThroughAPIAndControlledRestart(t *testing.T) {
	dsn := os.Getenv("ARC_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("real PostgreSQL integration fixture not configured")
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	name := "arc_server_" + store.NewID()
	if _, err = raw.ExecContext(t.Context(), `CREATE DATABASE "`+name+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.ExecContext(context.Background(), `DROP DATABASE "`+name+`" WITH (FORCE)`); raw.Close() })
	root := t.TempDir()
	st, err := store.Open(t.Context(), filepath.Join(root, "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := auth.Hash("fixture-password-not-real")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := st.CreateUser(t.Context(), "Owner", true, "violet", hash, "")
	if err != nil {
		t.Fatal(err)
	}
	token := "fixture-bearer-not-real"
	if err = st.CreateSession(t.Context(), auth.HashToken(token), admin.ID, "password", "fixture"); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"scan_on_start": "false", "scan_automatic": "false", "upnp": "false"} {
		if err = st.SetSetting(t.Context(), key, value); err != nil {
			t.Fatal(err)
		}
	}
	serverID, err := st.Setting(t.Context(), "server_id")
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(executable, "-data", root, "-config", filepath.Join(root, "no-config"))
	command.Env = append(os.Environ(), "ARC_TEST_AMS_PROCESS=1", "AMS_LISTEN="+address)
	logPath := filepath.Join(root, "process.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	command.Stdout = logFile
	command.Stderr = logFile
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		command.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			command.Process.Kill()
			<-done
		}
		logFile.Close()
	})
	client := &http.Client{Timeout: 10 * time.Second}
	request := func(method, path string, body []byte) (int, []byte, error) {
		req, e := http.NewRequest(method, "http://"+address+path, bytes.NewReader(body))
		if e != nil {
			return 0, nil, e
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, e := client.Do(req)
		if e != nil {
			return 0, nil, e
		}
		defer resp.Body.Close()
		b, e := io.ReadAll(resp.Body)
		return resp.StatusCode, b, e
	}
	waitFor := func(want string) {
		t.Helper()
		deadline := time.Now().Add(40 * time.Second)
		for time.Now().Before(deadline) {
			code, b, e := request("GET", "/api/v1/database", nil)
			if e == nil && code == 200 && strings.Contains(string(b), `"backend":"`+want+`"`) {
				return
			}
			select {
			case e := <-done:
				done <- e
				logFile.Sync()
				logs, _ := os.ReadFile(logPath)
				t.Fatalf("server exited: %v\n%s", e, logs)
			default:
			}
			time.Sleep(30 * time.Millisecond)
		}
		logFile.Sync()
		logs, _ := os.ReadFile(logPath)
		t.Fatalf("server never selected %s\n%s", want, logs)
	}
	waitFor("sqlite")
	port, _ := strconv.Atoi(parsed.Port())
	password, _ := parsed.User.Password()
	body, _ := json.Marshal(map[string]any{"connection": databaseconfig.Connection{Host: parsed.Hostname(), Port: port, Database: name, Username: parsed.User.Username(), Password: password, TLSMode: "disable"}, "confirm_downtime": true})
	code, response, err := request("POST", "/api/v1/database/transfer", body)
	if err != nil || code != 202 {
		t.Fatalf("transfer not accepted: %d %s %v", code, response, err)
	}
	waitFor("postgres")
	code, response, err = request("GET", "/api/v1/auth/status", nil)
	if err != nil || code != 200 || !strings.Contains(string(response), `"signed_in":true`) {
		t.Fatalf("session lost after restart: %d %s %v", code, response, err)
	}
	selected := databaseconfig.New(root, nil)
	pg, err := selected.Open(t.Context(), store.Options{})
	if err == nil {
		pg.Close()
		t.Fatal("second Arc store admitted while server owns PostgreSQL")
	}
	targetURL := *parsed
	targetURL.Path = "/" + name
	read, err := sql.Open("pgx", targetURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	var after string
	if err = read.QueryRowContext(t.Context(), "SELECT value FROM settings WHERE key='server_id'").Scan(&after); err != nil || after != serverID {
		t.Fatal("server identity changed", err)
	}
	// Reauthenticate using the preserved password rather than trusting only the old bearer.
	login, _ := json.Marshal(map[string]string{"password": "fixture-password-not-real"})
	code, response, err = request("POST", "/api/v1/auth/login", login)
	if err != nil || code != http.StatusNoContent {
		t.Fatalf("preserved password failed: %d %s %v", code, response, err)
	}
	logFile.Sync()
	logs, _ := os.ReadFile(logPath)
	for _, secret := range []string{password, token, "fixture-password-not-real", dsn} {
		if strings.Contains(string(logs), secret) {
			t.Fatal("server logs exposed a connection/auth secret")
		}
	}
}
