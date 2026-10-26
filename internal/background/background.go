// Package background coordinates optional work independently of media discovery.
package background

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"ams/internal/activity"
	"ams/internal/schedule"
	"ams/internal/settings"
	"ams/internal/store"
)

type Status struct {
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	State     string `json:"state"`
	Detail    string `json:"detail"`
	Pending   int    `json:"pending"`
	Completed int    `json:"completed"`
	Failed    int    `json:"failed"`
	Enabled   bool   `json:"enabled"`
}

var titles = map[string]string{"analysis": "Update media analysis", "previews": "Scrubbing previews", "markers": "Local intro and credits detection", "metadata": "Refresh known-title metadata", "backup": "Automatic database backups", "empty_trash": "Empty trash automatically"}

type waiter struct {
	ticket  uint64
	added   bool
	maximum int64
}

type Manager struct {
	waiting    map[string]waiter
	ticket     uint64
	completed  map[string]bool
	store      *store.Store
	Window     *schedule.Window
	Activity   *activity.Tracker
	mu         sync.Mutex
	generation int64
	cfg        settings.Settings
	force      map[string]int64
	wake       map[string]func()
	playing    func() bool
	slot       chan struct{}
	now        func() time.Time
}

func New(ctx context.Context, st *store.Store, w *schedule.Window, a *activity.Tracker) (*Manager, error) {
	if err := st.RecoverBackgroundJobs(ctx); err != nil {
		return nil, err
	}
	return &Manager{store: st, Window: w, Activity: a, force: map[string]int64{}, waiting: map[string]waiter{}, completed: map[string]bool{}, wake: map[string]func(){}, slot: make(chan struct{}, 1), now: time.Now}, nil
}
func (m *Manager) Configure(c settings.Settings) {
	m.mu.Lock()
	m.cfg = c
	m.completed = map[string]bool{}
	for k := range m.force {
		if m.modeLocked(k) == "off" {
			delete(m.force, k)
			delete(m.waiting, k)
		}
	}
	wakes := []func(){}
	for k, w := range m.wake {
		if k == "analysis" || k == "previews" || k == "markers" {
			wakes = append(wakes, w)
		}
	}
	m.mu.Unlock()
	for _, w := range wakes {
		w()
	}
}
func (m *Manager) SetPlayback(f func() bool)         { m.mu.Lock(); m.playing = f; m.mu.Unlock() }
func (m *Manager) Register(kind string, wake func()) { m.mu.Lock(); m.wake[kind] = wake; m.mu.Unlock() }
func (m *Manager) modeLocked(k string) string {
	switch k {
	case "analysis":
		if m.cfg.AnalysisEnabled {
			return "scheduled"
		}
	case "metadata":
		if m.cfg.MetadataRefreshEnabled {
			return "scheduled"
		}
	case "empty_trash":
		if m.cfg.EmptyTrashEnabled {
			return "scheduled"
		}
	case "backup":
		if m.cfg.AutoBackupsEnabled {
			return "scheduled"
		}
	case "previews":
		return m.cfg.PreviewMode
	case "markers":
		return m.cfg.DetectionMode
	}
	return "off"
}
func (m *Manager) busy() bool {
	m.mu.Lock()
	p := m.playing
	m.mu.Unlock()
	if p != nil && p() {
		return true
	}
	running, _ := m.Activity.Snapshot()
	for _, r := range running {
		if r.Kind == "scan" || r.Kind == "refresh" || r.Kind == "analysis" && r.CanStop {
			return true
		}
	}
	return false
}
func (m *Manager) allowed(kind string, newMedia bool, maxID int64) bool {
	m.mu.Lock()
	mode := m.modeLocked(kind)
	forced, hasForce := m.force[kind]
	m.mu.Unlock()
	return mode != "off" && (hasForce && maxID <= forced || m.Window.OpenAt(m.now()) || mode == "added" && newMedia) && !m.busy()
}
func (m *Manager) Mode(kind string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.modeLocked(kind)
}
func (m *Manager) RunNow(ctx context.Context, kind string) error {
	if kind == "empty_trash" {
		return errors.New("choose a library and confirm Empty trash for manual cleanup")
	}
	if _, ok := titles[kind]; !ok {
		return fmt.Errorf("unknown scheduled task")
	}
	if m.Mode(kind) == "off" {
		return fmt.Errorf("enable this task before running it")
	}
	id, err := m.store.MaxFileID(ctx)
	if err != nil {
		return err
	}
	if err = m.store.RetryBackgroundJobs(ctx, kind); err != nil {
		return err
	}
	m.mu.Lock()
	if _, already := m.force[kind]; already {
		m.mu.Unlock()
		return nil
	}
	m.force[kind] = id
	wake := m.wake[kind]
	m.mu.Unlock()
	if wake != nil {
		wake()
	}
	return nil
}
func (m *Manager) CompletePass(kind string) {
	m.mu.Lock()
	delete(m.force, kind)
	delete(m.waiting, kind)
	m.completed[kind] = true
	m.mu.Unlock()
}

// Begin gives one optional heavy job exclusive admission. Cancelling its lease stops
// FFmpeg/FFprobe through CommandContext; unfinished revisions are queued, never completed.
func (m *Manager) Begin(ctx context.Context, kind, key string, ids ...int64) (context.Context, func(error), bool) {
	return m.begin(ctx, kind, key, false, ids...)
}

// A missing preview artifact is pending work even if its old queue record was completed.
func (m *Manager) BeginPreview(ctx context.Context, id int64) (context.Context, func(error), bool) {
	return m.begin(ctx, "previews", fmt.Sprint(id), true, id)
}
func (m *Manager) begin(ctx context.Context, kind, key string, reopen bool, ids ...int64) (context.Context, func(error), bool) {
	if m == nil {
		return ctx, func(error) {}, true
	}
	revision, newest, maximum, err := m.store.BackgroundRevision(ctx, kind, ids)
	if err != nil {
		return ctx, nil, false
	}
	j, err := m.store.BackgroundJob(ctx, kind, key, revision, maximum)
	if err != nil || j.State == "done" && !reopen || j.State == "failed" && m.now().Unix()-j.UpdatedAt < 3600 {
		return ctx, nil, false
	}
	if j.State == "done" && reopen {
		if err := m.store.SetBackgroundJob(ctx, kind, key, revision, "queued", ""); err != nil {
			return ctx, nil, false
		}
	}
	m.mu.Lock()
	generation := m.generation
	epoch := m.cfg.BackgroundEpoch
	m.mu.Unlock()
	added := epoch > 0 && newest >= epoch
	m.mu.Lock()
	m.completed[kind] = false
	m.mu.Unlock()
	if !m.allowed(kind, added, maximum) {
		if j.State != "paused" {
			_ = m.store.SetBackgroundJob(ctx, kind, key, revision, "paused", "")
		}
		return ctx, nil, false
	}
	if !m.turn(kind, added, maximum) {
		return ctx, nil, false
	}
	if err = m.store.SetBackgroundJob(ctx, kind, key, revision, "running", ""); err != nil {
		<-m.slot
		return ctx, nil, false
	}
	work, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		timer := time.NewTicker(250 * time.Millisecond)
		defer timer.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-timer.C:
				m.mu.Lock()
				changed := generation != m.generation
				m.mu.Unlock()
				if changed || !m.allowed(kind, added, maximum) {
					cancel()
					return
				}
			}
		}
	}()
	var once sync.Once
	finish := func(err error) {
		once.Do(func() {
			cancel()
			<-stopped
			state, message := "done", ""
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				state = "queued"
			} else if err != nil {
				state, message = "failed", err.Error()
			}
			m.mu.Lock()
			current := generation == m.generation
			m.mu.Unlock()
			if ctx.Err() == nil && current {
				_ = m.store.SetBackgroundJob(ctx, kind, key, revision, state, message)
			}
			<-m.slot
		})
	}
	return work, finish, true
}
func (m *Manager) Snapshot(ctx context.Context) []Status {
	out := []Status{}
	for _, kind := range []string{"analysis", "previews", "markers", "metadata", "backup", "empty_trash"} {
		mode := m.Mode(kind)
		state, detail := "scheduled", "Waiting for maintenance"
		if mode == "off" {
			state, detail = "off", "Disabled"
		} else if m.busy() {
			state, detail = "paused", "Waiting for playback or scanning"
		} else if m.Window.OpenAt(m.now()) {
			state, detail = "queued", "Ready for eligible work"
		}
		if kind == "empty_trash" && mode != "off" {
			detail = "Checks complete scans; empty or unavailable folders are retained"
		}
		pending, done, failed, _ := m.store.BackgroundCounts(ctx, kind)
		running, _ := m.Activity.Snapshot()
		for _, r := range running {
			if r.Kind == kind || kind == "metadata" && r.Kind == "refresh" {
				state, detail = "running", r.Detail
			}
		}
		m.mu.Lock()
		complete := m.completed[kind]
		m.mu.Unlock()
		if complete && pending == 0 && done > 0 && state == "queued" {
			state, detail = "completed", "Completed work is remembered"
		}
		if mode != "off" && failed > 0 && pending == 0 {
			state, detail = "failed", "Some work will retry later"
		}
		out = append(out, Status{kind, titles[kind], state, detail, pending, done, failed, mode != "off"})
	}
	return out
}

// RunAnalysis drains database pages independently of scans. Only one bounded file is
// read at a time. A pause returns to the saved backlog at the next wake/tick.
func (m *Manager) RunAnalysis(ctx context.Context, analyze func(context.Context, store.MediaFile) error) {
	wake := make(chan struct{}, 1)
	m.Register("analysis", func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	for {
		if m.Mode("analysis") != "off" {
			m.analysisPass(ctx, analyze)
		}
		select {
		case <-ctx.Done():
			return
		case <-wake:
		case <-time.After(15 * time.Second):
		}
	}
}
func (m *Manager) analysisPass(ctx context.Context, analyze func(context.Context, store.MediaFile) error) {
	libs, err := m.store.ListLibraries(ctx)
	if err != nil {
		return
	}
	for _, lib := range libs {
		var after int64
		for ctx.Err() == nil {
			files, err := m.store.PendingFormatPage(ctx, lib.ID, after)
			if err != nil {
				return
			}
			if len(files) == 0 {
				break
			}
			for _, f := range files {
				after = f.ID
				work, finish, ok := m.Begin(ctx, "analysis", fmt.Sprint(f.ID), f.ID)
				if !ok {
					if m.outsideBacklog("analysis", f.ID) {
						continue
					}
					if m.Deferred("analysis") {
						return
					}
					if !m.allowed("analysis", false, f.ID) {
						return
					}
					continue
				}
				title, _ := m.store.FileActivityTitle(work, f.ID)
				if title == "" {
					title = filepath.Base(f.Path)
				}
				task := m.Activity.Start("analysis", "Analyzing formats in "+lib.Name)
				task.Detail("Background video, audio and HDR inspection")
				operation := task.Work("Inspecting source formats", title, filepath.Base(f.Path))
				err := analyze(work, f)
				operation.Finish()
				task.Detail("Video, audio and HDR inspection · " + title)
				paused := work.Err() != nil
				finish(err)
				task.Finish(err)
				if paused {
					return
				}
			}
		}
	}
	m.CompletePass("analysis")
}

func (m *Manager) Ready(kind string) bool { return m.allowed(kind, false, 0) }
func (m *Manager) Reset(ctx context.Context) {
	m.mu.Lock()
	m.generation++
	m.force = map[string]int64{}
	m.waiting = map[string]waiter{}
	m.mu.Unlock()
	_ = m.store.RecoverBackgroundJobs(ctx)
}

// CanStart cheaply filters an in-memory candidate list before revision/database admission.
func (m *Manager) CanStart(kind string, id, probedAt int64) bool {
	if m == nil {
		return true
	}
	m.mu.Lock()
	epoch := m.cfg.BackgroundEpoch
	m.mu.Unlock()
	return m.allowed(kind, epoch > 0 && probedAt >= epoch, id)
}

// Monitor wakes workers at window/foreground transitions instead of repeatedly walking their
// queues while scheduling or playback prevents work. Configure also wakes them immediately.
func (m *Manager) Monitor(ctx context.Context) {
	previous, previousWindow := false, false
	for {
		ready := len(m.slot) == 0 && !m.busy()
		open := m.Window.OpenAt(m.now())
		if ready && (!previous || open && !previousWindow) {
			m.mu.Lock()
			wakes := []func(){}
			for k, w := range m.wake {
				if k == "analysis" || k == "previews" || k == "markers" {
					wakes = append(wakes, w)
				}
			}
			m.mu.Unlock()
			for _, w := range wakes {
				w()
			}
		}
		previous = ready
		previousWindow = open
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func (m *Manager) Forced(kind string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.force[kind]
	return ok
}
func (m *Manager) Wake(kind string) {
	m.mu.Lock()
	wake := m.wake[kind]
	m.mu.Unlock()
	if wake != nil {
		wake()
	}
}

// Metadata is itself a refresh task, so it must not mistake its own Activity entry
// for competing foreground work. Playback and discovery still preempt it.
func (m *Manager) MaintenanceReady() bool {
	m.mu.Lock()
	enabled := m.cfg.MetadataRefreshEnabled
	_, forced := m.force["metadata"]
	playing := m.playing
	m.mu.Unlock()
	if !enabled || !forced && !m.Window.OpenAt(m.now()) || playing != nil && playing() {
		return false
	}
	running, _ := m.Activity.Snapshot()
	for _, r := range running {
		if r.Kind == "scan" || r.Kind == "analysis" && r.CanStop {
			return false
		}
	}
	return true
}

// FIFO between worker kinds prevents a large analysis backlog monopolising optional work.
func (m *Manager) turn(kind string, added bool, maximum int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	own, exists := m.waiting[kind]
	if !exists {
		m.ticket++
		own = waiter{m.ticket, added, maximum}
		m.waiting[kind] = own
	}
	open := m.Window.OpenAt(m.now())
	for k, w := range m.waiting {
		mode := m.modeLocked(k)
		forced, ok := m.force[k]
		eligible := mode != "off" && (open || ok && w.maximum <= forced || mode == "added" && w.added)
		if eligible && w.ticket < own.ticket {
			return false
		}
	}
	select {
	case m.slot <- struct{}{}:
		delete(m.waiting, kind)
		return true
	default:
		return false
	}
}
func (m *Manager) Deferred(kind string) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.waiting[kind]
	return ok
}
func (m *Manager) outsideBacklog(kind string, id int64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	last, ok := m.force[kind]
	return ok && id > last && !m.Window.OpenAt(m.now())
}

func (m *Manager) AbandonTurn(kind string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	delete(m.waiting, kind)
	m.mu.Unlock()
}
