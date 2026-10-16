// Package backup takes automatic daily backups of the AMS database, keeps a sensible number of
// them, and restores one on request. Backups hold everything AMS knows (matches, orderings,
// watch history); media files are never part of them and are never touched.
package backup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"ams/internal/store"
)

// Why a backup was taken. Automatic ones follow the retention rules; manual ones are kept until
// deleted; safety copies (before a restore or an upgrade) keep the newest few.
const (
	ReasonAuto          = "auto"
	ReasonManual        = "manual"
	ReasonBeforeRestore = "before-restore"
	ReasonBeforeUpgrade = "before-upgrade"
)

const (
	interval    = 24 * time.Hour
	keepDaily   = 7 // newest automatic backups always kept
	keepWeekly  = 4 // then one per week for this many weeks
	keepSafety  = 3 // per safety reason
	stampLayout = "20060102-150405"
)

var nameRe = regexp.MustCompile(`^ams-(\d{8}-\d{6})-(auto|manual|before-restore|before-upgrade)\.db$`)

type Backup struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Reason    string    `json:"reason"`
	Size      int64     `json:"size"`
}

type Manager struct {
	dir   string
	store *store.Store
	log   *slog.Logger

	mu sync.Mutex // one backup or restore at a time
}

func New(dir string, st *store.Store, log *slog.Logger) *Manager {
	return &Manager{dir: dir, store: st, log: log}
}

// Dir is where backups are kept.
func (m *Manager) Dir() string { return m.dir }

// List returns the backups, newest first.
func (m *Manager) List() ([]Backup, error) {
	entries, err := os.ReadDir(m.dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Backup{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Backup{}
	for _, e := range entries {
		b, ok := parseName(e.Name())
		if !ok || e.IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil {
			b.Size = info.Size()
		}
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func parseName(name string) (Backup, bool) {
	g := nameRe.FindStringSubmatch(name)
	if g == nil {
		return Backup{}, false
	}
	t, err := time.ParseInLocation(stampLayout, g[1], time.Local)
	if err != nil {
		return Backup{}, false
	}
	return Backup{Name: name, CreatedAt: t, Reason: g[2]}, true
}

// Create takes a backup now and applies the retention rules.
func (m *Manager) Create(ctx context.Context, reason string) (Backup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.create(ctx, reason)
}

func (m *Manager) create(ctx context.Context, reason string) (Backup, error) {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return Backup{}, err
	}
	now := time.Now()
	name := fmt.Sprintf("ams-%s-%s.db", now.Format(stampLayout), reason)
	// Two backups in the same second (e.g. a manual one right after a restore): wait it out.
	for _, err := os.Stat(filepath.Join(m.dir, name)); err == nil; _, err = os.Stat(filepath.Join(m.dir, name)) {
		time.Sleep(time.Second)
		now = time.Now()
		name = fmt.Sprintf("ams-%s-%s.db", now.Format(stampLayout), reason)
	}
	path := filepath.Join(m.dir, name)
	tmp := path + ".tmp"
	os.Remove(tmp)
	if err := m.store.BackupTo(ctx, tmp); err != nil {
		os.Remove(tmp)
		return Backup{}, fmt.Errorf("backup: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return Backup{}, err
	}
	b, _ := parseName(name)
	if info, err := os.Stat(path); err == nil {
		b.Size = info.Size()
	}
	m.log.Info("database backed up", "file", path, "reason", reason)
	if err := m.prune(); err != nil {
		m.log.Warn("removing old backups failed", "err", err)
	}
	return b, nil
}

// prune deletes automatic backups beyond the retention rules, and old safety copies. Manual
// backups are never deleted automatically.
func (m *Manager) prune() error {
	all, err := m.List()
	if err != nil {
		return err
	}
	for _, b := range expired(all) {
		if err := os.Remove(filepath.Join(m.dir, b.Name)); err != nil {
			return err
		}
		m.log.Info("removed old backup", "file", b.Name)
	}
	return nil
}

// expired picks the backups (sorted newest first) the retention rules no longer need: automatic
// ones beyond the newest keepDaily, except the newest in each of the keepWeekly weeks before them.
func expired(all []Backup) []Backup {
	var out []Backup
	auto, safety := 0, map[string]int{}
	weeks := map[string]bool{}
	for _, b := range all {
		switch b.Reason {
		case ReasonAuto:
			auto++
			if auto <= keepDaily {
				continue
			}
			y, w := b.CreatedAt.ISOWeek()
			week := fmt.Sprintf("%d-%d", y, w)
			if !weeks[week] && len(weeks) < keepWeekly {
				weeks[week] = true
				continue
			}
			out = append(out, b)
		case ReasonBeforeRestore, ReasonBeforeUpgrade:
			safety[b.Reason]++
			if safety[b.Reason] > keepSafety {
				out = append(out, b)
			}
		}
	}
	return out
}

// Delete removes one backup.
func (m *Manager) Delete(name string) error {
	if _, ok := parseName(name); !ok {
		return fmt.Errorf("no backup named %q", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return os.Remove(filepath.Join(m.dir, name))
}

// Restore replaces the database with a backup, after saving the current state as a
// before-restore backup so the restore can itself be undone.
func (m *Manager) Restore(ctx context.Context, name string) error {
	if _, ok := parseName(name); !ok {
		return fmt.Errorf("no backup named %q", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	path := filepath.Join(m.dir, name)
	if err := store.CheckBackup(ctx, path); err != nil {
		return err
	}
	if _, err := m.create(ctx, ReasonBeforeRestore); err != nil {
		return fmt.Errorf("couldn't save the current state first, so nothing was restored: %w", err)
	}
	if err := m.store.RestoreFrom(ctx, path); err != nil {
		return err
	}
	m.log.Info("database restored", "from", name)
	return nil
}

// Run takes an automatic backup whenever the last one is a day old, checking hourly, until ctx
// is cancelled. The first check happens shortly after startup.
func (m *Manager) Run(ctx context.Context) {
	var lastAttempt time.Time
	check := func() {
		if time.Since(lastAttempt) < interval {
			return
		}
		latest, err := m.latestAuto()
		if err != nil {
			m.log.Warn("listing backups failed", "err", err)
			return
		}
		if time.Since(latest) < interval {
			lastAttempt = latest
			return
		}
		lastAttempt = time.Now()
		if _, err := m.Create(ctx, ReasonAuto); err != nil && ctx.Err() == nil {
			m.log.Warn("automatic backup failed", "err", err)
		}
	}
	timer := time.NewTimer(time.Minute) // let the startup scan get going first
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			check()
			timer.Reset(time.Hour)
		}
	}
}

func (m *Manager) latestAuto() (time.Time, error) {
	all, err := m.List()
	if err != nil {
		return time.Time{}, err
	}
	for _, b := range all {
		if b.Reason == ReasonAuto {
			return b.CreatedAt, nil
		}
	}
	return time.Time{}, nil
}
