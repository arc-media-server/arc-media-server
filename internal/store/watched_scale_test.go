package store

import (
	"ams/internal/perftest"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Investigation only: a small library whose watched items are behind another library's history.
func TestWatchedWindowScaleMeasurements(t *testing.T) {
	if os.Getenv("ARC_RUN_WATCHED_SCALE") != "1" {
		t.Skip("opt-in mixed-library watched measurements")
	}
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "sparse.db")
	st, err := Open(ctx, path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	seed, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer seed.Close()
	tx, err := seed.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = perftest.Seed(ctx, tx, 100, 200, 500); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`DELETE FROM user_item_data`, `INSERT INTO user_item_data(user_id,item_id,played,last_played_at) SELECT 'perf-owner',id,1,CASE WHEN kind='movie' THEN 1 ELSE 1000+rowid END FROM items WHERE kind<>'series'`} {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = seed.ExecContext(ctx, "ANALYZE"); err != nil {
		t.Fatal(err)
	}
	for _, lib := range []int64{1, 2} {
		kind := "movie"
		if lib == 2 {
			kind = "episode"
		}
		q := ItemQuery{LibraryID: lib, Kind: kind, Played: true, Sort: "watched", UserID: perftest.UserID, Limit: 20}
		cond, args := itemFilter(q)
		order, orderArgs := itemOrdering(q)
		oldSQL := `SELECT ` + itemCols + ` FROM items WHERE ` + cond + ` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
		oldArgs := append(append(append([]any{}, args...), orderArgs...), 21, 0)
		historySQL, historyArgs := recentItemWindowSQL(q, 21, 0, sql.NullInt64{})
		var expected []Item
		for _, tc := range []struct {
			name, q string
			args    []any
		}{{"catalogue", oldSQL, oldArgs}, {"previous-history", historySQL, historyArgs}} {
			rows, err := st.db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+tc.q, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			var plan []string
			for rows.Next() {
				var a, b, c int
				var line string
				if err = rows.Scan(&a, &b, &c, &line); err != nil {
					t.Fatal(err)
				}
				plan = append(plan, line)
			}
			rows.Close()
			t.Logf("PLAN library=%d variant=%s\n%s", lib, tc.name, strings.Join(plan, "\n"))
			for run := 0; run < 4; run++ {
				start := time.Now()
				got, err := st.queryItems(ctx, tc.q, tc.args...)
				elapsed := time.Since(start)
				if err != nil {
					t.Fatal(err)
				}
				if expected == nil {
					expected = got
				} else if !reflect.DeepEqual(expected, got) {
					t.Fatal("rows changed")
				}
				t.Logf("MEASURE library=%d variant=%s run=%d duration_ms=%.3f rows=%d", lib, tc.name, run, float64(elapsed.Microseconds())/1000, len(got))
			}
		}
		for run := 0; run < 4; run++ {
			start := time.Now()
			got, err := st.listItemWindow(ctx, q, 21, 0)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(expected, got) {
				t.Fatal("adaptive rows changed")
			}
			t.Logf("ADAPTIVE library=%d run=%d duration_ms=%.3f rows=%d", lib, run, float64(time.Since(start).Microseconds())/1000, len(got))
		}
	}
}
