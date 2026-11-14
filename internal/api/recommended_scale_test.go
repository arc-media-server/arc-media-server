package api

import (
	"ams/internal/perftest"
	"ams/internal/settings"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestRecommendedScaleMeasurements exercises the complete authenticated HTTP response,
// including JSON encoding, against a broad and a long-running-series catalogue. It does not
// open media files. First means the first request to a fresh store; OS caches are not dropped.
//
// ARC_RUN_RECOMMENDED_SCALE=1 go test ./internal/api -run '^TestRecommendedScaleMeasurements$' -count=1 -v
func TestRecommendedScaleMeasurements(t *testing.T) {
	if os.Getenv("ARC_RUN_RECOMMENDED_SCALE") != "1" {
		t.Skip("opt-in 100,000-episode Recommended measurements")
	}
	for _, shape := range []struct {
		name            string
		shows, episodes int
	}{{"broad", 1000, 100}, {"long-shows", 200, 500}} {
		t.Run(shape.name, func(t *testing.T) {
			started := time.Now()
			_, st := libraryPerformanceServer(t, 10000, shape.shows, shape.episodes, func(tx *sql.Tx) error {
				// All top-level titles have real metadata, a shared cast/director pool and multiple genres.
				statements := []string{
					`UPDATE items SET overview=?`,
					`UPDATE items SET rating=5+(rowid%50)/10.0,release_date=printf('%d-01-01',2000+rowid%25),metadata_state='matched',content_rating='PG',min_age=10 WHERE kind<>'episode'`,
					`INSERT INTO item_genres(item_id,name) SELECT id,'Comedy' FROM items WHERE kind<>'episode' AND rowid%3=0`,
					`INSERT INTO item_genres(item_id,name) SELECT id,'Adventure' FROM items WHERE kind<>'episode' AND rowid%7=0`,
					`WITH RECURSIVE p(n) AS (SELECT 0 UNION ALL SELECT n+1 FROM p WHERE n<459) INSERT INTO people(id,tmdb_id,name) SELECT 1000+n,1000+n,'Person '||n FROM p`,
					`WITH RECURSIVE c(n) AS (SELECT 0 UNION ALL SELECT n+1 FROM c WHERE n<7) INSERT INTO item_people(item_id,person_id,role,ord) SELECT items.id,1000+(items.rowid*7+c.n*13)%400,'actor',c.n FROM items CROSS JOIN c WHERE items.kind<>'episode'`,
					`INSERT INTO item_people(item_id,person_id,role,ord) SELECT id,1400+rowid%60,'director',0 FROM items WHERE kind<>'episode'`,
					`DELETE FROM user_item_data`,
				}
				for i, q := range statements {
					var err error
					if i == 0 {
						_, err = tx.ExecContext(t.Context(), q, strings.Repeat("Synthetic programme description. ", 12))
					} else {
						_, err = tx.ExecContext(t.Context(), q)
					}
					if err != nil {
						return fmt.Errorf("scale statement %d: %w", i, err)
					}
				}
				// One in five shows is unstarted. For the others, three quarters of the episodes are
				// watched and the next is in progress. Spread dates across 400 days to exercise Rediscover.
				_, err := tx.ExecContext(t.Context(), `INSERT INTO user_item_data(user_id,item_id,played,position_ms,last_played_at)
      SELECT ?,id,CASE WHEN kind='movie' OR (CAST(substr(id,3) AS INTEGER)-1)%? < ? THEN 1 ELSE 0 END,
       CASE WHEN kind='episode' AND (CAST(substr(id,3) AS INTEGER)-1)%? = ? THEN 120000 ELSE 0 END,
       ?-(rowid%400)*86400 FROM items
      WHERE (kind='movie' AND CAST(substr(id,3) AS INTEGER)%5<>0)
       OR (kind='episode' AND CAST(substr(parent_id,3) AS INTEGER)%5<>0 AND (CAST(substr(id,3) AS INTEGER)-1)%? <= ?)`,
					perftest.UserID, shape.episodes, shape.episodes*3/4, shape.episodes, shape.episodes*3/4, time.Now().Unix(), shape.episodes, shape.episodes*3/4)
				return err
			})
			t.Logf("seeded movies=10000 shows=%d episodes=%d in %s", shape.shows, shape.shows*shape.episodes, time.Since(started).Round(time.Millisecond))
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			h := New(Deps{Store: st, Log: logger, Settings: func() settings.Settings { return settings.Settings{} }})
			for _, lib := range []int{2, 1} {
				var warm []float64
				var first float64
				var rowCount, bytes int
				for run := 0; run < 6; run++ {
					ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
					started := time.Now()
					w := fixtureAPICall(ctx, h, "GET", fmt.Sprintf("/libraries/%d/recommended", lib), "")
					took := float64(time.Since(started).Microseconds()) / 1000
					cancel()
					if w.Code != 200 {
						t.Fatalf("HTTP %d: %s", w.Code, w.Body.String())
					}
					var page recommendedResponse
					if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
						t.Fatal(err)
					}
					rowCount, bytes = len(page.Rows), w.Body.Len()
					if rowCount < 10 {
						t.Fatalf("fixture did not exercise enough row families: %d", rowCount)
					}
					if run == 0 {
						first = took
					} else {
						warm = append(warm, took)
					}
				}
				sort.Float64s(warm)
				summary, _ := json.Marshal(map[string]any{"shape": shape.name, "library": lib, "first_request_ms": first, "warm_median_ms": warm[len(warm)/2], "warm_samples_ms": warm, "rows": rowCount, "bytes": bytes})
				t.Log(string(summary))
			}
		})
	}
}
