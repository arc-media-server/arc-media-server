package schedule

import (
	"testing"
	"time"
)

func TestWindow(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 10, 18, h, 30, 0, 0, time.Local) }
	night := New(2, 5)
	for h, want := range map[int]bool{1: false, 2: true, 4: true, 5: false, 14: false} {
		if got := night.OpenAt(at(h)); got != want {
			t.Errorf("2–5 at %d:30 = %v", h, got)
		}
	}
	late := New(23, 4)
	for h, want := range map[int]bool{22: false, 23: true, 0: true, 3: true, 4: false} {
		if got := late.OpenAt(at(h)); got != want {
			t.Errorf("23–4 at %d:30 = %v", h, got)
		}
	}
	if late.Night(at(1)) != "2026-10-17" || late.Night(at(23)) != "2026-10-18" {
		t.Errorf("nights: %s, %s", late.Night(at(1)), late.Night(at(23)))
	}
	var none *Window
	if !none.OpenAt(at(12)) || !New(3, 3).OpenAt(at(12)) {
		t.Error("a nil or all-day window should be open")
	}
}
