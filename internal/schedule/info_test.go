package schedule

import (
	"testing"
	"time"
)

func TestInfoMatchesWindowAcrossDSTAndOvernight(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, now  string
		start, end int
		open, all  bool
		next       string
	}{
		{"daytime", "2026-10-25T12:00:00Z", 2, 5, false, false, "2026-10-26T02:00:00Z"},
		{"overnight closed", "2026-10-25T12:00:00Z", 23, 4, false, false, "2026-10-25T23:00:00Z"},
		{"overnight open", "2026-10-25T01:30:00Z", 23, 4, true, false, ""},
		{"all day", "2026-10-25T12:00:00Z", 6, 6, true, true, ""},
		{"spring nonexistent hour", "2026-03-29T00:30:00Z", 1, 4, false, false, "2026-03-29T01:00:00Z"},
		{"fall repeated hour", "2026-10-25T00:30:00Z", 1, 2, true, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tc.now)
			now = now.In(london)
			got := New(tc.start, tc.end).Info(now)
			if got.Open != tc.open || got.AllDay != tc.all || got.LocalTime != now.Format("2006-01-02 15:04:05 -07:00 MST") || got.Timezone != "Europe/London" {
				t.Fatalf("clock %+v", got)
			}
			if tc.next != "" && got.NextStart.UTC().Format(time.RFC3339) != tc.next {
				t.Fatalf("next %s want %s", got.NextStart, tc.next)
			}
		})
	}
}
