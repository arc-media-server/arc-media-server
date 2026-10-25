package store

import "testing"

func TestCompletionBoundary(t *testing.T) {
	for _, tc := range []struct {
		pos, duration int64
		percent       int
		want          bool
	}{
		{899999, 1000000, 0, false}, {900000, 1000000, 0, true},
		{949999, 1000000, 95, false}, {950000, 1000000, 95, true},
		{999999, 1000000, 100, false}, {1000000, 1000000, 100, true},
		{1000001, 1000000, 100, true}, {1000000, 0, 90, false},
		{900000, 1000000, -1, true}, {900000, 1000000, 101, true},
	} {
		if got := CompletionReached(tc.pos, tc.duration, tc.percent); got != tc.want {
			t.Errorf("%+v got %v", tc, got)
		}
	}
}
