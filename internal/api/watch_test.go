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
