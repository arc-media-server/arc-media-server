package main

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/api"
	"ams/internal/store"
)

type databaseEditor struct {
	dir, activeDSN string
	store          *store.Store
	log            *slog.Logger
	mu             sync.Mutex
	saved          databaseChoice
}

func (d *databaseEditor) Saved() api.DatabaseSettingsView {
	d.mu.Lock()
	defer d.mu.Unlock()
	return api.DatabaseSettingsView{
		Engine: "postgres", Connection: editablePostgresConnection(d.saved.DSN),
		RestartRequired: d.saved.DSN != d.activeDSN,
	}
}

// Only edit the single-host URL produced by onboarding or documented for recovery.
// More advanced DSNs can still be managed offline without dropping driver options.
func editablePostgresConnection(dsn string) *api.PostgresConnection {
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") ||
		u.Hostname() == "" || u.User == nil || u.User.Username() == "" || u.Fragment != "" {
		return nil
	}
	query, err := url.ParseQuery(u.RawQuery)
	mode := query.Get("sslmode")
	if err != nil || len(query) != 1 || len(query["sslmode"]) != 1 ||
		(mode != "verify-full" && mode != "disable") {
		return nil
	}
	port := 5432
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
	}
	database := strings.TrimPrefix(u.Path, "/")
	if err != nil || port < 1 || port > 65535 || database == "" {
		return nil
	}
	return &api.PostgresConnection{Host: u.Hostname(), Port: port, Database: database, User: u.User.Username(), NoTLS: mode == "disable"}
}

func (d *databaseEditor) Update(ctx context.Context, update api.PostgresConnectionUpdate) error {
	if !d.mu.TryLock() {
		return errors.New("Arc is already checking the PostgreSQL connection")
	}
	defer d.mu.Unlock()
	if editablePostgresConnection(d.saved.DSN) == nil {
		return errors.New("edit this connection in Arc's data folder while Arc is stopped")
	}
	input := databaseInput{Engine: "postgres", Host: update.Host, Port: update.Port,
		Database: update.Database, User: update.User, Password: update.Password, NoTLS: update.NoTLS}
	dsn, err := input.postgresDSN()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if err := d.store.VerifyPostgresConnection(ctx, dsn, d.saved.ID); err != nil {
		return err
	}
	if d.saved.DSN == dsn {
		return nil
	}
	next := d.saved
	next.DSN = dsn
	if err := saveDatabaseChoice(d.dir, next, false); err != nil {
		// The file may have been replaced even if syncing its parent failed.
		if saved, readErr := loadDatabaseChoice(d.dir); readErr == nil {
			d.saved = saved
		}
		d.log.Warn("could not persist PostgreSQL connection settings")
		return errors.New("could not save the PostgreSQL connection in Arc's data folder")
	}
	d.saved = next
	d.log.Info("PostgreSQL connection settings saved", "restart_required", d.activeDSN != dsn)
	return nil
}
