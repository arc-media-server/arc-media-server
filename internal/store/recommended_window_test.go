package store

import (
	"ams/internal/perftest"
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestRecommendationWindowsPreserveWatchOrderAccessAndBoundaries(t *testing.T) {
	for _, kind := range []string{"movie", "episode"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			st := openTest(t)
			lib, err := st.SyncLibrary(ctx, "Allowed", "movies", "tmdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			other, err := st.SyncLibrary(ctx, "Other", "movies", "tmdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
			if err != nil {
				t.Fatal(err)
			}
			parent, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
			if err != nil {
				t.Fatal(err)
			}
			pg := "PG"
			if err = st.EditItem(ctx, parent, ItemEdit{ContentRating: &pg}); err != nil {
				t.Fatal(err)
			}
			ids := []string{}
			for i := 0; i < 8; i++ {
				lid := lib
				if i == 7 {
					lid = other
				}
				title := fmt.Sprintf("Title %d", i)
				it := &Item{LibraryID: lid, Kind: kind, Title: title}
				if kind == "episode" {
					it.ParentID = parent
				}
				if kind == "episode" && i == 5 {
					adult, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Adult show"}, "adult-show")
					if err != nil {
						t.Fatal(err)
					}
					age := "18"
					if err := st.EditItem(ctx, adult, ItemEdit{ContentRating: &age}); err != nil {
						t.Fatal(err)
					}
					it.ParentID = adult
				}
				id, _, err := st.EnsureItem(ctx, it, title)
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
				fid, err := st.SaveFile(ctx, &MediaFile{LibraryID: lid, Path: "/fixture/" + title}, 1, 1)
				if err != nil {
					t.Fatal(err)
				}
				if err = st.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
					t.Fatal(err)
				}
				age := pg
				if i == 5 {
					age = "18"
				}
				if err = st.EditItem(ctx, id, ItemEdit{ContentRating: &age}); err != nil {
					t.Fatal(err)
				}
				if err = st.SetPlayed(ctx, u.ID, []string{id}, i != 4); err != nil {
					t.Fatal(err)
				}
				if _, err = st.db.ExecContext(ctx, `UPDATE user_item_data SET last_played_at=? WHERE user_id=? AND item_id=?`, 100+i%3, u.ID, id); err != nil {
					t.Fatal(err)
				}
				if i == 6 {
					if _, err = st.db.ExecContext(ctx, `UPDATE media_files SET missing_since=1 WHERE id=?`, fid); err != nil {
						t.Fatal(err)
					}
				}
			}
			q := ItemQuery{LibraryID: lib, Kind: kind, UserID: u.ID, Played: true, Sort: "watched", Access: Access{MaxAge: 12, Libraries: []int64{lib}}}
			want := []string{ids[2], ids[1], ids[0], ids[3]}
			for _, tc := range []struct{ limit, offset int }{{1, 0}, {2, 0}, {2, 2}, {3, 0}, {4, 0}, {20, 0}, {2, 100}} {
				q.Limit, q.Offset = tc.limit, tc.offset
				got, more, err := st.ListItemsWindow(ctx, q)
				if err != nil {
					t.Fatal(err)
				}
				end := min(len(want), tc.offset+tc.limit)
				start := min(len(want), tc.offset)
				values := []string{}
				for _, it := range got {
					values = append(values, it.ID)
				}
				if !slices.Equal(values, want[start:end]) || more != (end < len(want)) {
					t.Fatalf("limit=%d offset=%d: %v more=%v", tc.limit, tc.offset, values, more)
				}
				page, total, err := st.ListItems(ctx, q)
				if err != nil || total != 4 || len(page) != len(got) {
					t.Fatalf("exact page total=%d err=%v", total, err)
				}
			}
			anonymous := q
			anonymous.UserID, anonymous.Offset, anonymous.Limit = "", 0, 20
			unfiltered, _, err := st.ListItemsWindow(ctx, anonymous)
			if err != nil || len(unfiltered) != 5 {
				t.Fatalf("without a profile, watch-state filters keep their existing semantics: %d %v", len(unfiltered), err)
			}
			query, args := itemWindowSQL(q, 21, 0)
			rows, err := st.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			plan := ""
			for rows.Next() {
				var id, parent, unused int
				var line string
				if err = rows.Scan(&id, &parent, &unused, &line); err != nil {
					t.Fatal(err)
				}
				plan += line + "\n"
			}
			if err = rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(plan, "SEARCH items USING INDEX items_library_kind") {
				t.Fatalf("fallback must use the library catalogue rather than global history:\n%s", plan)
			}
		})
	}
}

// Cross-library history must not hide older matches, cutoff ties, or NULL timestamps.
func TestWatchedWindowsFallBackWithoutChangingResults(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = perftest.Seed(ctx, tx, 100, 4, 200); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM user_item_data`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO user_item_data(user_id,item_id,played,last_played_at)
 SELECT 'perf-owner',id,1,CASE WHEN kind='movie' THEN 1 ELSE 1000+rowid END FROM items WHERE kind<>'series'`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"older-library", "boundary-ties", "null-history"} {
		if mode == "boundary-ties" {
			_, err = st.db.ExecContext(ctx, `UPDATE user_item_data SET last_played_at=2000`)
		}
		if mode == "null-history" {
			_, err = st.db.ExecContext(ctx, `UPDATE user_item_data SET last_played_at=NULL WHERE item_id LIKE 'm-%' OR item_id='e-1'`)
		}
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range []ItemQuery{
			{LibraryID: 1, Kind: "movie"}, {LibraryID: 2, Kind: "episode"},
			{LibraryID: 2, Kind: "episode", Search: "not a title"},
			{LibraryID: 2, Kind: "episode", Access: Access{Libraries: []int64{1}}},
			{LibraryID: 1, Kind: "movie", Genre: "absent"},
		} {
			q.UserID, q.Sort, q.Played = perftest.UserID, "watched", true
			for _, offset := range []int{0, 19, 99, 600, 10000} {
				q.Offset, q.Limit = offset, 20
				query, args := itemWindowSQL(q, 21, offset)
				want, err := st.queryItems(ctx, query, args...)
				if err != nil {
					t.Fatal(err)
				}
				got, err := st.listItemWindow(ctx, q, 21, offset)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s library%d offset%d: changed rows", mode, q.LibraryID, offset)
				}
				window, more, err := st.ListItemsWindow(ctx, q)
				if err != nil {
					t.Fatal(err)
				}
				if len(window) != min(20, len(want)) || more != (len(want) > 20) {
					t.Fatalf("%s: boundary changed", mode)
				}
				page, total, err := st.ListItems(ctx, q)
				if err != nil {
					t.Fatal(err)
				}
				if len(page) != len(window) || (offset == 0 && total < len(window)) {
					t.Fatalf("%s: exact page changed", mode)
				}
			}
		}
	}
	// Fast-path SQL must include an indexed timestamp range; no unbounded global-history scan.
	query, args := recentItemWindowSQL(ItemQuery{LibraryID: 1, Kind: "movie", UserID: perftest.UserID, Played: true, Sort: "watched"}, 21, 0, sql.NullInt64{Int64: 2000, Valid: true})
	rows, err := st.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan string
	for rows.Next() {
		var a, b, c int
		var line string
		if err = rows.Scan(&a, &b, &c, &line); err != nil {
			t.Fatal(err)
		}
		plan += line + "\n"
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, "last_played_at>?") {
		t.Fatalf("history lookup is not range-bounded:\n%s", plan)
	}
}
