package activity

import (
	"errors"
	"fmt"
	"sync"
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

func TestConcurrentWorkSnapshotsAndCleanup(t *testing.T) {
	tr := New()
	a, b := tr.Start("scan", "Movies"), tr.Start("scan", "Series")
	first := a.Work("Matching movie", "First", "TMDB")
	second := a.Work("Inspecting file", "Second", "Second.mkv")
	other := b.Work("Linking episodes", "Example Show", "Saved metadata")
	running, _ := tr.Snapshot()
	running[0].Current[0].Subject = "mutated snapshot"
	first.Update("Saving movie details", "Artwork references")
	second.Progress(0, 10)
	running, _ = tr.Snapshot()
	if len(running[0].Current) != 2 || running[0].Current[0].Subject != "First" || running[1].Current[0].Subject != "Example Show" {
		t.Fatalf("concurrent operations: %+v", running)
	}
	first.Finish()
	running, _ = tr.Snapshot()
	if len(running[0].Current) != 1 || running[0].Current[0].Subject != "Second" {
		t.Fatalf("wrong worker removed: %+v", running)
	}
	var wg sync.WaitGroup
	for n := range 6 {
		wg.Go(func() {
			for range 100 {
				w := a.Work("Inspecting file", fmt.Sprint(n), "File")
				w.Update("Saving", "Details")
				tr.Snapshot()
				w.Finish()
			}
		})
	}
	wg.Wait()
	a.Finish(nil)
	second.Update("late update", "Must not reappear")
	second.Finish()
	_, recent := tr.Snapshot()
	if len(recent) != 1 || len(recent[0].Current) != 0 {
		t.Fatalf("finished work leaked: %+v", recent)
	}
	other.Finish()
	var none *Handle
	w := none.Work("Stage", "Title", "Detail")
	w.Update("Stage", "Detail")
	w.Progress(0, 0)
	w.Finish()
}

func TestDiscardLeavesNoFinishedEntry(t *testing.T) {
	tr := New()
	quiet := tr.Start("markers", "Finding intros and credits")
	quiet.Cancellable(func() {})
	kept := tr.Start("scan", "Scanning Movies")
	quiet.Discard()
	kept.Finish(nil)
	running, recent := tr.Snapshot()
	if len(running) != 0 {
		t.Errorf("running = %+v", running)
	}
	if len(recent) != 1 || recent[0].Title != "Scanning Movies" {
		t.Errorf("recent = %+v", recent)
	}
	if tr.Stop(quiet.task.ID) {
		t.Error("a discarded task can still be stopped")
	}
	var none *Handle
	none.Discard()
}
