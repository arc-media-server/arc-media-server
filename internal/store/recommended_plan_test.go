package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The queries Recommended runs for a library must look up watch state by title (the
// user_item_data key, or a show's episodes through items_parent). Starting from the profile's
// whole watch history instead reads it once per title: on a large library with a long history
// that took the owner's server about 4.5 seconds for one Recommended page.
func TestRecommendedQueriesLookUpWatchStateByTitle(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, filepath.Join(t.TempDir(), "ams.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(ctx, "TV", "shows", "tmdb_aired", []string{"/tv"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	// A little real data, so the plans are the ones SQLite picks for a populated library.
	var watched []string
	for s := range 3 {
		sid, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Show"}, string(rune('a'+s)))
		if err != nil {
			t.Fatal(err)
		}
		for e := range 4 {
			key := string(rune('a'+s)) + string(rune('0'+e))
			eid, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: sid, Title: key}, key)
			if err != nil {
				t.Fatal(err)
			}
			watched = append(watched, eid)
		}
	}
	if err := st.SetPlayed(ctx, u.ID, watched, true); err != nil {
		t.Fatal(err)
	}

	plan := func(name, q string, args []any) {
		t.Helper()
		rows, err := st.db.QueryContext(ctx, `EXPLAIN QUERY PLAN `+q, args...)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer rows.Close()
		var lines []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			lines = append(lines, detail)
		}
		for _, l := range lines {
			// Searching user_item_data by the profile alone (its recent-history index, without a
			// played-since bound) means reading the whole history; a full scan of a temporary
			// summary means reading it again for every row.
			if strings.HasSuffix(l, "user_item_data_recent (user_id=?)") || strings.HasPrefix(l, "SCAN watched") {
				t.Errorf("%s reads the watch history per title:\n%s", name, strings.Join(lines, "\n"))
				return
			}
		}
	}
	listing := func(name string, q ItemQuery) {
		q.LibraryID, q.UserID, q.Limit = lib, u.ID, 20
		cond, args := itemFilter(q)
		order, orderArgs := itemOrdering(q)
		plan(name+" (count)", `SELECT COUNT(*) FROM items WHERE `+cond, args)
		plan(name, `SELECT id FROM items WHERE `+cond+` ORDER BY `+order+` LIMIT 20`, append(append([]any{}, args...), orderArgs...))
	}
	listing("Start Watching", ItemQuery{Kind: "series", Unstarted: true, Sort: "shuffle"})
	listing("Recently Watched (episodes)", ItemQuery{Kind: "episode", Played: true, Sort: "watched"})
	listing("Recently Watched (shows)", ItemQuery{Kind: "series", Played: true, Sort: "watched"})
	listing("Rediscover", ItemQuery{Kind: "series", PlayedBefore: time.Now().Add(-180 * 24 * time.Hour).Unix(), Sort: "shuffle"})
	listing("Unwatched", ItemQuery{Kind: "series", Unwatched: true, Sort: "rating"})
	q, args := featuredPeopleSQL(u.ID, lib, "series", 3, 10, Access{})
	plan("Featured people", q, args)
}
