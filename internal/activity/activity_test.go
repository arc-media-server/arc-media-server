package activity

import (
	"errors"
	"testing"
)

func TestTracker(t *testing.T) {
	tr := New()
	a := tr.Start("scan", "Scanning Movies")
	b := tr.Start("backup", "Backing up")
	a.Progress(3, 10)
	a.Detail("Getting details")
	b.Finish(errors.New("disk full"))

	running, recent := tr.Snapshot()
	if len(running) != 1 || running[0].Title != "Scanning Movies" || running[0].Done != 3 || running[0].Detail != "Getting details" {
		t.Errorf("running = %+v", running)
	}
	if len(recent) != 1 || recent[0].Error != "disk full" || recent[0].FinishedAt.IsZero() {
		t.Errorf("recent = %+v", recent)
	}

	// A nil tracker and handle do nothing.
	var none *Tracker
	h := none.Start("scan", "x")
	h.Progress(1, 2)
	h.Finish(nil)
	if r, _ := none.Snapshot(); len(r) != 0 {
		t.Error("nil tracker has tasks")
	}
}
