package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"ams/internal/store"
)

const batchTime int64 = 1_700_000_000

type batchEpisode struct {
	season int
	ago    int64
}
type batchFixture struct {
	s      *Server
	db     *sql.DB
	user   *store.User
	lib    int64
	series string
	ids    []string
	files  []int64
}

func newBatchFixture(t *testing.T, fresh bool, episodes []batchEpisode) *batchFixture {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "batches.db")
	s, user := securityServerAt(t, path)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	f := &batchFixture{s: s, db: db, user: user}
	f.lib, err = s.store.SyncLibrary(ctx, "Batches", "shows", "tmdb_aired", []string{t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	f.series, _, err = s.store.EnsureItem(ctx, &store.Item{LibraryID: f.lib, Kind: "series", Title: "Batch show"}, "batch-show")
	if err != nil {
		t.Fatal(err)
	}
	showTime := batchTime - 30*86400
	if fresh {
		showTime = batchTime
	}
	var entries []store.OrderingEntry
	for i, e := range episodes {
		id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: f.lib, Kind: "episode", ParentID: f.series, Title: fmt.Sprint("Episode ", i)}, fmt.Sprint("episode-", i))
		if err != nil {
			t.Fatal(err)
		}
		f.ids = append(f.ids, id)
		fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: f.lib, Path: fmt.Sprintf("/isolated/batch-%d.mkv", i), Size: 1, DurationMS: 1000}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		f.files = append(f.files, fid)
		if err = s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		if e.season >= 0 {
			entries = append(entries, store.OrderingEntry{EpisodeID: id, Season: e.season, Episode: i + 1})
		}
	}
	oid, err := s.store.UpsertOrdering(ctx, f.series, "tmdb_aired", "", "Aired", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.ReplaceOrdering(ctx, oid, map[int]string{}, map[int]string{}, entries); err != nil {
		t.Fatal(err)
	}
	// First linking records genuine availability. Set historical fixture times only after
	// those writes, keeping the arrival table and item timestamps consistent.
	for i, id := range f.ids {
		for _, q := range []string{"UPDATE items SET added_at=? WHERE id=?", "UPDATE item_arrivals SET added_at=? WHERE item_id=?"} {
			if _, err := db.Exec(q, batchTime-episodes[i].ago, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, q := range []string{"UPDATE items SET added_at=? WHERE id=?", "UPDATE item_arrivals SET added_at=? WHERE item_id=?"} {
		if _, err := db.Exec(q, showTime, f.series); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f *batchFixture) cards(t *testing.T, limit int) []recentCard {
	t.Helper()
	r := httptest.NewRequest("GET", fmt.Sprintf("/api/libraries/%d/recent-shows?limit=%d", f.lib, limit), nil)
	r.SetPathValue("id", strconv.FormatInt(f.lib, 10))
	r = r.WithContext(context.WithValue(r.Context(), userKey, f.user))
	w := httptest.NewRecorder()
	f.s.recentlyAddedShows(w, r)
	if w.Code != 200 {
		t.Fatalf("status%d: %s", w.Code, w.Body.String())
	}
	var cards []recentCard
	if err := json.Unmarshal(w.Body.Bytes(), &cards); err != nil {
		t.Fatal(err)
	}
	return cards
}

func TestRecentBatchesClassifyByEpisodesAndSeasons(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		for _, tc := range []struct {
			name     string
			episodes []batchEpisode
			kind     string
			season   int
		}{
			{"single", []batchEpisode{{1, 0}}, "episode", -1},
			{"same season", []batchEpisode{{1, 0}, {1, 3600}}, "season", 1},
			{"across seasons", []batchEpisode{{1, 0}, {2, 3600}}, "series", -1},
			{"specials", []batchEpisode{{0, 0}, {0, 3600}}, "season", 0},
			{"specials and regular", []batchEpisode{{0, 0}, {1, 3600}}, "series", -1},
			{"single unplaced", []batchEpisode{{-1, 0}}, "episode", -1},
			{"known and unplaced", []batchEpisode{{1, 0}, {-1, 3600}}, "series", -1},
		} {
			t.Run(fmt.Sprintf("fresh=%v/%s", fresh, tc.name), func(t *testing.T) {
				f := newBatchFixture(t, fresh, tc.episodes)
				cards := f.cards(t, 24)
				if len(cards) != 1 || cards[0].Type != tc.kind || cards[0].Episodes != len(tc.episodes) {
					t.Fatalf("wrong batch: %+v", cards)
				}
				c := cards[0]
				if c.AddedAt != batchTime {
					t.Fatalf("arrival anchor=%d", c.AddedAt)
				}
				if tc.season >= 0 {
					if c.SeasonNumber == nil || *c.SeasonNumber != tc.season {
						t.Fatalf("wrong season: %+v", c)
					}
				} else if c.SeasonNumber != nil {
					t.Fatal("unexpected season on episode/series card")
				}
				if tc.kind == "episode" && c.Item.ID != f.ids[0] {
					t.Fatal("single card lost episode identity")
				}
			})
		}
	}
}

func TestRecentBatchesWindowAndCompleteLimits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		ages   []int64
		limit  int
		counts []int
	}{
		{"exactly24h", []int64{0, 86400}, 24, []int{2}},
		{"outside24h", []int64{0, 86401}, 24, []int{1, 1}},
		{"no chaining", []int64{0, 20 * 3600, 40 * 3600}, 24, []int{2, 1}},
		{"repeated season batches", []int64{0, 3600, 25 * 3600, 26 * 3600}, 24, []int{2, 2}},
		{"limit keeps whole batch", []int64{0, 3600, 25 * 3600, 26 * 3600}, 1, []int{2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var eps []batchEpisode
			for _, age := range tc.ages {
				eps = append(eps, batchEpisode{1, age})
			}
			f := newBatchFixture(t, false, eps)
			cards := f.cards(t, tc.limit)
			if len(cards) != len(tc.counts) {
				t.Fatalf("wrong batch count: %+v", cards)
			}
			for i, n := range tc.counts {
				if cards[i].Episodes != n {
					t.Fatalf("partial/combined batch: %+v", cards)
				}
				if i > 0 && cards[i].AddedAt >= cards[i-1].AddedAt {
					t.Fatal("batches not newest first")
				}
			}
		})
	}
}

func TestRecentBatchesIgnoreWatchStateAndExcludeUnavailableEpisodes(t *testing.T) {
	f := newBatchFixture(t, false, []batchEpisode{{1, 0}, {2, 3600}})
	before := f.cards(t, 24)
	if err := f.s.store.SetPlayed(t.Context(), f.user.ID, f.ids, true); err != nil {
		t.Fatal(err)
	}
	if err := f.s.store.UpdateMetadata(t.Context(), &store.Item{ID: f.ids[0], Title: "Refreshed title", MetadataState: "matched"}); err != nil {
		t.Fatal(err)
	}
	after := f.cards(t, 24)
	if len(before) != len(after) || before[0].Type != after[0].Type || before[0].Episodes != after[0].Episodes || before[0].AddedAt != after[0].AddedAt {
		t.Fatalf("watch/metadata changed grouping: before=%+v after=%+v", before, after)
	}
	if _, err := f.db.Exec("UPDATE media_files SET missing_since=1 WHERE id=?", f.files[1]); err != nil {
		t.Fatal(err)
	}
	cards := f.cards(t, 24)
	if len(cards) != 1 || cards[0].Type != "episode" || cards[0].Item.ID != f.ids[0] || cards[0].Episodes != 1 {
		t.Fatalf("unavailable episode influenced batch: %+v", cards)
	}
	f.user.LibraryIDs = []int64{f.lib + 1}
	if cards := f.cards(t, 24); len(cards) != 0 {
		t.Fatalf("restricted library leaked: %+v", cards)
	}
}

func TestRecentBatchesUseCurrentDisplaySeasons(t *testing.T) {
	f := newBatchFixture(t, false, []batchEpisode{{1, 0}, {2, 3600}})
	oid, err := f.s.store.UpsertOrdering(t.Context(), f.series, "custom", "fixture", "Combined season", "")
	if err != nil {
		t.Fatal(err)
	}
	entries := []store.OrderingEntry{{EpisodeID: f.ids[0], Season: 9, Episode: 1}, {EpisodeID: f.ids[1], Season: 9, Episode: 2}}
	if err = f.s.store.ReplaceOrdering(t.Context(), oid, map[int]string{}, map[int]string{}, entries); err != nil {
		t.Fatal(err)
	}
	if err = f.s.store.SetDisplayOrdering(t.Context(), f.series, "", oid); err != nil {
		t.Fatal(err)
	}
	cards := f.cards(t, 24)
	if len(cards) != 1 || cards[0].Type != "season" || cards[0].SeasonNumber == nil || *cards[0].SeasonNumber != 9 || cards[0].Episodes != 2 {
		t.Fatalf("display season ignored: %+v", cards)
	}
	if cards[0].SeasonPoster != "" {
		t.Fatal("custom display season must not borrow an aired-season poster")
	}
	if err = f.s.store.ClearDisplayOrdering(t.Context(), f.series, ""); err != nil {
		t.Fatal(err)
	}
	cards = f.cards(t, 24)
	if len(cards) != 1 || cards[0].Type != "series" || cards[0].AddedAt != batchTime {
		t.Fatalf("default display not restored: %+v", cards)
	}
}
