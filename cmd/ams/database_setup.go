package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/api"
	"ams/internal/auth"
	"ams/internal/settings"
	"ams/internal/store"
	"ams/internal/webui"
)

const databaseChoiceFile = "database.json"

type databaseChoice struct {
	Engine  string `json:"engine"`
	DSN     string `json:"dsn,omitempty"`
	ID      string `json:"installation_id,omitempty"`
	Pending bool   `json:"pending,omitempty"`
}

func loadDatabaseChoice(dir string) (databaseChoice, error) {
	var choice databaseChoice
	data, err := os.ReadFile(filepath.Join(dir, databaseChoiceFile))
	if errors.Is(err, os.ErrNotExist) {
		return choice, nil
	}
	if err != nil {
		return choice, err
	}
	if json.Unmarshal(data, &choice) != nil ||
		!(choice.Engine == "sqlite" && choice.DSN == "" && choice.ID == "" && !choice.Pending ||
			choice.Engine == "postgres" && choice.DSN != "" && choice.ID != "") {
		return databaseChoice{}, errors.New("invalid saved database choice")
	}
	return choice, nil
}

func saveDatabaseChoice(dir string, choice databaseChoice, initial bool) error {
	temp, err := os.CreateTemp(dir, ".database-choice-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := json.NewEncoder(temp).Encode(choice); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	path := filepath.Join(dir, databaseChoiceFile)
	if initial {
		// Publish one complete choice without replacing a choice another process made.
		err = os.Link(temp.Name(), path)
	} else {
		err = os.Rename(temp.Name(), path)
	}
	if err != nil || runtime.GOOS == "windows" {
		return err
	}
	parent, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}

func openDatabaseChoice(ctx context.Context, dir string, choice databaseChoice, opts store.Options) (*store.Store, error) {
	if choice.Engine != "postgres" {
		return store.Open(ctx, filepath.Join(dir, "ams.db"), opts)
	}
	st, err := store.OpenSelectedPostgres(ctx, choice.DSN, filepath.Join(dir, "backups"), choice.ID, choice.Pending, nil)
	if err != nil {
		return nil, fmt.Errorf("open selected PostgreSQL database: %w", err)
	}
	if err := completeDatabaseChoice(dir, choice); err != nil {
		st.Close()
		return nil, err
	}
	return st, nil
}

func completeDatabaseChoice(dir string, choice databaseChoice) error {
	if !choice.Pending {
		return nil
	}
	choice.Pending = false
	return saveDatabaseChoice(dir, choice, false)
}

type databaseInput struct {
	Engine   string `json:"engine"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	Password string `json:"password"`
	NoTLS    bool   `json:"no_tls"`
}

var databaseHostname = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func (input databaseInput) choice() (databaseChoice, error) {
	if input.Engine == "sqlite" {
		return databaseChoice{Engine: input.Engine}, nil
	}
	if input.Engine != "postgres" || !(net.ParseIP(input.Host) != nil || databaseHostname.MatchString(input.Host)) ||
		input.Port < 1 || input.Port > 65535 || input.Database == "" || input.User == "" || input.Password == "" {
		return databaseChoice{}, errors.New("enter the PostgreSQL host, port, database, username and password")
	}
	mode := "verify-full"
	if input.NoTLS {
		mode = "disable"
	}
	address := url.URL{Scheme: "postgres", User: url.UserPassword(input.User, input.Password),
		Host: net.JoinHostPort(input.Host, strconv.Itoa(input.Port)), Path: "/" + input.Database,
		RawQuery: url.Values{"sslmode": {mode}}.Encode()}
	return databaseChoice{Engine: input.Engine, DSN: address.String(), ID: store.NewID(), Pending: true}, nil
}

func chooseDatabase(ctx context.Context, dir string, input databaseInput, opts store.Options) (*store.Store, error) {
	choice, err := input.choice()
	if err != nil {
		return nil, err
	}
	save := func() error { return saveDatabaseChoice(dir, choice, true) }
	if choice.Engine == "sqlite" {
		if err := save(); err != nil {
			return nil, err
		}
		return openDatabaseChoice(ctx, dir, choice, opts)
	}
	st, err := store.OpenSelectedPostgres(ctx, choice.DSN, filepath.Join(dir, "backups"), choice.ID, true, save)
	if err != nil {
		return nil, err
	}
	if err := completeDatabaseChoice(dir, choice); err != nil {
		st.Close()
		return nil, err
	}
	return st, nil
}

type bootstrapResult struct {
	store *store.Store
	err   error
}

// waitForDatabase serves the first screen before creating a database. Older setup clients can
// still submit the original account request, which chooses the default and then enters the API.
func waitForDatabase(ctx context.Context, dir string, opts store.Options, log *slog.Logger) (*store.Store, func(http.Handler), func(), error) {
	initialSettings := settings.Settings{Port: settings.DefaultPort}
	listener, err := net.Listen("tcp", initialSettings.ListenAddress())
	if err != nil {
		return nil, nil, nil, err
	}
	result := make(chan bootstrapResult)
	ready := make(chan struct{})
	var next http.Handler
	publish := func(h http.Handler) {
		next = h
		close(ready)
	}
	var choiceMu sync.Mutex
	committed := false
	pick := func(w http.ResponseWriter, r *http.Request, input databaseInput, explicit bool) bool {
		if !choiceMu.TryLock() {
			databaseResponse(w, http.StatusConflict, map[string]string{"error": "Arc is already opening a database"})
			return false
		}
		defer choiceMu.Unlock()
		if committed {
			if explicit {
				databaseResponse(w, http.StatusConflict, map[string]string{"error": "Arc has already chosen a database"})
				return false
			}
			return true
		}
		pickCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		st, err := chooseDatabase(pickCtx, dir, input, opts)
		if err != nil {
			log.Warn("initial database selection failed", "engine", input.Engine, "err", err)
			databaseResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			if _, statErr := os.Lstat(filepath.Join(dir, databaseChoiceFile)); statErr == nil {
				committed = true
				select {
				case result <- bootstrapResult{err: err}:
				case <-ctx.Done():
				}
			}
			return false
		}
		committed = true
		log.Info("initial database selected", "engine", input.Engine)
		if explicit {
			w.WriteHeader(http.StatusNoContent)
		}
		select {
		case result <- bootstrapResult{store: st}:
		case <-ctx.Done():
			st.Close()
			return false
		}
		return true
	}
	mux := http.NewServeMux()
	mux.Handle("/", webui.Handler())
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	statusHandler := func(w http.ResponseWriter, r *http.Request) {
		databaseResponse(w, http.StatusOK, map[string]any{
			"database_needed": true, "setup_needed": true, "signed_in": false, "profile": nil,
			"profiles": []any{}, "local": auth.IsLocal(r), "bypass": false, "friend": false, "password_session": false,
		})
	}
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		databaseResponse(w, http.StatusOK, map[string]any{"name": "Arc Media Server", "version": version, "api_version": api.APIVersion, "status": "ok"})
	}
	for _, prefix := range []string{"/api", "/api/v1"} {
		mux.HandleFunc("GET "+prefix+"/health", healthHandler)
		mux.HandleFunc("GET "+prefix+"/openapi.json", api.ServeOpenAPIDocument)
		mux.HandleFunc("GET "+prefix+"/auth/status", statusHandler)
		mux.HandleFunc("POST "+prefix+"/auth/setup", func(w http.ResponseWriter, r *http.Request) {
			if !api.DatabaseSetupAllowed(r) {
				databaseResponse(w, http.StatusForbidden, map[string]string{"error": "open Arc directly to set it up"})
				return
			}
			if !validateLegacySetup(w, r) || !pick(w, r, databaseInput{Engine: "sqlite"}, false) {
				return
			}
			select {
			case <-ready:
				next.ServeHTTP(w, r)
			case <-r.Context().Done():
			}
		})
	}
	mux.HandleFunc("POST /api/auth/database", func(w http.ResponseWriter, r *http.Request) {
		if !api.DatabaseSetupAllowed(r) {
			databaseResponse(w, http.StatusForbidden, map[string]string{"error": "choose the database from Arc on your home network"})
			return
		}
		var input databaseInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input); err != nil {
			databaseResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid database setup request"})
			return
		}
		if input.Engine == "postgres" && !auth.IsDatabaseSetupLocal(r) {
			databaseResponse(w, http.StatusForbidden, map[string]string{"error": "choose PostgreSQL from Arc on your home network"})
			return
		}
		if _, err := input.choice(); err != nil {
			databaseResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		pick(w, r, input, true)
	})

	server := &http.Server{Handler: api.ProtectDatabaseSetup(mux), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx }}
	server.SetKeepAlivesEnabled(false)
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	log.Info("waiting for initial database selection", "address", displayAddress(listener.Addr()))
	closeServer := func() { _ = server.Close() }
	select {
	case selected := <-result:
		// Stop accepting setup calls. A request in flight can finish through the normal API.
		_ = listener.Close()
		if selected.err != nil {
			closeServer()
			return nil, nil, nil, selected.err
		}
		return selected.store, publish, closeServer, nil
	case err := <-finished:
		closeServer()
		return nil, nil, nil, err
	case <-ctx.Done():
		closeServer()
		return nil, nil, nil, ctx.Err()
	}
}

// Only a valid account request may implicitly choose SQLite for an older setup client.
func validateLegacySetup(w http.ResponseWriter, r *http.Request) bool {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<10))
	var request struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err != nil || json.Unmarshal(data, &request) != nil {
		databaseResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return false
	}
	if strings.TrimSpace(request.Name) == "" {
		databaseResponse(w, http.StatusBadRequest, map[string]string{"error": "enter your name"})
		return false
	}
	if len(request.Password) < auth.MinPasswordLength {
		databaseResponse(w, http.StatusBadRequest, map[string]string{"error": "the password needs at least 8 characters"})
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return true
}

func databaseResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
