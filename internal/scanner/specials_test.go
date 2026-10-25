package scanner

import (
	"ams/internal/metadata"
	"reflect"
	"testing"
)

func TestSpecialHintsUseMappedIdentityAndExplicitTargets(t *testing.T) {
	p := func(n int) *int { return &n }
	entries := []metadata.Entry{
		{Season: 2, Number: 3, Episode: &metadata.Episode{ID: 23}},
		{Season: 2, Number: 1, Episode: &metadata.Episode{ID: 21}},
	}
	for i, hint := range []struct{ s, e *int }{{p(2), p(3)}, {p(2), nil}, {p(2), p(0)}, {nil, p(1)}, {p(0), p(1)}, {p(99), p(1)}, {p(2), p(99)}, {p(2), p(-1)}, {p(2), p(1)}} {
		entries = append(entries, metadata.Entry{Season: 0, Number: i + 1, Episode: &metadata.Episode{ID: int64(i + 1), AirsBeforeSeason: hint.s, AirsBeforeEpisode: hint.e}})
	}
	stable := map[int64]string{1: "special1", 2: "special2", 3: "special3", 4: "unknown", 5: "invalid", 6: "absentseason", 7: "absentepisode", 8: "negative", 21: "target1", 23: "target3"}
	got := specialPlacements(metadata.Ordering{Entries: entries}, func(id int64) string { return stable[id] })
	want := map[string]string{"special1": "target3", "special2": "target1", "special3": "target1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("placements=%v want %v", got, want)
	}
	delete(stable, 21)
	got = specialPlacements(metadata.Ordering{Entries: entries}, func(id int64) string { return stable[id] })
	if !reflect.DeepEqual(got, map[string]string{"special1": "target3"}) {
		t.Fatalf("guessed an unmapped target: %v", got)
	}
}
