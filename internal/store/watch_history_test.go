package store

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"ams/internal/perftest"
)

// The rows Recommended builds from a profile's watch history read it once and share it. They
// must give what each row's own query over the whole history gave: these are those queries,
// checked against the shared version on a library with ties, part-watched and undated plays,
// missing files, age limits and a second profile.
func TestWatchHistoryRowsMatchTheirOwnQueries(t *testing.T) {
	st := openTest(t)
	ctx := t.Context()
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err = perftest.Seed(ctx, tx, 80, 14, 12); err != nil {
		t.Fatal(err)
	}
	rnd := rand.New(rand.NewPCG(7, 11))
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`DELETE FROM user_item_data`)
	exec(`INSERT INTO users(id,name,is_admin,created_at) VALUES('other','Other',0,1)`)
	for p := range 25 {
		exec(`INSERT INTO people(id,tmdb_id,name) VALUES(?,?,?)`, 100+p, 100+p, fmt.Sprintf("Person %02d", p))
	}
	var titles, episodes []string
	for m := 1; m <= 80; m++ {
		titles = append(titles, fmt.Sprintf("m-%d", m))
	}
	for s := 1; s <= 14; s++ {
		titles = append(titles, fmt.Sprintf("s-%d", s))
	}
	for e := 1; e <= 14*12; e++ {
		episodes = append(episodes, fmt.Sprintf("e-%d", e))
	}
	for _, id := range titles {
		exec(`UPDATE items SET min_age=? WHERE id=?`, []int{0, 0, 12, 18}[rnd.IntN(4)], id)
		for _, p := range rnd.Perm(25)[:2+rnd.IntN(6)] {
			role := "actor"
			if rnd.IntN(5) == 0 {
				role = "director"
			}
			exec(`INSERT OR IGNORE INTO item_people(item_id,person_id,role,ord) VALUES(?,?,?,?)`, id, 100+p, role, rnd.IntN(13))
		}
		if rnd.IntN(3) == 0 {
			exec(`INSERT OR IGNORE INTO item_genres(item_id,name) VALUES(?,?)`, id, []string{"Comedy", "Crime", "Drama"}[rnd.IntN(3)])
		}
	}
	// Some titles' files are gone (an offline drive), so they don't count for people.
	exec(`UPDATE media_files SET missing_since=1 WHERE id IN (SELECT file_id FROM item_files WHERE item_id IN ('m-3','m-7','m-11') OR item_id IN (SELECT id FROM items WHERE parent_id='s-4'))`)
	// Plays a few days apart, so several titles share a time and the walk must not stop inside a tie.
	for _, user := range []string{perftest.UserID, "other"} {
		for _, id := range append(slices.Clone(titles[:80]), episodes...) {
			if rnd.IntN(3) != 0 {
				continue
			}
			var at any = 1700000000 + rnd.IntN(40)*86400
			if rnd.IntN(15) == 0 {
				at = nil
			}
			played, pos := 1, 0
			if rnd.IntN(6) == 0 {
				played, pos = 0, 60000
			}
			exec(`INSERT INTO user_item_data(user_id,item_id,played,position_ms,last_played_at) VALUES(?,?,?,?,?)`, user, id, played, pos, at)
		}
		// A show's own row (from an older import) counts as watching it, but not for its cast.
		exec(`INSERT INTO user_item_data(user_id,item_id,played,position_ms,last_played_at) VALUES(?,'s-2',1,0,1700000000+90*86400)`, user)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}

	type person struct {
		ID         int64
		Name, Role string
		Titles     int
		At         int64
	}
	oldPeople := func(user string, lib int64, kind string, minTitles, limit int, acc Access) []person {
		titles, args := acc.Where("items", `SELECT items.id, COALESCE(CASE WHEN items.kind = 'series' THEN (
				SELECT MAX(u.last_played_at) FROM items w JOIN user_item_data u ON u.item_id = w.id AND u.user_id = ?
				WHERE w.parent_id = items.id AND u.last_played_at IS NOT NULL AND (u.played = 1 OR u.position_ms > 0))
			ELSE (SELECT u.last_played_at FROM user_item_data u WHERE u.user_id = ? AND u.item_id = items.id
				AND u.last_played_at IS NOT NULL AND (u.played = 1 OR u.position_ms > 0)) END, 0) AS at
			FROM items WHERE items.library_id = ? AND items.kind = ? AND `+playableSQL, []any{user, user, lib, kind})
		rows, err := st.db.QueryContext(ctx, `WITH titles AS MATERIALIZED (`+titles+`)
			SELECT p.person_id, pe.name, p.role, COUNT(*), MAX(titles.at) FROM titles
			JOIN item_people p ON p.item_id = titles.id JOIN people pe ON pe.id = p.person_id
			WHERE (p.role = 'director' OR (p.role = 'actor' AND p.ord < 10))
			GROUP BY p.person_id, pe.name, p.role HAVING COUNT(*) >= ?
			ORDER BY 5 DESC, 4 DESC, pe.name, p.role LIMIT ?`, append(args, minTitles, limit)...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []person
		for rows.Next() {
			var p person
			if err := rows.Scan(&p.ID, &p.Name, &p.Role, &p.Titles, &p.At); err != nil {
				t.Fatal(err)
			}
			out = append(out, p)
		}
		return out
	}
	oldRecent := func(user string, lib int64, limit int) []string {
		rows, err := st.db.QueryContext(ctx, `SELECT COALESCE(CASE WHEN i.kind = 'episode' THEN i.parent_id END, i.id), MAX(u.last_played_at)
			FROM user_item_data u JOIN items i ON i.id = u.item_id
			WHERE u.user_id = ? AND i.library_id = ? AND (u.played = 1 OR u.position_ms > 0) AND u.last_played_at IS NOT NULL
			GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT ?`, user, lib, limit)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id string
			var at int64
			if err := rows.Scan(&id, &at); err != nil {
				t.Fatal(err)
			}
			out = append(out, fmt.Sprint(at))
		}
		return out
	}

	for _, batch := range []int{1, 3, 50} {
		featuredPeopleBatch = batch
		for _, user := range []string{perftest.UserID, "other", "nobody"} {
			for _, lib := range []struct {
				id   int64
				kind string
			}{{1, "movie"}, {2, "series"}} {
				h, err := st.LibraryWatchHistory(ctx, user, lib.id)
				if err != nil {
					t.Fatal(err)
				}
				for _, acc := range []Access{{}, {MaxAge: 12}} {
					for _, minTitles := range []int{1, 2, 3} {
						for _, limit := range []int{1, 3, 10, 100} {
							got, err := st.FeaturedPeopleFrom(ctx, h, lib.id, lib.kind, minTitles, acc, limit)
							if err != nil {
								t.Fatal(err)
							}
							var mine []person
							for _, p := range got {
								mine = append(mine, person{p.ID, p.Name, p.Role, p.Titles, p.LastWatchedAt})
							}
							if want := oldPeople(user, lib.id, lib.kind, minTitles, limit, acc); !reflect.DeepEqual(mine, want) {
								t.Fatalf("featured people, batch %d %s library %d %+v min %d limit %d:\n got %v\nwant %v", batch, user, lib.id, acc, minTitles, limit, mine, want)
							}
						}
					}
				}
				for _, limit := range []int{1, 3, 10} {
					items, err := st.RecentlyWatchedFrom(ctx, h, limit)
					if err != nil {
						t.Fatal(err)
					}
					// The old query's order and its pick among titles tied at the cut-off were up to
					// the database; the shared one is newest first. Compare by play time.
					var times []string
					for i, it := range items {
						at := h.Titles[h.byID[it.ID]].LastPlayedAt
						if i > 0 && at > h.Titles[h.byID[items[i-1].ID]].LastPlayedAt {
							t.Fatalf("recently watched not newest first: %v", items)
						}
						times = append(times, fmt.Sprint(at))
					}
					if want := oldRecent(user, lib.id, limit); !slices.Equal(times, want) {
						t.Fatalf("recently watched %s library %d limit %d: %v, want %v", user, lib.id, limit, times, want)
					}
				}
			}
		}
	}
	featuredPeopleBatch = 50
}
