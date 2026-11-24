package store

import (
	"reflect"
	"testing"
)

func TestSeriesDisplayOrdersMatchesSingleSeries(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	lib, err := st.SyncLibrary(ctx, "Shows", "shows", "tvdb_aired", nil)
	check(err)

	var ids []string
	want := map[string]SeriesDisplayOrder{}
	for _, choice := range []string{"library default", "file order override", "explicit custom", "no order"} {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: choice}, choice)
		check(err)
		ids = append(ids, id)
		if choice == "no order" {
			continue
		}
		var entryIDs []string
		for _, key := range []string{"regular", "special"} {
			ep, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: id, Title: key}, choice+key)
			check(err)
			entryIDs = append(entryIDs, ep)
		}
		fallback := "tvdb_aired"
		if choice == "file order override" {
			fallback = "file"
			check(st.SetSourceOrdering(ctx, id, fallback))
		}
		for i, source := range []string{"tmdb_aired", "tvdb_aired", "file", "custom"} {
			order, err := st.UpsertOrdering(ctx, id, source, "", source, "")
			check(err)
			check(st.ReplaceOrdering(ctx, order, nil, nil, []OrderingEntry{
				{EpisodeID: entryIDs[0], Season: i + 1, Episode: 1},
				{EpisodeID: entryIDs[1], Season: 0, Episode: 1},
			}))
			if choice == "explicit custom" && source == "custom" {
				check(st.SetDisplayOrdering(ctx, id, "", order))
			}
		}
		on := true
		check(st.SetInlineSpecials(ctx, id, &on))
		check(st.ReplaceSpecialPlacements(ctx, id, map[string]string{entryIDs[1]: entryIDs[0]}))
		orderID, err := st.DisplayOrdering(ctx, id, fallback)
		check(err)
		order, err := st.GetOrdering(ctx, orderID)
		check(err)
		entries, err := st.DisplayEntries(ctx, orderID)
		check(err)
		want[id] = SeriesDisplayOrder{Source: order.Source, Entries: entries}
	}
	got, err := st.SeriesDisplayOrders(ctx, ids)
	check(err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("batch display orders = %+v, want %+v", got, want)
	}
}
