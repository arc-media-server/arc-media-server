package store

import (
	"sync"
	"sync/atomic"
)

type catalogGate struct {
	mu       sync.RWMutex
	reserved atomic.Bool
}

// BeginCatalogWork protects a runtime operation that retains catalog identities between reads
// and writes. Async callers acquire before capturing work and release only when it finishes.
func (s *Store) BeginCatalogWork() (func(), bool) {
	if s == nil {
		return func() {}, true
	}
	if !s.catalog.mu.TryRLock() {
		return nil, false
	}
	var once sync.Once
	return func() { once.Do(s.catalog.mu.RUnlock) }, true
}

// ReserveCatalogMutation atomically requires quiescence and blocks new catalog work across
// safety backup, destructive mutation and runtime reload. Busy work is not forcibly aborted.
func (s *Store) ReserveCatalogMutation() (func(), bool) {
	if s == nil {
		return func() {}, true
	}
	if !s.catalog.mu.TryLock() {
		return nil, false
	}
	s.catalog.reserved.Store(true)
	var once sync.Once
	return func() { once.Do(func() { s.catalog.reserved.Store(false); s.catalog.mu.Unlock() }) }, true
}
func (s *Store) CatalogReserved() bool { return s != nil && s.catalog.reserved.Load() }
