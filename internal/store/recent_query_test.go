package store

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

// Retain the previous window query as a compatibility oracle for permission/tie/group semantics.
// The manual cases include hidden/latest and offline episodes, complete large groups and legacy
// missing arrival rows, so a faster result cannot pass merely by returning fewer episodes.
func previousRecentEpisodes(ctx context.Context, s *Store, library int64, acc Access, limit int) ([]Item, error) {
	q := `SELECT items.*,COALESCE((SELECT sequence FROM item_arrivals WHERE item_id=items.id),0) AS arrival_order FROM items WHERE library_id=? AND kind='episode' AND ` + playableSQL
	q, args := acc.Where("items", q, []any{library})
	q = `WITH eligible AS NOT MATERIALIZED (` + q + `), ranked AS (
 SELECT id,parent_id,added_at,arrival_order,release_date,ROW_NUMBER() OVER (PARTITION BY parent_id ORDER BY added_at DESC,arrival_order DESC,release_date DESC,id) AS position FROM eligible
 ) SELECT ` + itemCols + ` FROM eligible WHERE parent_id IN (
 SELECT parent_id FROM ranked WHERE position=1 ORDER BY added_at DESC,arrival_order DESC,release_date DESC,id LIMIT ?
 ) ORDER BY added_at DESC,arrival_order DESC,release_date DESC,id`
	return s.queryItems(ctx, q, append(args, limit)...)
}

func TestRecentEpisodeQueryPreservesGroupsPermissionsAndTies(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	library, err := st.SyncLibrary(ctx, "Recent fixture", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	for group := range 7 {
		series, _, err := st.EnsureItem(ctx, &Item{LibraryID: library, Kind: "series", Title: fmt.Sprint("Series", group)}, fmt.Sprint("s", group))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = st.db.ExecContext(ctx, "UPDATE items SET min_age=? WHERE id=?", group*3, series); err != nil {
			t.Fatal(err)
		}
		for episode := range 13 {
			key := fmt.Sprintf("e%d-%d", group, episode)
			id, _, err := st.EnsureItem(ctx, &Item{LibraryID: library, Kind: "episode", ParentID: series, Title: key}, key)
			if err != nil {
				t.Fatal(err)
			}
			if episode != 0 {
				file, err := st.SaveFile(ctx, &MediaFile{LibraryID: library, Path: "/fixture-only/" + key, Size: 1, ModTime: 1}, 1, 1)
				if err != nil {
					t.Fatal(err)
				}
				if err = st.SetFileItems(ctx, file, []string{id}, 0); err != nil {
					t.Fatal(err)
				}
				if episode == 12 {
					if _, err = st.db.ExecContext(ctx, "UPDATE media_files SET missing_since=1 WHERE id=?", file); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err = st.db.ExecContext(ctx, "UPDATE items SET added_at=?,release_date=? WHERE id=?", 1000+episode/4, fmt.Sprintf("2020-01-%02d", episode%3+1), id); err != nil {
				t.Fatal(err)
			}
			if _, err = st.db.ExecContext(ctx, "UPDATE item_arrivals SET sequence=? WHERE item_id=?", episode%2, id); err != nil {
				t.Fatal(err)
			}
			if episode == 10 {
				if _, err = st.db.ExecContext(ctx, "DELETE FROM item_arrivals WHERE item_id=?", id); err != nil {
					t.Fatal(err)
				}
			}
			if episode >= 9 && group%2 == 0 {
				if _, err = st.db.ExecContext(ctx, "INSERT INTO item_labels(item_id,label) VALUES(?,'Hidden')", id); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	for ai, acc := range []Access{{}, {Libraries: []int64{}}, {Libraries: []int64{library}}, {MaxAge: 12}, {ExcludeLabels: []string{"Hidden"}}, {MaxAge: 12, ExcludeLabels: []string{"Hidden"}}} {
		for _, limit := range []int{0, 1, 3, 24, -1} {
			want, oldErr := previousRecentEpisodes(ctx, st, library, acc, limit)
			got, err := st.RecentEpisodes(ctx, library, acc, limit)
			// PostgreSQL rejects negative LIMIT; SQLite treats it as unlimited. Preserve each
			// engine's existing outcome instead of forcing SQLite-only semantics onto PostgreSQL.
			if oldErr != nil {
				if err == nil || err.Error() != oldErr.Error() {
					t.Fatalf("error changed: old=%v new=%v", oldErr, err)
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("different full results: access=%d limit=%d got=%d want=%d", ai, limit, len(got), len(want))
			}
			if ai == 0 && limit == 3 && len(got) != 33 {
				t.Fatalf("expected three complete eleven-present-episode groups, got %d", len(got))
			}
		}
	}
}

func TestLibraryCountOnlyMatchesBrowse(t *testing.T) {
	st := smallPerformanceStore(t)
	if _, err := st.db.ExecContext(t.Context(), "UPDATE media_files SET missing_since=1 WHERE id IN (SELECT file_id FROM item_files WHERE item_id IN ('m-1','e-1'))"); err != nil {
		t.Fatal(err)
	}
	libs, err := st.ListLibraries(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	counts, err := st.LibraryCountsFor(t.Context(), libs)
	if err != nil {
		t.Fatal(err)
	}
	for _, lib := range libs {
		kind := "movie"
		if lib.Kind == "shows" {
			kind = "series"
		}
		_, want, err := st.ListItems(t.Context(), ItemQuery{LibraryID: lib.ID, Kind: kind, Limit: 1})
		if err != nil || counts[lib.ID] != want {
			t.Fatal(counts, want, err)
		}
	}
	none, err := st.LibraryCountsFor(t.Context(), nil)
	if err != nil || len(none) != 0 {
		t.Fatal(none, err)
	}
}
