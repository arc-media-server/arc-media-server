package store

import (
	"fmt"
	"testing"
)

func TestImportedDuplicatePrefersPresentThenNewest(t *testing.T) {
	for _, kind := range []string{"movie", "series", "episode"} {
		t.Run(kind, func(t *testing.T) {
			s := openTest(t)
			ctx := t.Context()
			lib, err := s.SyncLibrary(ctx, "Duplicates", "shows", "tvdb_aired", []string{"/fixture"})
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, 3)
			for i := range ids {
				key := fmt.Sprintf("duplicate:%d", i)
				if kind == "episode" {
					key = fmt.Sprintf("ep:show%d:tvdb:777", i)
				}
				ids[i], _, err = s.EnsureItem(ctx, &Item{LibraryID: lib, Kind: kind, Title: "Fixture"}, key)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.db.ExecContext(ctx, `UPDATE items SET tmdb_id=777,tvdb_id=777,imdb_id='tt777',added_at=? WHERE id=?`, 100+i, ids[i]); err != nil {
					t.Fatal(err)
				}
				target := ids[i]
				if kind == "series" {
					target, _, err = s.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: ids[i], Title: "Episode"}, fmt.Sprintf("child:%d", i))
					if err != nil {
						t.Fatal(err)
					}
				}
				file, err := s.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: fmt.Sprintf("/fixture/%d.mkv", i), Size: 1}, 1, 1)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.db.ExecContext(ctx, `INSERT INTO item_files(item_id,file_id) VALUES (?,?)`, target, file); err != nil {
					t.Fatal(err)
				}
				if i != 1 {
					if _, err = s.db.ExecContext(ctx, `UPDATE media_files SET missing_since=1 WHERE id=?`, file); err != nil {
						t.Fatal(err)
					}
				}
			}
			refs := []ImportRef{{Kind: kind, TMDB: 777}, {Kind: kind, IMDB: "tt777"}}
			if kind != "movie" {
				refs = append(refs, ImportRef{Kind: kind, TVDB: 777})
			}
			if kind == "episode" {
				refs = []ImportRef{{Kind: kind, TVDB: 777}}
			}
			for _, ref := range refs {
				got, err := s.FindImported(ctx, ref)
				if err != nil || got != ids[1] {
					t.Errorf("present duplicate: got %q want %q (%v)", got, ids[1], err)
				}
			}
			if _, err = s.db.ExecContext(ctx, `UPDATE media_files SET missing_since=NULL WHERE path='/fixture/2.mkv'`); err != nil {
				t.Fatal(err)
			}
			got, err := s.FindImported(ctx, refs[0])
			if err != nil || got != ids[2] {
				t.Errorf("newest present duplicate: got %q want %q (%v)", got, ids[2], err)
			}
			if _, err = s.db.ExecContext(ctx, `UPDATE media_files SET missing_since=1`); err != nil {
				t.Fatal(err)
			}
			got, err = s.FindImported(ctx, refs[0])
			if err != nil || got != ids[2] {
				t.Errorf("newest unavailable fallback: got %q want %q (%v)", got, ids[2], err)
			}
		})
	}
}

func TestImportedEpisodeByNumberPrefersPresentTargetAcrossShows(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	lib, err := s.SyncLibrary(ctx, "Shows", "shows", "tvdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	var wanted string
	for i := 0; i < 2; i++ {
		show, _, err := s.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Duplicate show"}, fmt.Sprintf("show:%d", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.db.ExecContext(ctx, `UPDATE items SET tvdb_id=888,added_at=? WHERE id=?`, 200-i, show); err != nil {
			t.Fatal(err)
		}
		episode, _, err := s.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: show, Title: "Target"}, fmt.Sprintf("target:%d", i))
		if err != nil {
			t.Fatal(err)
		}
		order, err := s.UpsertOrdering(ctx, show, "tvdb_aired", "", "Aired", "")
		if err != nil {
			t.Fatal(err)
		}
		if err = s.ReplaceOrdering(ctx, order, nil, nil, []OrderingEntry{{EpisodeID: episode, Season: 1, Episode: 2}}); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			file, err := s.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/fixture/target.mkv", Size: 1}, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			if err = s.SetFileItems(ctx, file, []string{episode}, 0); err != nil {
				t.Fatal(err)
			}
			wanted = episode
		}
	}
	got, err := s.FindImported(ctx, ImportRef{Kind: "episode", SeriesTVDB: 888, Season: 1, Episode: 2})
	if err != nil || got != wanted {
		t.Fatalf("target episode got %q want %q (%v)", got, wanted, err)
	}
}
