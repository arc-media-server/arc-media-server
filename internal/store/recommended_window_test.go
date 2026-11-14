package store

import (
	"context"
	"fmt"
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
			if !strings.Contains(plan, "SEARCH watched USING INDEX user_item_data_recent") {
				t.Fatalf("recent history must drive listing:\n%s", plan)
			}
		})
	}
}
