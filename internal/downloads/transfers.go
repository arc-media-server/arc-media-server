package downloads

import (
	"sync"
	"time"
)

type transferControl struct {
	mu       sync.Mutex
	owners   map[string]bool
	outcomes map[string]time.Time
}

// BeginTransfer bounds live requests independently of playback slots. The request owns release.
func (m *Manager) BeginTransfer(user, session string) (func(), bool) {
	c := &m.transfers
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.owners == nil {
		c.owners = map[string]bool{}
	}
	key := user + ":" + session
	if c.owners[key] || len(c.owners) >= 4 {
		return nil, false
	}
	c.owners[key] = true
	var once sync.Once
	return func() { once.Do(func() { c.mu.Lock(); delete(c.owners, key); c.mu.Unlock() }) }, true
}

// TransferOutcome bounds successful/failed range diagnostics to one per record per minute;
// expected interruptions stay debug-only. No provenance or owner/session values are logged.
func (m *Manager) TransferOutcome(id string, bytes int64, took time.Duration, reason string) {
	if reason == "cancelled" || reason == "revoked" {
		m.log.Debug("download transfer interrupted", "download_id", id, "reason_code", reason)
		return
	}
	c := &m.transfers
	c.mu.Lock()
	now := time.Now()
	if c.outcomes == nil {
		c.outcomes = map[string]time.Time{}
	}
	if until := c.outcomes[id]; now.Before(until) {
		c.mu.Unlock()
		return
	}
	if len(c.outcomes) >= 256 {
		for k, until := range c.outcomes {
			if !now.Before(until) {
				delete(c.outcomes, k)
			}
		}
	}
	if len(c.outcomes) >= 256 {
		c.mu.Unlock()
		return
	}
	c.outcomes[id] = now.Add(time.Minute)
	c.mu.Unlock()
	if reason != "" {
		m.log.Warn("download transfer unavailable", "download_id", id, "reason_code", reason, "took", took)
		return
	}
	m.log.Info("download transfer finished", "download_id", id, "bytes", bytes, "took", took)
}
