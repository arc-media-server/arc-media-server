package scanner

import (
	"testing"
	"time"
)

func TestVideoNames(t *testing.T) {
	mod := time.Date(2021, 3, 4, 10, 0, 0, 0, time.Local)
	for _, c := range []struct{ path, title, date string }{
		{"/v/Holidays/Beach_day.at.Whitby.mp4", "Beach day at Whitby", "2021-03-04"},
		{"/v/2019-07-14 Wedding.mkv", "2019-07-14 Wedding", "2019-07-14"},
		{"/v/IMG_20180102_101500.mov", "IMG 20180102 101500", "2018-01-02"},
		{"/v/Party 1999.mp4", "Party 1999", "2021-03-04"},
	} {
		if got := videoTitle(c.path); got != c.title {
			t.Errorf("videoTitle(%q) = %q, want %q", c.path, got, c.title)
		}
		if got := videoDate(c.path, mod); got != c.date {
			t.Errorf("videoDate(%q) = %q, want %q", c.path, got, c.date)
		}
	}
}
