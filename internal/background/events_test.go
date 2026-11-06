package background

import (
	"ams/internal/events"
	"ams/internal/settings"
	"context"
	"errors"
	"testing"
)

func TestBackgroundOutcomeHintsIncludeFailuresAndAreIdempotent(t *testing.T) {
	for _, kind := range []string{"analysis", "previews", "markers"} {
		for _, outcome := range []error{nil, errors.New("fixture failure"), context.Canceled} {
			t.Run(kind+"/"+fmtError(outcome), func(t *testing.T) {
				ctx, st, m, lib := fixture(t)
				f := savedFile(t, ctx, st, lib, "fixture.mkv", 99)
				m.Events = events.New()
				_, cursor, _ := m.Events.Read("")
				_, finish, ok := m.Begin(ctx, kind, "fixture", f.ID)
				if !ok {
					t.Fatal("work not admitted")
				}
				finish(outcome)
				hints, cursor, stale := m.Events.Read(cursor)
				want := kind
				if kind == "analysis" {
					want = "catalog"
				}
				topics := map[string]bool{}
				for _, hint := range hints {
					topics[hint.Topic] = true
					if hint.Topic == "maintenance" && !hint.Admin {
						t.Fatal("maintenance disclosed to non-admins")
					}
				}
				if stale || len(hints) != 2 || !topics[want] || !topics["maintenance"] {
					t.Fatalf("hints: %+v", hints)
				}
				finish(outcome)
				if again, _, _ := m.Events.Read(cursor); len(again) != 0 {
					t.Fatalf("duplicate completion: %+v", again)
				}
			})
		}
	}
}
func fmtError(err error) string {
	if err == nil {
		return "success"
	}
	return err.Error()
}

func TestUpkeepPublishesAfterResultIsSaved(t *testing.T) {
	ctx, st, m, _ := fixture(t)
	m.Events = events.New()
	_, cursor, _ := m.Events.Read("")
	m.Configure(settings.Settings{DatabaseUpkeepEnabled: true})
	m.upkeepPass(ctx, "database", func(context.Context, func() bool) (string, error) { return "fixture complete", nil })
	hints, _, stale := m.Events.Read(cursor)
	result, err := st.MaintenanceRun(ctx, "database")
	if err != nil || result.LastSuccessAt == 0 || result.Detail != "fixture complete" {
		t.Fatalf("result: %+v %v", result, err)
	}
	if stale || len(hints) != 1 || hints[0].Topic != "maintenance" || !hints[0].Admin {
		t.Fatalf("hints: %+v", hints)
	}
}
