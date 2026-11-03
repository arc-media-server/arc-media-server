package optimize

import (
	"ams/internal/events"
	"testing"
)

func TestFailedConversionPublishesAfterFailureIsSaved(t *testing.T) {
	x := conversionTest(t)
	x.m.Events = events.New()
	_, cursor, _ := x.m.Events.Read("")
	o := x.queue(t, 720)
	x.m.FFmpeg = func() string { return "" }
	if err := x.m.convert(t.Context(), o, nil); err == nil {
		t.Fatal("missing FFmpeg succeeded")
	}
	saved, err := x.st.OptimizedVersions(t.Context())
	if err != nil || len(saved) != 1 || saved[0].Status != "failed" {
		t.Fatalf("failure not persisted: %+v %v", saved, err)
	}
	hints, _, stale := x.m.Events.Read(cursor)
	if stale || len(hints) != 1 || hints[0].Topic != "optimized" {
		t.Fatalf("hints: %+v", hints)
	}
}
