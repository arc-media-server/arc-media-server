// Package schedule holds the maintenance window: the hours of the night heavy background work
// (refreshing metadata, backups, optionally scrubbing previews) waits for.
package schedule

import (
	"sync"
	"time"
)

// Window is a daily span of hours, e.g. 2 to 5 for 02:00–05:00; it may run past midnight (23 to
// 4). A nil Window is always open.
type Window struct {
	mu         sync.Mutex
	start, end int
}

// New makes a window from start to end (hours, 0–23).
func New(start, end int) *Window {
	w := &Window{}
	w.Set(start, end)
	return w
}

// Set changes the window's hours.
func (w *Window) Set(start, end int) {
	w.mu.Lock()
	w.start, w.end = start, end
	w.mu.Unlock()
}

// Open reports whether it's inside the window now.
func (w *Window) Open() bool { return w.OpenAt(time.Now()) }

// OpenAt reports whether t is inside the window. A window whose start and end are the same hour
// is open all day.
func (w *Window) OpenAt(t time.Time) bool {
	if w == nil {
		return true
	}
	w.mu.Lock()
	start, end := w.start, w.end
	w.mu.Unlock()
	h := t.Hour()
	switch {
	case start == end:
		return true
	case start < end:
		return h >= start && h < end
	default: // past midnight
		return h >= start || h < end
	}
}

// Night names the night t belongs to (the date the window started), so work done once per window
// isn't repeated when the window runs past midnight.
func (w *Window) Night(t time.Time) string {
	if w == nil {
		return t.Format(time.DateOnly)
	}
	w.mu.Lock()
	start, end := w.start, w.end
	w.mu.Unlock()
	if start > end && t.Hour() < end {
		t = t.AddDate(0, 0, -1)
	}
	return t.Format(time.DateOnly)
}

type Info struct {
	Now       time.Time `json:"now"`
	LocalTime string    `json:"local_time"`
	Timezone  string    `json:"timezone"`
	Open      bool      `json:"open"`
	AllDay    bool      `json:"all_day"`
	NextStart time.Time `json:"next_start,omitzero"`
	NextLocal string    `json:"next_local,omitempty"`
}

// Info uses the scheduler's own clock semantics, including DST skips/repeated local hours.
func (w *Window) Info(now time.Time) Info {
	out := Info{Now: now, LocalTime: now.Format("2006-01-02 15:04:05 -07:00 MST"), Timezone: now.Location().String(), Open: w.OpenAt(now), AllDay: true}
	if out.Timezone == "Local" {
		out.Timezone, _ = now.Zone()
	}
	if w != nil {
		w.mu.Lock()
		out.AllDay = w.start == w.end
		w.mu.Unlock()
	}
	if !out.Open {
		// Inspect real clock instants, not a possibly nonexistent/ambiguous time.Date hour.
		for next := now.Truncate(time.Minute).Add(time.Minute); next.Before(now.Add(49 * time.Hour)); next = next.Add(time.Minute) {
			if w.OpenAt(next) {
				out.NextStart, out.NextLocal = next, next.Format("2006-01-02 15:04 -07:00 MST")
				break
			}
		}
	}
	return out
}
