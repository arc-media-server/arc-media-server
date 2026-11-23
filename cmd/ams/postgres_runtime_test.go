package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ams/internal/store"
)

func TestPostgresScratchServer(t *testing.T) {
	dsn := os.Getenv("ARC_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("set ARC_TEST_POSTGRES to a dedicated empty PostgreSQL fixture database")
	}
	// Scope the test to its generated Arc data and loopback HTTP listener.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "AMS_") {
			t.Setenv(key, "")
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	t.Setenv("AMS_LISTEN", address)

	dataDir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ids := make(chan string, 1)
	runErr := make(chan error, 1)
	go func() {
		runErr <- runWithStore(ctx, dataDir, filepath.Join(dataDir, "no-legacy-config"),
			slog.New(slog.NewTextHandler(os.Stderr, nil)), func(ctx context.Context, _ string, _ store.Options) (*store.Store, error) {
				st, err := store.OpenPostgres(ctx, dsn)
				if err != nil {
					return nil, err
				}
				count, err := st.CountUsers(ctx)
				if err != nil || count != 0 {
					st.Close()
					return nil, errors.New("fixture requires a PostgreSQL database without profiles")
				}
				for _, name := range []string{"scan_on_start", "scan_automatic"} {
					if err := st.SetSetting(ctx, name, "0"); err != nil {
						st.Close()
						return nil, err
					}
				}
				library, err := st.CreateLibrary(ctx, "Scratch films", "movies", "", nil)
				var itemID string
				if err == nil {
					itemID, _, err = st.EnsureItem(ctx, &store.Item{Title: "Scratch Film", Kind: "movie", LibraryID: library}, "scratch")
				}
				if err == nil {
					var fileID int64
					fileID, err = st.SaveFile(ctx, &store.MediaFile{LibraryID: library, Path: filepath.Join(dataDir, "fixture.mkv")}, 1, 1)
					if err == nil {
						err = st.SetFileItems(ctx, fileID, []string{itemID}, 0)
					}
				}
				if err != nil {
					st.Close()
					return nil, fmt.Errorf("seed runtime fixture: %w", err)
				}
				ids <- itemID
				return st, nil
			})
	}()
	finished := false
	defer func() {
		if !finished {
			cancel()
			<-runErr
		}
	}()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 3 * time.Second}
	base := "http://" + address
	deadline := time.Now().Add(15 * time.Second)
	for {
		resp, err := client.Get(base + "/api/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		select {
		case err := <-runErr:
			finished = true
			t.Fatalf("scratch Arc server exited: %v", err)
		case <-time.After(30 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("scratch Arc server did not start")
		}
	}

	request := func(method, path, body string, want int) []byte {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		if err != nil || res.StatusCode != want {
			t.Fatalf("%s %s: HTTP %d, want %d (read error: %v)", method, path, res.StatusCode, want, err)
		}
		return data
	}

	itemID := <-ids
	publicData := request("GET", "/api/auth/status", "", http.StatusOK)
	if !strings.Contains(string(publicData), `"setup_needed":true`) {
		t.Fatal("fresh PostgreSQL database did not start at setup")
	}
	profile := request("POST", "/api/auth/setup", `{"name":"Scratch owner","password":"fixture-password-1234"}`, http.StatusCreated)
	var owner struct{ ID string }
	if err := json.Unmarshal(profile, &owner); err != nil || owner.ID == "" {
		t.Fatal("setup did not return a profile")
	}
	if data := request("GET", "/api/libraries", "", http.StatusOK); !strings.Contains(string(data), "Scratch films") {
		t.Fatal("library missing from PostgreSQL-backed Arc")
	}
	if data := request("GET", "/api/items?kind=movie", "", http.StatusOK); !strings.Contains(string(data), "Scratch Film") {
		t.Fatal("catalog item missing from PostgreSQL-backed Arc")
	}
	if data := request("GET", "/api/items?kind=movie&q=scratch", "", http.StatusOK); !strings.Contains(string(data), "Scratch Film") {
		t.Fatal("search did not find the scratch item")
	}
	for range 2 {
		request("PUT", "/api/items/"+itemID+"/watchlist", `{"on":true}`, http.StatusNoContent)
	}
	if data := request("GET", "/api/watchlist", "", http.StatusOK); !strings.Contains(string(data), "Scratch Film") {
		t.Fatal("watchlist did not include the saved item")
	}
	if another, err := store.OpenPostgres(ctx, dsn); err == nil {
		another.Close()
		t.Fatal("another Arc writer opened the PostgreSQL database while the server was running")
	}

	cancel()
	err = <-runErr
	finished = true
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "ams.db")); !os.IsNotExist(err) {
		t.Fatalf("PostgreSQL startup left a SQLite database: %v", err)
	}
	reopened, err := store.OpenPostgres(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.GetUser(t.Context(), owner.ID)
	if err != nil || got == nil || !got.IsAdmin {
		t.Fatalf("profile did not survive reopen: %v", err)
	}
	id, err := reopened.AddPlay(t.Context(), store.Play{UserID: owner.ID, ItemID: itemID, Kind: "movie", Title: "Scratch Film"})
	if err != nil || id <= 0 {
		t.Fatalf("record playback history: id %d, err %v", id, err)
	}
}
