package api

import (
	"ams/internal/store"
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestUpNextOrderedCandidateWindows(t *testing.T) {
	for _, mode := range []string{"ordinary", "missing-and-watched", "in-progress", "custom-order", "special", "dismissed", "unmapped"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "test.db")
			s, u := securityServerAt(t, path)
			ctx := t.Context()
			lib, err := s.store.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
			if err != nil {
				t.Fatal(err)
			}
			sid, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
			if err != nil {
				t.Fatal(err)
			}
			oid, err := s.store.UpsertOrdering(ctx, sid, "tmdb_aired", "", "Aired", "")
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			var entries []store.OrderingEntry
			for i := 0; i < 150; i++ {
				id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "episode", ParentID: sid, Title: fmt.Sprint(i)}, fmt.Sprint(i))
				if err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
				entries = append(entries, store.OrderingEntry{EpisodeID: id, Season: 1, Episode: i + 1})
				if mode != "missing-and-watched" || i == 0 || i >= 140 {
					fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: fmt.Sprintf("/fixture/%d", i)}, 1, 1)
					if err != nil {
						t.Fatal(err)
					}
					if err = s.store.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
						t.Fatal(err)
					}
				}
			}
			want := ids[1]
			if err = s.store.SetPlayed(ctx, u.ID, []string{ids[0]}, true); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "missing-and-watched":
				if err = s.store.SetPlayed(ctx, u.ID, []string{ids[140]}, true); err != nil {
					t.Fatal(err)
				}
				// Use the fixture DB to keep episode zero the most recently watched, even though
				// another later successor has already been watched. No real store is involved.
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				if _, err = db.Exec(`UPDATE user_item_data SET last_played_at=CASE WHEN item_id=? THEN 200 ELSE 100 END`, ids[0]); err != nil {
					t.Fatal(err)
				}
				want = ids[141]
			case "in-progress":
				if _, err = s.store.ReportProgress(ctx, u.ID, ids[1], 120000, 3600000); err != nil {
					t.Fatal(err)
				}
				want = ""
			case "custom-order":
				oid, err = s.store.UpsertOrdering(ctx, sid, "custom", "", "Custom", "")
				if err != nil {
					t.Fatal(err)
				}
				entries[1], entries[55] = entries[55], entries[1]
				entries[1].Episode = 2
				entries[55].Episode = 56
				if err = s.store.SetDisplayOrdering(ctx, sid, "", oid); err != nil {
					t.Fatal(err)
				}
				want = ids[55]
			case "special":
				entries[1].Season, entries[1].Episode = 0, 1
				on := true
				if err = s.store.SetInlineSpecials(ctx, sid, &on); err != nil {
					t.Fatal(err)
				}
				if err = s.store.ReplaceSpecialPlacements(ctx, sid, map[string]string{ids[1]: ids[2]}); err != nil {
					t.Fatal(err)
				}
			case "dismissed":
				if err = s.store.DismissContinue(ctx, u.ID, []string{sid}); err != nil {
					t.Fatal(err)
				}
				want = ""
			case "unmapped":
				entries = entries[1:]
				want = ""
			}
			if err = s.store.ReplaceOrdering(ctx, oid, nil, nil, entries); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/", nil)
			r = r.WithContext(context.WithValue(ctx, userKey, u))
			contexts := s.episodeContexts(ctx)
			got, err := s.upNextWithContexts(r, store.Access{}, contexts)
			if err != nil {
				t.Fatal(err)
			}
			if want == "" {
				if len(got) != 0 {
					t.Fatal("unexpected successor", got)
				}
				return
			}
			if len(got) != 1 || got[0].item.ID != want {
				t.Fatalf("want %s, got %+v", want, got)
			}
			if mode == "ordinary" || mode == "custom-order" {
				if err := s.store.SetSeasonPosters(ctx, sid, map[int]string{1: "/season.jpg"}); err != nil {
					t.Fatal(err)
				}
				out, err := s.homeDTOsWithContexts(r, got, contexts)
				if err != nil {
					t.Fatal(err)
				}
				card := out[0]
				if card.EpisodeNumber == nil || *card.EpisodeNumber != 2 {
					t.Fatalf("missing episode number: %+v", card)
				}
				if poster := card.Images["season_primary"]; (poster != "") != (mode == "ordinary") {
					t.Fatalf("unexpected season poster %q", poster)
				}
			}
		})
	}
}
