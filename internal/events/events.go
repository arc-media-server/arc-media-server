// Package events coalesces invalidation hints in bounded memory. It creates no goroutines,
// retains no item metadata and never blocks a producer on a slow network consumer.
package events

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
)

const Retained = 256
const Pending = 128
const MaxStreams = 32

type Event struct {
	ID        string `json:"-"`
	Topic     string `json:"topic"`
	LibraryID int64  `json:"library_id,omitempty"`
	UserID    string `json:"-"`
	Admin     bool   `json:"-"`
}
type Hub struct {
	mu       sync.Mutex
	epoch    string
	sequence uint64
	ring     []Event
	pending  map[string]Event
	streams  int
}

func New() *Hub {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return &Hub{epoch: hex.EncodeToString(b[:]), pending: map[string]Event{}}
}
func (h *Hub) Publish(e Event) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	key := fmt.Sprintf("%s/%s/%d/%t", e.Topic, e.UserID, e.LibraryID, e.Admin)
	if len(h.pending) >= Pending {
		if _, exists := h.pending[key]; !exists {
			h.pending = map[string]Event{"resync": {Topic: "resync"}}
		}
	}
	h.pending[key] = e
}
func (h *Hub) Open() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.streams >= MaxStreams {
		return false
	}
	h.streams++
	return true
}
func (h *Hub) Close()         { h.mu.Lock(); defer h.mu.Unlock(); h.streams-- }
func (h *Hub) cursor() string { return h.epoch + ":" + strconv.FormatUint(h.sequence, 10) }

// Read flushes coalesced pending hints. Invalid/stale/previous-process cursors require a full
// refetch. All clients advance past filtered events, without disclosing their private payloads.
func (h *Hub) Read(after string) ([]Event, string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	keys := make([]string, 0, len(h.pending))
	for k := range h.pending {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e := h.pending[k]
		h.sequence++
		e.ID = h.cursor()
		h.ring = append(h.ring, e)
	}
	clear(h.pending)
	if len(h.ring) > Retained {
		h.ring = h.ring[len(h.ring)-Retained:]
	}
	epoch, raw, ok := strings.Cut(after, ":")
	seq, err := strconv.ParseUint(raw, 10, 64)
	stale := !ok || err != nil || epoch != h.epoch || seq > h.sequence
	if !stale && len(h.ring) > 0 {
		stale = seq+uint64(len(h.ring)) < h.sequence
	}
	latest := h.cursor()
	if stale {
		return nil, latest, true
	}
	out := []Event{}
	for _, e := range h.ring {
		_, n, _ := strings.Cut(e.ID, ":")
		v, _ := strconv.ParseUint(n, 10, 64)
		if v > seq {
			out = append(out, e)
		}
	}
	return out, latest, false
}
