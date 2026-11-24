package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"ams/internal/api"
	"ams/internal/settings"
	"ams/internal/store"
	"ams/internal/webui"
)

// waitForPostgres serves only app assets and a recovery status until the selected
// database is reachable. Database-backed settings are unavailable during this phase.
func waitForPostgres(ctx context.Context, dir string, choice databaseChoice, opts store.Options, log *slog.Logger) (*store.Store, error) {
	address := (settings.Settings{Port: settings.DefaultPort}).ListenAddress()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { api.WriteDatabaseUnavailable(w) })
	mux.Handle("/", webui.Handler())
	server := &http.Server{Handler: api.ProtectDatabaseSetup(mux), ReadHeaderTimeout: 5 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx }}
	server.SetKeepAlivesEnabled(false)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	defer server.Close()
	started := time.Now()
	log.Warn("waiting for PostgreSQL at startup", "address", displayAddress(listener.Addr()))
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-served:
			return nil, err
		case <-ticker.C:
			st, err := openDatabaseChoice(ctx, dir, choice, opts)
			if err == nil {
				log.Info("PostgreSQL available at startup", "waited", time.Since(started).Round(time.Millisecond))
				return st, nil
			}
			if !store.IsPostgresUnavailable(err) {
				return nil, err
			}
		}
	}
}
