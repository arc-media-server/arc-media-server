// Package activity keeps track of what the server is busy with (library scans, scrubbing
// previews, backups), for the Activity menu.
package activity

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

// keep is how many finished tasks are remembered.
const keep = 20

// Task is one piece of background work.
type Task struct {
	ID    int64  `json:"id"`
	Kind  string `json:"kind"` // scan, refresh, previews, backup
	Title string `json:"title"`
	// What it's doing now, e.g. "Looking through folders · 120 files".
	Detail string `json:"detail,omitempty"`
	// Progress, when it's known: Done of Total.
	Done       int       `json:"done,omitempty"`
	Total      int       `json:"total,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitzero"`
	Error      string    `json:"error,omitempty"`
}

// Tracker holds the running tasks and the last few finished ones. A nil Tracker tracks nothing.
type Tracker struct {
	mu      sync.Mutex
	next    int64
	running []*Task
	recent  []Task
}

func New() *Tracker { return &Tracker{} }

// Handle reports on one task. Its methods do nothing on a nil Handle.
type Handle struct {
	t    *Tracker
	task *Task
}

// Start begins tracking a task.
func (t *Tracker) Start(kind, title string) *Handle {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	task := &Task{ID: t.next, Kind: kind, Title: title, StartedAt: time.Now()}
	t.running = append(t.running, task)
	return &Handle{t: t, task: task}
}

func (h *Handle) set(fn func(*Task)) {
	if h == nil {
		return
	}
	h.t.mu.Lock()
	fn(h.task)
	h.t.mu.Unlock()
}

// Detail says what the task is doing now.
func (h *Handle) Detail(s string) { h.set(func(t *Task) { t.Detail = s }) }

// Progress sets how far through the task is (total 0 for unknown).
func (h *Handle) Progress(done, total int) { h.set(func(t *Task) { t.Done, t.Total = done, total }) }

// Finish ends the task, with the error that stopped it (nil when it went fine).
func (h *Handle) Finish(err error) {
	if h == nil {
		return
	}
	t := h.t
	t.mu.Lock()
	defer t.mu.Unlock()
	h.task.FinishedAt = time.Now()
	switch {
	case errors.Is(err, context.Canceled) || err != nil && strings.HasSuffix(err.Error(), context.Canceled.Error()):
		// e.g. the page that asked for a refresh was closed, or Arc is stopping.
		h.task.Error = "Stopped before it finished"
	case err != nil:
		h.task.Error = err.Error()
	}
	for i, r := range t.running {
		if r == h.task {
			t.running = append(t.running[:i], t.running[i+1:]...)
			break
		}
	}
	t.recent = append([]Task{*h.task}, t.recent...)
	if len(t.recent) > keep {
		t.recent = t.recent[:keep]
	}
}

// Snapshot returns copies of the running tasks (oldest first) and the finished ones (newest first).
func (t *Tracker) Snapshot() (running, recent []Task) {
	running, recent = []Task{}, []Task{}
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, r := range t.running {
		running = append(running, *r)
	}
	return running, append(recent, t.recent...)
}
