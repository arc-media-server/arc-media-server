package background

import (
	"context"
	"errors"
	"fmt"
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
	if !m.Forced(kind) && (last.LastSuccessAt > m.now().Add(-7*24*time.Hour).Unix() || last.Error != "" && last.FinishedAt > m.now().Add(-time.Hour).Unix()) {
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
	last.StartedAt, last.FinishedAt, last.Error = m.now().Unix(), 0, ""
	if err := m.store.SaveMaintenanceRun(ctx, kind, last); err != nil {
		m.upkeepStateFailure(kind, err)
		return
	}
	task := m.Activity.Start(kind, titles[kind])
	if kind == "database" {
		task.Detail("Checking database integrity and available disk space")
	} else {
		task.Detail("Checking retained file and artwork references")
	}
	workCtx, cancel := context.WithCancel(ctx)
	task.Cancellable(cancel)
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
	if err != nil {
		last.Error = err.Error()
		if errors.Is(err, context.Canceled) {
			last.Error = "Paused before completion; will retry during maintenance"
		}
	} else {
		last.LastSuccessAt = last.FinishedAt
	}
	// Cancellation of the optional lease must not erase the last result. Only server shutdown
	// may leave an unfinished marker, which the next startup reports as interrupted.
	if saveErr := m.store.SaveMaintenanceRun(ctx, kind, last); saveErr != nil {
		err = errors.Join(err, fmt.Errorf("saving maintenance result: %w", saveErr))
	}
	task.Detail(detail)
	task.Finish(err)
	m.CompletePass(kind)
}

func (m *Manager) upkeepStateFailure(kind string, err error) {
	task := m.Activity.Start(kind, titles[kind])
	task.Detail("Could not start maintenance; resolve the reported error and retry")
	task.Finish(err)
	m.CompletePass(kind)
}
