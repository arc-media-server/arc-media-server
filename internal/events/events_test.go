package events

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCoalescingReplayAndBounds(t *testing.T) {
	h := New()
	_, cursor, reset := h.Read("")
	if !reset {
		t.Fatal("initial resync")
	}
	for range 1000 {
		h.Publish(Event{Topic: "profile", UserID: "secret"})
	}
	batch, next, reset := h.Read(cursor)
	if reset || len(batch) != 1 {
		t.Fatal("coalescing", len(batch), reset)
	}
	b, _ := json.Marshal(batch[0])
	if strings.Contains(string(b), "secret") {
		t.Fatal("private fields serialized")
	}
	replay, _, reset := h.Read(cursor)
	if reset || len(replay) != 1 {
		t.Fatal("replay")
	}
	h.Publish(Event{Topic: "profile", UserID: "secret"})
	batch, cursor, reset = h.Read(next)
	if reset || len(batch) != 1 {
		t.Fatal("trailing update lost")
	}
	for i := range Retained + 1 {
		h.Publish(Event{Topic: fmt.Sprint(i)})
		h.Read(cursor)
	}
	if _, _, reset = h.Read(cursor); !reset {
		t.Fatal("old cursor accepted")
	}
	if _, _, reset = New().Read(cursor); !reset {
		t.Fatal("old process accepted")
	}
	for i := range Pending + 2 {
		h.Publish(Event{Topic: fmt.Sprint(i)})
	}
	if len(h.pending) > Pending {
		t.Fatal("pending unbounded")
	}
	_, cursor, _ = h.Read("")
	h.Publish(Event{Topic: "ok"})
	if batch, _, reset = h.Read(cursor); reset || len(batch) != 1 {
		t.Fatal("overflow recovery")
	}
	for i := range MaxStreams {
		if !h.Open(fmt.Sprint(i)) {
			t.Fatal("early cap")
		}
	}
	if h.Open("overflow") {
		t.Fatal("cap not enforced")
	}
	h.Close("0")
	if !h.Open("replacement") {
		t.Fatal("slot not released")
	}
}
func TestConcurrentPublishRead(t *testing.T) {
	h := New()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, cursor, _ := h.Read("")
			for range 500 {
				h.Publish(Event{Topic: "activity", Admin: true})
				_, cursor, _ = h.Read(cursor)
			}
		})
	}
	wg.Wait()
}

func TestConcurrentProfileAdmissionAndDisconnect(t *testing.T) {
	h := New()
	var wg sync.WaitGroup
	var admitted atomic.Int32
	for range 100 {
		wg.Go(func() {
			if h.Open("one-profile") {
				admitted.Add(1)
			}
		})
	}
	wg.Wait()
	if got := admitted.Load(); got != MaxProfileStreams {
		t.Fatalf("admitted %d", got)
	}
	if !h.Open("another-profile") {
		t.Fatal("one profile starved another")
	}
	for range MaxProfileStreams {
		h.Close("one-profile")
	}
	if len(h.profileStreams) != 1 {
		t.Fatal("disconnected profiles retained")
	}
	if !h.Open("one-profile") {
		t.Fatal("released profile cannot reconnect")
	}
	h.Close("missing-profile")
	if h.streams != 2 {
		t.Fatal("unpaired close changed global capacity")
	}
	if h.Open("") {
		t.Fatal("empty principal accepted")
	}
}
