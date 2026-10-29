package background

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

func upkeepKind(kind string) bool { return kind == "database" || kind == "cache" }

// RunUpkeep runs server-wide maintenance weekly, or one explicit Run now request. Optional
// work shares the same FIFO slot as analysis/previews and yields to foreground work/playback.
func (m *Manager) RunUpkeep(ctx context.Context, kind string, work func(context.Context, func() bool) (string, error)) {
	wake := make(chan struct{}, 1)
	m.Register(kind, func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		m.upkeepPass(ctx, kind, work)
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-wake:
		}
	}
}

func (m *Manager) upkeepPass(ctx context.Context, kind string, work func(context.Context, func() bool) (string, error)) {
	if !upkeepKind(kind) {
		return
	}
	last, err := m.store.MaintenanceRun(ctx, kind)
	if err != nil {
		m.upkeepStateFailure(kind, err)
		return
	}
	if last.StartedAt > 0 && last.FinishedAt == 0 {
		last.FinishedAt, last.Error = m.now().Unix(), "Interrupted by a server restart; retry during maintenance or use Run now"
		if err := m.store.SaveMaintenanceRun(ctx, kind, last); err != nil {
			m.upkeepStateFailure(kind, err)
			return
		}
	}
	if !m.Ready(kind) {
		return
	}
	if !m.Forced(kind) && (last.Stopped && last.FinishedAt > m.now().Add(-7*24*time.Hour).Unix() || last.LastSuccessAt > m.now().Add(-7*24*time.Hour).Unix() || last.Error != "" && last.FinishedAt > m.now().Add(-time.Hour).Unix()) {
		return
	}
	if !m.turn(kind, false, 0) {
		return
	}
	defer func() { <-m.slot }()
	allowed := func() bool { return m.Ready(kind) }
	if !allowed() {
		return
	}
	last.StartedAt, last.FinishedAt, last.Error, last.Paused, last.Stopped = m.now().Unix(), 0, "", false, false
	if err := m.store.SaveMaintenanceRun(ctx, kind, last); err != nil {
		m.upkeepStateFailure(kind, err)
		return
	}
	started := time.Now()
	m.logger().Info("maintenance started", "kind", kind)
	task := m.Activity.Start(kind, titles[kind])
	if kind == "database" {
		task.Detail("Checking database integrity and available disk space")
	} else {
		task.Detail("Checking retained file and artwork references")
	}
	workCtx, cancel := context.WithCancel(ctx)
	var stopRequested atomic.Bool
	task.Cancellable(func() { stopRequested.Store(true); cancel() })
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		tick := time.NewTicker(250 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-workCtx.Done():
				return
			case <-tick.C:
				if !allowed() {
					cancel()
					return
				}
			}
		}
	}()
	detail, err := work(workCtx, allowed)
	cancel()
	<-stopped
	last.FinishedAt, last.Detail = m.now().Unix(), detail
	stoppedByUser := stopRequested.Load() && errors.Is(err, context.Canceled)
	paused := errors.Is(err, context.Canceled) && !stoppedByUser
	if stoppedByUser {
		last.Stopped = true
		last.Detail = "Stopped by request. " + detail
	} else if paused {
		last.Paused = true
		last.Detail = "Paused before completion; waiting to retry. " + detail
	} else if err != nil {
		last.Error = err.Error()
	} else {
		last.LastSuccessAt = last.FinishedAt
	}
	// Keep the explicit request pending on preemption, so it resumes after
	// foreground work instead of disappearing or becoming a server error.
	if saveErr := m.store.SaveMaintenanceRun(ctx, kind, last); saveErr != nil && !errors.Is(saveErr, context.Canceled) {
		err = errors.Join(err, fmt.Errorf("saving maintenance result: %w", saveErr))
		paused, stoppedByUser = false, false
	}
	task.Detail(last.Detail)
	if paused || errors.Is(ctx.Err(), context.Canceled) {
		task.Finish(nil)
		m.logger().Info("maintenance paused", "kind", kind, "took", time.Since(started).Round(time.Millisecond))
		return
	}
	if stoppedByUser {
		task.Finish(nil)
		m.logger().Info("maintenance stopped", "kind", kind, "took", time.Since(started).Round(time.Millisecond))
		m.CompletePass(kind)
		return
	}
	task.Finish(err)
	if err != nil {
		m.logger().Warn("maintenance failed", "kind", kind, "took", time.Since(started).Round(time.Millisecond), "err", err)
	} else {
		m.logger().Info("maintenance finished", "kind", kind, "took", time.Since(started).Round(time.Millisecond), "detail", detail)
	}
	m.CompletePass(kind)
}

func (m *Manager) upkeepStateFailure(kind string, err error) {
	if !errors.Is(err, context.Canceled) {
		m.logger().Warn("maintenance could not start", "kind", kind, "err", err)
	}
	task := m.Activity.Start(kind, titles[kind])
	task.Detail("Could not start maintenance; resolve the reported error and retry")
	task.Finish(err)
	m.CompletePass(kind)
}
