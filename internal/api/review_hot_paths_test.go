package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// This opt-in investigation opens only a disposable fictional catalog. It has no
// latency pass threshold: cloud timings/allocations are evidence, not a production SLO.
func TestReviewHotPathMeasurements(t *testing.T) {
	if os.Getenv("ARC_RUN_REVIEW_HOT_PATHS") != "1" {
		t.Skip("opt-in review measurement")
	}
	h := reviewHotPathServer(t)
	rowResponse := fixtureAPICall(t.Context(), h, "GET", "/libraries/1/recommended", "")
	if rowResponse.Code != 200 {
		t.Fatal(rowResponse.Code, rowResponse.Body.String())
	}
	var rows recommendedResponse
	if err := json.Unmarshal(rowResponse.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	paths := []string{"/home/continue?with_up_next=1", "/playlists", "/libraries/1/collections"}
	for _, typ := range []string{"recently_added", "because_you_watched"} {
		found := false
		for _, row := range rows.Rows {
			if row.Type == typ {
				paths = append(paths, strings.TrimPrefix(row.ItemsURL, "/api/v1")+"?limit=50")
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("measurement fixture missing recommended %s row", typ)
		}
	}
	for _, path := range paths {
		response := fixtureAPICall(t.Context(), h, "GET", path, "")
		if response.Code != 200 {
			t.Fatal(path, response.Code, response.Body.String())
		}
		var milliseconds, allocations []float64
		for range 5 {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			start := time.Now()
			response = fixtureAPICall(t.Context(), h, "GET", path, "")
			elapsed := time.Since(start)
			runtime.ReadMemStats(&after)
			if response.Code != 200 {
				t.Fatal(path, response.Code, response.Body.String())
			}
			milliseconds = append(milliseconds, float64(elapsed)/float64(time.Millisecond))
			allocations = append(allocations, float64(after.TotalAlloc-before.TotalAlloc)/1024/1024)
		}
		sort.Float64s(milliseconds)
		sort.Float64s(allocations)
		t.Logf("review_hot_path path=%s median_ms=%.3f max_ms=%.3f median_alloc_mib=%.3f response_bytes=%d", path, milliseconds[2], milliseconds[4], allocations[2], response.Body.Len())
	}
	fmt.Fprintln(os.Stdout, "fixture: 4000 movies, 80 shows x 50 episodes, 20 smart + 20 manual playlists, 40 collections; 5 warm-cache samples each; no media/provider requests")
}

func reviewHotPathServer(t *testing.T) http.Handler {
	t.Helper()
	h, _ := libraryPerformanceServer(t, 4000, 80, 50, func(tx *sql.Tx) error {
		statements := []string{
			"INSERT INTO item_genres(item_id,name) SELECT id, 'Fixture Shared Genre' FROM items WHERE kind='movie'",
			"UPDATE items SET runtime_min=90+rowid%60 WHERE kind='movie'",
			"DELETE FROM user_item_data WHERE item_id IN (SELECT id FROM items WHERE kind='episode')",
			"INSERT INTO user_item_data(user_id,item_id,played,last_played_at) SELECT 'perf-owner',id,1,1700000000+rowid FROM items WHERE kind='episode' AND (CAST(substr(id,3) AS INTEGER)-1)%50=0",
			"WITH RECURSIVE n(x) AS(SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<20) INSERT INTO playlists(id,user_id,title,filter,created_at,updated_at) SELECT 'smart-'||x,'perf-owner','Smart '||x,'{\"library_id\":1,\"genre\":\"Drama\",\"sort\":\"title\"}',1,1 FROM n",
			"WITH RECURSIVE n(x) AS(SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<20) INSERT INTO playlists(id,user_id,title,created_at,updated_at) SELECT 'manual-'||x,'perf-owner','Manual '||x,1,1 FROM n",
			"INSERT INTO playlist_entries(playlist_id,item_id,position) SELECT p.id,i.id,CAST(substr(i.id,3) AS INTEGER) FROM playlists p JOIN items i ON i.kind='movie' WHERE p.id LIKE 'manual-%' AND CAST(substr(i.id,3) AS INTEGER)<=500",
			"WITH RECURSIVE n(x) AS(SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<40) INSERT INTO collections(id,library_id,kind,title,sort_title,grid_mode,created_at) SELECT 'fixture-'||x,1,'manual','Collection '||x,'collection '||x,'both',1 FROM n",
			"INSERT INTO collection_items(collection_id,item_id,position) SELECT c.id,i.id,CAST(substr(i.id,3) AS INTEGER) FROM collections c JOIN items i ON i.kind='movie' WHERE CAST(substr(i.id,3) AS INTEGER) BETWEEN (CAST(substr(c.id,9) AS INTEGER)-1)*50+1 AND CAST(substr(c.id,9) AS INTEGER)*50",
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(t.Context(), statement); err != nil {
				return err
			}
		}
		return nil
	})

	return h
}

// This generous allocation envelope detects accidentally materializing every
// full playlist entry merely to return counts and four images. It is deliberately
// opt-in with the isolated fixture and is not a latency requirement.
func TestPlaylistListingAllocationBudget(t *testing.T) {
	if os.Getenv("ARC_RUN_REVIEW_HOT_PATHS") != "1" {
		t.Skip("opt-in allocation regression")
	}
	h := reviewHotPathServer(t)
	if w := fixtureAPICall(t.Context(), h, "GET", "/playlists", ""); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	durations := map[string]int64{}
	for _, kind := range []string{"smart", "manual"} {
		detail := fixtureAPICall(t.Context(), h, "GET", "/playlists/"+kind+"-1", "")
		if detail.Code != 200 {
			t.Fatal(detail.Code, detail.Body.String())
		}
		var page playlistPage
		if err := json.Unmarshal(detail.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if page.Playlist.Count != 500 || len(page.Entries) != 500 {
			t.Fatal("full fixture lost its 500-item window")
		}
		durations[kind] = page.Playlist.DurationMS
	}
	var allocated []uint64
	for range 3 {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		response := fixtureAPICall(t.Context(), h, "GET", "/playlists", "")
		runtime.ReadMemStats(&after)
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		var lists []playlistDTO
		if err := json.Unmarshal(response.Body.Bytes(), &lists); err != nil {
			t.Fatal(err)
		}
		if len(lists) != 40 {
			t.Fatalf("fixture count=%d", len(lists))
		}
		for _, p := range lists {
			kind := "manual"
			if p.SmartFilter != nil {
				kind = "smart"
			}
			if p.Count != 500 || p.DurationMS != durations[kind] {
				t.Fatalf("summary lost contents: count=%d duration=%d", p.Count, p.DurationMS)
			}
		}
		allocated = append(allocated, after.TotalAlloc-before.TotalAlloc)
	}
	sort.Slice(allocated, func(i, j int) bool { return allocated[i] < allocated[j] })
	if allocated[1] > 32<<20 {
		t.Fatalf("playlist summaries materialize too much data: median %.3f MiB allocated for 40 summaries, budget 32 MiB", float64(allocated[1])/1024/1024)
	}
	t.Logf("playlist summary median allocation %.3f MiB", float64(allocated[1])/1024/1024)
}

func TestRecommendationPageAllocationBudget(t *testing.T) {
	if os.Getenv("ARC_RUN_REVIEW_HOT_PATHS") != "1" {
		t.Skip("opt-in allocation regression")
	}
	h := reviewHotPathServer(t)
	response := fixtureAPICall(t.Context(), h, "GET", "/libraries/1/recommended", "")
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var rows recommendedResponse
	if err := json.Unmarshal(response.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	path := ""
	for _, row := range rows.Rows {
		if row.Type == "because_you_watched" {
			path = strings.TrimPrefix(row.ItemsURL, "/api/v1")
			break
		}
	}
	if path == "" {
		t.Fatal("fixture has no curated row")
	}
	var allocated []uint64
	var total int
	for range 3 {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		response = fixtureAPICall(t.Context(), h, "GET", path+"?limit=50", "")
		runtime.ReadMemStats(&after)
		if response.Code != 200 {
			t.Fatal(response.Code, response.Body.String())
		}
		var page recommendedItemsPage
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Items) != 50 || page.Total < 50 || !page.Bounded {
			t.Fatalf("curated page changed: items=%d total=%d bounded=%v", len(page.Items), page.Total, page.Bounded)
		}
		if total != 0 && total != page.Total {
			t.Fatal("fixture selection changed")
		}
		total = page.Total
		allocated = append(allocated, after.TotalAlloc-before.TotalAlloc)
	}
	sort.Slice(allocated, func(i, j int) bool { return allocated[i] < allocated[j] })
	if allocated[1] > 16<<20 {
		t.Fatalf("one recommendation page rebuilds too much data: median %.3f MiB, budget 16 MiB", float64(allocated[1])/1024/1024)
	}
	t.Logf("curated page median allocation %.3f MiB", float64(allocated[1])/1024/1024)
}
