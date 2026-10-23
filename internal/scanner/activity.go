package scanner

import (
	"context"
	"fmt"

	"ams/internal/activity"
)

type activityWorkKey struct{}

func (s *Scanner) working(ctx context.Context, libraryID int64, stage, subject, detail string) (context.Context, func()) {
	s.mu.Lock()
	task := s.tasks[libraryID]
	s.mu.Unlock()
	work := task.Work(stage, subject, detail)
	return context.WithValue(ctx, activityWorkKey{}, work), work.Finish
}

func currentWork(ctx context.Context) *activity.Work {
	work, _ := ctx.Value(activityWorkKey{}).(*activity.Work)
	return work
}

func activityTitle(title string, year int) string {
	if year > 0 {
		return fmt.Sprintf("%s (%d)", title, year)
	}
	return title
}
