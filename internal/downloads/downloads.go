// Package downloads prepares original-file descriptors. It never serves bytes, modifies media,
// starts playback, or claims that a device has retained a playable offline copy.
package downloads

import (
	"ams/internal/store"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"os"
	"time"
)

var ErrCatalogBusy = errors.New("catalog maintenance in progress")

type Manager struct {
	store     *store.Store
	log       *slog.Logger
	wake      chan struct{}
	transfers transferControl
	// Playback is installed before Run starts. Foreground playback pauses hashing.
	Playback func() bool
}

func New(st *store.Store, log *slog.Logger) *Manager {
	return &Manager{store: st, log: log, wake: make(chan struct{}, 1)}
}
func (m *Manager) Wake() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Run has one hashing worker owned by the process root. No per-request goroutine is created.
func (m *Manager) Run(ctx context.Context) {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for ctx.Err() == nil {
		release, admitted := m.store.BeginCatalogWork()
		if !admitted {
			select {
			case <-ctx.Done():
				return
			case <-time.After(250 * time.Millisecond):
			}
			continue
		}
		d, err := m.store.NextDownload(ctx)
		if errors.Is(err, store.ErrDownloadNotFound) {
			release()
			select {
			case <-ctx.Done():
				return
			case <-m.wake:
			case <-timer.C:
			}
			continue
		}
		if err != nil {
			release()
			if ctx.Err() == nil {
				m.log.Warn("download preparation queue unavailable")
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
			continue
		}
		err = m.prepare(ctx, d)
		release()
		if err != nil && ctx.Err() == nil {
			m.log.Warn("download preparation bookkeeping failed", "download_id", d.ID)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}
}

// Prepare is synchronous so callers can join shutdown before closing the store. Interrupted
// hashes remain preparing and restart from the beginning; partial digests are never published.
func (m *Manager) Prepare(ctx context.Context, d *store.Download) error {
	release, ok := m.store.BeginCatalogWork()
	if !ok {
		return ErrCatalogBusy
	}
	defer release()
	return m.prepare(ctx, d)
}

func (m *Manager) prepare(ctx context.Context, d *store.Download) error {
	started := time.Now()
	m.log.Info("download preparation started", "download_id", d.ID)
	reason := ""
	digest := ""
	var ns int64
	err := func() error {
		f, err := m.store.DownloadSource(ctx, d)
		if err != nil {
			if errors.Is(err, store.ErrDownloadNotFound) {
				reason = "source_or_access_changed"
				return nil
			}
			return err
		}
		before, err := os.Stat(f.Path)
		if err != nil || !before.Mode().IsRegular() || before.Size() != d.Size || before.ModTime().Unix() != d.SourceMTime {
			reason = "source_unavailable"
			return nil
		}
		fh, err := os.Open(f.Path)
		if err != nil {
			reason = "source_unavailable"
			return nil
		}
		defer fh.Close()
		opened, err := fh.Stat()
		if err != nil || !same(before, opened) {
			reason = "source_changed"
			return nil
		}
		h := sha256.New()
		buf := make([]byte, 256*1024)
		var size int64
		lastCheck := time.Time{}
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			if time.Since(lastCheck) >= 250*time.Millisecond {
				if _, err := m.store.DownloadSource(ctx, d); err != nil {
					if errors.Is(err, store.ErrDownloadNotFound) {
						reason = "source_or_access_changed"
						return nil
					}
					return err
				}
				lastCheck = time.Now()
			}
			if m.Playback != nil && m.Playback() {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(100 * time.Millisecond):
				}
				continue
			}
			n, readErr := fh.Read(buf)
			if n > 0 {
				_, _ = h.Write(buf[:n])
				size += int64(n)
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				reason = "source_unavailable"
				return nil
			}
		}
		after, err := fh.Stat()
		if err != nil || !same(opened, after) || size != d.Size {
			reason = "source_changed"
			return nil
		}
		pathInfo, err := os.Stat(f.Path)
		if err != nil || !same(opened, pathInfo) {
			reason = "source_changed"
			return nil
		}
		if _, err := m.store.DownloadSource(ctx, d); err != nil {
			if errors.Is(err, store.ErrDownloadNotFound) {
				reason = "source_or_access_changed"
				return nil
			}
			return err
		}
		digest = hex.EncodeToString(h.Sum(nil))
		ns = opened.ModTime().UnixNano()
		return nil
	}()
	if err != nil {
		if ctx.Err() != nil {
			m.log.Debug("download preparation interrupted", "download_id", d.ID)
			return ctx.Err()
		}
		return err
	}
	published, err := m.store.FinishDownload(ctx, d, digest, ns, reason)
	if err != nil {
		return err
	}
	if !published {
		m.log.Debug("download preparation discarded", "download_id", d.ID)
		return nil
	}
	if reason != "" {
		m.log.Warn("download preparation unavailable", "download_id", d.ID, "reason_code", reason, "took", time.Since(started))
	} else {
		m.log.Info("download preparation finished", "download_id", d.ID, "bytes", d.Size, "took", time.Since(started))
	}
	return nil
}
func same(a, b os.FileInfo) bool {
	return a != nil && b != nil && a.Mode().IsRegular() && b.Mode().IsRegular() && os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}
