package api

import (
	"testing"

	"ams/internal/store"
)

func TestMergeHome(t *testing.T) {
	ep := func(id, show string) store.Item { return store.Item{ID: id, Kind: "episode", ParentID: show} }
	inProgress := []homeEntry{
		{store.Item{ID: "heat", Kind: "movie"}, 300},
		{ep("ff-e3", "firefly"), 100},
	}
	upNext := []homeEntry{
		{ep("mt-e5", "mushoku"), 200},
		{ep("ff-e5", "firefly"), 400}, // watched an episode since stopping part-way through e3
	}
	got := mergeHome(inProgress, upNext, 30)
	var ids []string
	for _, e := range got {
		ids = append(ids, e.item.ID)
	}
	want := []string{"ff-e5", "heat", "mt-e5"}
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got %v, want %v", ids, want)
		}
	}
	if n := len(mergeHome(inProgress, upNext, 2)); n != 2 {
		t.Errorf("limit: %d", n)
	}
}

func TestNextInOrder(t *testing.T) {
	entries := []store.OrderingEntry{
		{EpisodeID: "s0e1", Season: 0, Episode: 1},
		{EpisodeID: "s0e2", Season: 0, Episode: 2},
		{EpisodeID: "pilot", Season: 1, Episode: 1},
		{EpisodeID: "pilot", Season: 1, Episode: 2}, // a two-part pilot in one file
		{EpisodeID: "e3", Season: 1, Episode: 3},
		{EpisodeID: "e4", Season: 1, Episode: 4}, // not on disk
		{EpisodeID: "s2e1", Season: 2, Episode: 1},
	}
	have := map[string]bool{"s0e1": true, "s0e2": true, "pilot": true, "e3": true, "s2e1": true}
	for current, want := range map[string]string{
		"pilot":   "e3",   // past both of its positions
		"e3":      "s2e1", // skips the missing episode, on into the next season
		"s2e1":    "",     // the last one
		"s0e1":    "s0e2", // specials carry on through specials
		"s0e2":    "",     // and stop there, not into season 1
		"unknown": "",
	} {
		if got := nextInOrder(entries, current, have); got != want {
			t.Errorf("after %s: got %q, want %q", current, got, want)
		}
	}
}
