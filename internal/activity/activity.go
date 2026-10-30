// Package activity keeps track of what the server is busy with (library scans, scrubbing
// previews, backups), for the Activity menu.
package activity

import (
	"context"
	"crypto/rand"
	"encoding/binary"
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
	// Independently tracked concurrent operations; never a last-worker-wins label.
	Current []Operation `json:"current,omitempty"`
	// Progress, when it's known: Done of Total.
	Done       int       `json:"done,omitempty"`
	Total      int       `json:"total,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at,omitzero"`
	Error      string    `json:"error,omitempty"`
	CanStop    bool      `json:"can_stop,omitempty"`
	Stopping   bool      `json:"stopping,omitempty"`
	Stopped    bool      `json:"stopped,omitempty"`
}

// Operation is one active title/file within a task. Detail contains a filename or
// short explanation, not a full media path or provider response.
type Operation struct {
	ID      int64  `json:"id"`
	Stage   string `json:"stage"`
	Subject string `json:"subject"`
	Detail  string `json:"detail,omitempty"`
	Done    int    `json:"done,omitempty"`
	Total   int    `json:"total,omitempty"`
}

// Tracker holds the running tasks and the last few finished ones. A nil Tracker tracks nothing.
type Tracker struct {
	onChange      func()
	mu            sync.Mutex
	next          int64
	nextOperation int64
	running       []*Task
	recent        []Task
	cancels       map[int64]func()
}

// SetOnChange installs a cheap nonblocking invalidation hook; it must never call back into Tracker.
func (t *Tracker) SetOnChange(fn func()) { t.mu.Lock(); defer t.mu.Unlock(); t.onChange = fn }
func (t *Tracker) changedLocked() {
	if t.onChange != nil {
		t.onChange()
	}
}

// NewRunID seeds exact-in-JavaScript numeric IDs independently for each server lifetime.
func NewRunID() int64 {
	var seed [8]byte
	_, _ = rand.Read(seed[:])
	return int64(binary.BigEndian.Uint64(seed[:]) & ((1 << 52) - 1))
}

func New() *Tracker { return &Tracker{next: NewRunID()} }

// Cancellable attaches a job-owned cancellation callback. Stop never calls it under the tracker lock.
func (h *Handle) Cancellable(cancel func()) {
	if h == nil {
		return
	}
	h.t.mu.Lock()
	defer h.t.mu.Unlock()
	if !h.task.FinishedAt.IsZero() {
		return
	}
	if h.t.cancels == nil {
		h.t.cancels = map[int64]func(){}
	}
	h.t.cancels[h.task.ID] = cancel
	h.task.CanStop = true
	h.t.changedLocked()
}

func (h *Handle) Stopping() { h.set(func(t *Task) { t.Stopping = true }) }

// Stop is idempotent for the same task, including an already stopped recent task.
func (t *Tracker) Stop(id int64) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	for _, task := range t.running {
		if task.ID != id {
			continue
		}
		cancel := t.cancels[id]
		if cancel == nil {
			t.mu.Unlock()
			return false
		}
		if task.Stopping {
			t.mu.Unlock()
			return true
		}
		task.Stopping = true
		t.changedLocked()
		t.mu.Unlock()
		cancel()
		return true
	}
	for _, task := range t.recent {
		if task.ID == id && task.Stopped {
			t.mu.Unlock()
			return true
		}
	}
	t.mu.Unlock()
	return false
}

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
	t.changedLocked()
	return &Handle{t: t, task: task}
}

func (h *Handle) set(fn func(*Task)) {
	if h == nil {
		return
	}
	h.t.mu.Lock()
	fn(h.task)
	h.t.changedLocked()
	h.t.mu.Unlock()
}

// ID is the task's id, as the Activity list shows it (0 on a nil Handle).
func (h *Handle) ID() int64 {
	if h == nil {
		return 0
	}
	return h.task.ID
}

// Detail says what the task is doing now.
func (h *Handle) Detail(s string) { h.set(func(t *Task) { t.Detail = s }) }

// Progress sets how far through the task is (total 0 for unknown).
func (h *Handle) Progress(done, total int) { h.set(func(t *Task) { t.Done, t.Total = done, total }) }

// Work reports one concurrent operation. The bound exceeds current scanner worker
// counts; reporting never creates an unbounded per-file history.
func (h *Handle) Work(stage, subject, detail string) *Work {
	if h == nil {
		return nil
	}
	h.t.mu.Lock()
	defer h.t.mu.Unlock()
	if !h.task.FinishedAt.IsZero() || len(h.task.Current) >= 16 {
		return nil
	}
	h.t.nextOperation++
	id := h.t.nextOperation
	h.task.Current = append(h.task.Current, Operation{ID: id, Stage: stage, Subject: subject, Detail: detail})
	h.t.changedLocked()
	return &Work{h: h, id: id}
}

// Work is scoped to a worker; finish it on every exit, including cancellation.
type Work struct {
	h  *Handle
	id int64
}

func (w *Work) change(fn func(*Operation)) {
	if w == nil {
		return
	}
	w.h.set(func(t *Task) {
		for i := range t.Current {
			if t.Current[i].ID == w.id {
				fn(&t.Current[i])
				return
			}
		}
	})
}
func (w *Work) Update(stage, detail string) {
	w.change(func(o *Operation) { o.Stage, o.Detail, o.Done, o.Total = stage, detail, 0, 0 })
}
func (w *Work) Progress(done, total int) {
	w.change(func(o *Operation) { o.Done, o.Total = done, total })
}
func (w *Work) Finish() {
	if w == nil {
		return
	}
	w.h.set(func(t *Task) {
		for i, o := range t.Current {
			if o.ID == w.id {
				t.Current = append(t.Current[:i], t.Current[i+1:]...)
				return
			}
		}
	})
}

// Finish ends the task, with the error that stopped it (nil when it went fine).
func (h *Handle) Finish(err error) {
	if h == nil {
		return
	}
	t := h.t
	t.mu.Lock()
	defer t.mu.Unlock()
	defer t.changedLocked()
	h.task.FinishedAt = time.Now()
	h.task.Current = nil
	delete(t.cancels, h.task.ID)
	h.task.CanStop = false
	if h.task.Stopping {
		h.task.Stopped, h.task.Stopping = true, false
		h.task.Detail = "Stopped · " + h.task.Detail
		err = nil
	}
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

// Discard ends a task that turned out to have nothing to do, without listing it as finished,
// so routine checks don't crowd out the work people want to see.
func (h *Handle) Discard() {
	if h == nil {
		return
	}
	t := h.t
	t.mu.Lock()
	defer t.mu.Unlock()
	defer t.changedLocked()
	delete(t.cancels, h.task.ID)
	for i, r := range t.running {
		if r == h.task {
			t.running = append(t.running[:i], t.running[i+1:]...)
			break
		}
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
		copy := *r
		copy.Current = append([]Operation(nil), r.Current...)
		running = append(running, copy)
	}
	return running, append(recent, t.recent...)
}
