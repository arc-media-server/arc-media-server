package store

import (
	"ams/internal/perftest"
	"reflect"
	"testing"
)

func TestFavouriteGenresKeepEpisodeWeightsAndProfileScope(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = perftest.Seed(ctx, tx, 20, 4, 20); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`DELETE FROM user_item_data`,
		`INSERT INTO item_genres SELECT id,'Shared' FROM items WHERE kind='series'`,
		`INSERT INTO user_item_data(user_id,item_id,played,position_ms,last_played_at) SELECT 'perf-owner',id,1,0,100 FROM items WHERE id IN ('m-1','s-1') OR parent_id='s-2' OR (parent_id='s-1' AND rowid%3=0)`,
		`INSERT INTO user_item_data VALUES('perf-owner','e-80',100,0,0,100,0,NULL)`,
	} {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	const original = `SELECT g.name FROM user_item_data u JOIN items i ON i.id=u.item_id JOIN item_genres g ON g.item_id=COALESCE(CASE WHEN i.kind='episode' THEN i.parent_id END,i.id) WHERE u.user_id=? AND i.library_id=? AND u.played=1 GROUP BY g.name ORDER BY COUNT(*) DESC,g.name LIMIT ?`
	for _, lib := range []int64{1, 2, 99} {
		for _, user := range []string{perftest.UserID, "other"} {
			for _, limit := range []int{1, 2, 20} {
				rows, err := st.db.QueryContext(ctx, original, user, lib, limit)
				if err != nil {
					t.Fatal(err)
				}
				var want []string
				for rows.Next() {
					var g string
					if err = rows.Scan(&g); err != nil {
						t.Fatal(err)
					}
					want = append(want, g)
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					t.Fatal(err)
				}
				got, err := st.FavouriteGenres(ctx, user, lib, limit)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("library%d user%s limit%d: %v != %v", lib, user, limit, got, want)
				}
			}
		}
	}
}
