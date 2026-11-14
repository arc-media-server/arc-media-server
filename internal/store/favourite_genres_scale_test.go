package store

import (
	"ams/internal/perftest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestFavouriteGenreScaleMeasurements(t *testing.T) {
	if os.Getenv("ARC_RUN_GENRE_SCALE") != "1" {
		t.Skip("opt-in history aggregation measurement")
	}
	st := openTest(t)
	ctx := t.Context()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = perftest.Seed(ctx, tx, 100, 200, 500); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`DELETE FROM user_item_data`, `INSERT INTO user_item_data(user_id,item_id,played,last_played_at) SELECT 'perf-owner',id,1,100 FROM items WHERE kind='episode' AND rowid%4<>0`, `INSERT INTO item_genres SELECT id,'Shared' FROM items WHERE kind='series'`} {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err = st.db.ExecContext(ctx, "ANALYZE"); err != nil {
		t.Fatal(err)
	}
	const original = `SELECT g.name FROM user_item_data u JOIN items i ON i.id=u.item_id JOIN item_genres g ON g.item_id=COALESCE(CASE WHEN i.kind='episode' THEN i.parent_id END,i.id) WHERE u.user_id=? AND i.library_id=? AND u.played=1 GROUP BY g.name ORDER BY COUNT(*) DESC,g.name LIMIT ?`
	var want []string
	for _, variant := range []string{"previous", "weighted"} {
		for run := 0; run < 4; run++ {
			start := time.Now()
			var got []string
			if variant == "weighted" {
				got, err = st.FavouriteGenres(ctx, perftest.UserID, 2, 10)
			} else {
				rows, err := st.db.QueryContext(ctx, original, perftest.UserID, 2, 10)
				if err != nil {
					t.Fatal(err)
				}
				for rows.Next() {
					var name string
					if err = rows.Scan(&name); err != nil {
						t.Fatal(err)
					}
					got = append(got, name)
				}
				err = rows.Err()
				rows.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			if want == nil {
				want = got
			} else if !reflect.DeepEqual(want, got) {
				t.Fatalf("changed genres: %v != %v", got, want)
			}
			t.Logf("variant=%s run=%d duration_ms=%.3f", variant, run, float64(time.Since(start).Microseconds())/1000)
		}
	}
}
