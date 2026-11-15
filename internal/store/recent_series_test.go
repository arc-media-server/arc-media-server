package store

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"ams/internal/perftest"
)

// Continue Watching finds the shows a profile watched most recently by reading its history
// newest first and stopping once it has enough. It must find what grouping the whole history
// did, and the database must read that history through its recent-history index, not sort it.
func TestRecentlyWatchedSeriesReadsNewestFirst(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = perftest.Seed(ctx, tx, 10, 30, 20); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`DELETE FROM user_item_data`)
	exec(`INSERT INTO users(id,name,is_admin,created_at) VALUES('other','Other',0,1)`)
	exec(`UPDATE items SET min_age=18 WHERE id IN ('s-3','s-9')`)
	rnd := rand.New(rand.NewPCG(3, 5))
	at := 1700000000
	for e := 1; e <= 30*20; e++ {
		if rnd.IntN(3) == 0 {
			continue
		}
		at += 1 + rnd.IntN(100) // distinct times: ties were never ordered
		played, pos := 1, 0
		if rnd.IntN(5) == 0 {
			played, pos = 0, 5000 // part-watched doesn't count
		}
		exec(`INSERT INTO user_item_data(user_id,item_id,played,position_ms,last_played_at) VALUES(?,?,?,?,?)`, perftest.UserID, fmt.Sprintf("e-%d", e), played, pos, at)
		if rnd.IntN(4) == 0 {
			exec(`INSERT INTO user_item_data(user_id,item_id,played,position_ms,last_played_at) VALUES('other',?,1,0,?)`, fmt.Sprintf("e-%d", e), at+7)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	old := func(user string, limit int, acc Access) []WatchedSeries {
		q, args := acc.Where("sr", `
			SELECT i.parent_id, i.id, MAX(u.last_played_at) FROM user_item_data u JOIN items i ON i.id = u.item_id
			JOIN items sr ON sr.id = i.parent_id
			WHERE u.user_id = ? AND u.played = 1 AND i.kind = 'episode' AND u.last_played_at IS NOT NULL`, []any{user})
		rows, err := st.db.QueryContext(ctx, q+` GROUP BY i.parent_id ORDER BY 3 DESC LIMIT ?`, append(args, limit)...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []WatchedSeries
		for rows.Next() {
			var w WatchedSeries
			if err := rows.Scan(&w.SeriesID, &w.LastEpisodeID, &w.LastPlayedAt); err != nil {
				t.Fatal(err)
			}
			out = append(out, w)
		}
		return out
	}
	for _, user := range []string{perftest.UserID, "other", "nobody"} {
		for _, acc := range []Access{{}, {MaxAge: 12}, {Libraries: []int64{1}}} {
			for _, limit := range []int{1, 5, 40} {
				got, err := st.RecentlyWatchedSeries(ctx, user, limit, acc)
				if err != nil {
					t.Fatal(err)
				}
				if want := old(user, limit, acc); !reflect.DeepEqual(got, want) {
					t.Fatalf("%s %+v limit %d:\n got %v\nwant %v", user, acc, limit, got, want)
				}
			}
		}
	}

	rows, err := st.db.QueryContext(ctx, `EXPLAIN QUERY PLAN SELECT i.parent_id, i.id, u.last_played_at FROM user_item_data u CROSS JOIN items i ON i.id = u.item_id
		JOIN items sr ON sr.id = i.parent_id
		WHERE u.user_id = ? AND u.played = 1 AND i.kind = 'episode' AND u.last_played_at IS NOT NULL ORDER BY u.last_played_at DESC`, perftest.UserID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	joined := strings.Join(plan, "\n")
	if !strings.Contains(joined, "user_item_data_recent") || strings.Contains(joined, "TEMP B-TREE") {
		t.Errorf("recently watched shows should read the recent-history index in order:\n%s", joined)
	}
}
