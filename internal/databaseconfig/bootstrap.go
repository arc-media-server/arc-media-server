package databaseconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ams/internal/store"
)

const bootstrapName = "database-bootstrap.json"

type journal struct {
	ID       string     `json:"id"`
	Phase    string     `json:"phase"`
	Snapshot string     `json:"snapshot"`
	Target   Connection `json:"target"`
}
type bootstrap struct {
	Version    int                   `json:"version"`
	Backend    string                `json:"backend"`
	Postgres   *Connection           `json:"postgres,omitempty"`
	Pending    *journal              `json:"pending,omitempty"`
	LastReport *store.TransferReport `json:"last_report,omitempty"`
}
type Status struct {
	Backend           string                `json:"backend"`
	TransferAvailable bool                  `json:"transfer_available"`
	State             string                `json:"state"`
	LastReport        *store.TransferReport `json:"last_report,omitempty"`
	BackupFormat      string                `json:"backup_format"`
}
type Manager struct {
	dir       string
	log       *slog.Logger
	mu        sync.Mutex
	changing  atomic.Bool
	requested chan struct{}
}

func New(dir string, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Manager{dir: dir, log: log, requested: make(chan struct{}, 1)}
}
func (m *Manager) Requested() <-chan struct{} { return m.requested }
func (m *Manager) Transitioning() bool        { return m.changing.Load() }
func (m *Manager) load() (bootstrap, error) {
	b := bootstrap{Version: 1, Backend: "sqlite"}
	path := filepath.Join(m.dir, bootstrapName)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if _, e := os.Stat(filepath.Join(m.dir, ".postgres-required")); e == nil {
			return b, errors.New("PostgreSQL selector is missing; restore the protected bootstrap instead of opening stale SQLite")
		} else if !errors.Is(e, os.ErrNotExist) {
			return b, e
		}
		return b, nil
	}
	if err != nil {
		return b, err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return b, errors.New("database bootstrap must be an owner-only regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return b, err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, 65537))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&b); err != nil {
		return b, errors.New("database bootstrap is invalid")
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return b, errors.New("database bootstrap has trailing content")
	}
	if b.Version != 1 || (b.Backend != "sqlite" && b.Backend != "postgres") || b.Backend == "postgres" && b.Postgres == nil {
		return b, errors.New("database bootstrap version or backend is unsupported")
	}
	return b, nil
}
func (m *Manager) save(b bootstrap) error {
	if err := os.MkdirAll(m.dir, 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(m.dir, ".arc-database-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(m.dir, bootstrapName)); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		d, e := os.Open(m.dir)
		if e != nil {
			return e
		}
		defer d.Close()
		return d.Sync()
	}
	return nil
}
func (m *Manager) Status() (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := m.load()
	if err != nil {
		return Status{}, err
	}
	state := "ready"
	if b.Pending != nil {
		state = b.Pending.Phase
	}
	return Status{Backend: b.Backend, TransferAvailable: b.Backend == "sqlite" && b.Pending == nil, State: state, LastReport: b.LastReport, BackupFormat: "arc-sqlite"}, nil
}
func (m *Manager) Open(ctx context.Context, opts store.Options) (*store.Store, error) {
	b, err := m.load()
	if err != nil {
		return nil, err
	}
	if b.Pending != nil {
		return nil, errors.New("database transfer recovery must complete before serving requests")
	}
	if b.Backend == "postgres" {
		dsn, e := b.Postgres.DSN()
		if e != nil {
			return nil, e
		}
		return store.OpenPostgres(ctx, dsn, opts)
	}
	return store.Open(ctx, filepath.Join(m.dir, "ams.db"), opts)
}
func (m *Manager) Test(ctx context.Context, c Connection) (store.PostgresPreflight, error) {
	dsn, err := c.DSN()
	if err != nil {
		return store.PostgresPreflight{}, err
	}
	return store.InspectPostgres(ctx, dsn)
}
func (m *Manager) Request(ctx context.Context, c Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := m.load()
	if err != nil {
		return err
	}
	if b.Backend != "sqlite" || b.Pending != nil || m.changing.Load() {
		return errors.New("database transfer is unavailable or already in progress")
	}
	check, err := m.Test(ctx, c)
	if err != nil {
		return err
	}
	if !check.Empty || !check.CanCreate {
		return errors.New("PostgreSQL target must be empty and the account must be able to create its schema")
	}
	b.Pending = &journal{ID: store.NewID(), Phase: "requested", Target: c}
	if err = m.save(b); err != nil {
		return err
	}
	m.changing.Store(true)
	m.requested <- struct{}{}
	m.log.Info("database transfer requested", "from", "sqlite", "to", "postgres")
	return nil
}

// Recover runs only after the previous server/store and workers have stopped, and before the
// next store opens. Until the atomic selector switch, SQLite remains authoritative.
func (m *Manager) Recover(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := m.load()
	if err != nil {
		return err
	}
	if b.Pending == nil {
		return nil
	}
	j := b.Pending
	if b.Backend != "sqlite" || j.ID == "" {
		return errors.New("invalid database transfer journal")
	}
	started := time.Now()
	m.log.Info("database transfer started", "phase", j.Phase)
	defer func() {
		if err != nil {
			m.log.Warn("database transfer incomplete; SQLite selector retained", "phase", j.Phase, "took", time.Since(started).Round(time.Millisecond), "err", err)
		}
	}()
	dsn, err := j.Target.DSN()
	if err != nil {
		return err
	}
	if j.Phase == "requested" {
		if err = os.MkdirAll(filepath.Join(m.dir, "backups"), 0700); err != nil {
			return err
		}
		stamp := time.Now()
		for {
			j.Snapshot = fmt.Sprintf("ams-%s-before-upgrade.db", stamp.Format("20060102-150405"))
			if _, e := os.Stat(filepath.Join(m.dir, "backups", j.Snapshot)); errors.Is(e, os.ErrNotExist) {
				break
			} else if e != nil {
				return e
			}
			stamp = stamp.Add(time.Second)
		}
		// Persist the reserved name before snapshotting. A restart never guesses a different source.
		j.Phase = "snapshotting"
		if err = m.save(b); err != nil {
			return err
		}
	}
	if filepath.Base(j.Snapshot) != j.Snapshot || !strings.HasPrefix(j.Snapshot, "ams-") {
		return errors.New("invalid transfer safety-backup path")
	}
	snapshot := filepath.Join(m.dir, "backups", j.Snapshot)
	if j.Phase == "snapshotting" {
		// No server may start while this journal is pending. A failed partial snapshot is not reused.
		if _, e := os.Stat(snapshot); e == nil {
			if err = store.CheckBackup(ctx, snapshot); err != nil {
				return errors.New("incomplete safety snapshot; cancel the pending transfer before retrying")
			}
		} else if errors.Is(e, os.ErrNotExist) {
			if err = store.SnapshotSQLite(ctx, filepath.Join(m.dir, "ams.db"), snapshot); err != nil {
				return err
			}
		} else {
			return e
		}
		m.log.Info("database transfer snapshot ready")
		j.Phase = "copying"
		if err = m.save(b); err != nil {
			return err
		}
	}
	if j.Phase != "copying" && j.Phase != "verified" {
		return errors.New("unknown database transfer journal phase")
	}
	report, complete, e := store.CompletedTransfer(ctx, snapshot, dsn, j.ID)
	if e != nil {
		err = e
		return err
	}
	if !complete {
		m.log.Info("database transfer copying and verifying")
		if report, err = store.TransferSQLiteWithID(ctx, snapshot, dsn, j.ID); err != nil {
			return err
		}
	}
	m.log.Info("database transfer verified", "tables", len(report.Tables), "rows", report.Rows)
	j.Phase = "verified"
	b.LastReport = &report
	if err = m.save(b); err != nil {
		return err
	}
	guard, e := os.OpenFile(filepath.Join(m.dir, ".postgres-required"), os.O_CREATE|os.O_WRONLY, 0600)
	if e != nil {
		err = e
		return err
	}
	if err = guard.Sync(); err != nil {
		guard.Close()
		return err
	}
	guard.Close()
	b.Backend = "postgres"
	b.Postgres = &j.Target
	b.Pending = nil
	if err = m.save(b); err != nil {
		return err
	}
	m.log.Info("database transfer finished", "tables", len(report.Tables), "rows", report.Rows, "took", time.Since(started).Round(time.Millisecond))
	return nil
}

// Cancel is an explicit pre-cutover escape hatch. It never changes an active PostgreSQL selector.
func (m *Manager) Cancel() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := m.load()
	if err != nil {
		return err
	}
	if b.Backend != "sqlite" || b.Pending == nil {
		return errors.New("no pre-cutover transfer can be cancelled")
	}
	b.Pending = nil
	return m.save(b)
}
