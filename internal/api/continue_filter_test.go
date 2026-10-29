package api

import (
	"context"
	"database/sql"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"ams/internal/store"
)

func TestContinueWatchingFiltersBeforeLimiting(t *testing.T) {
	for _, kind := range []string{"movie", "episode"} {
		for _, reason := range []string{"dismissed", "library access", "age limit", "hidden label"} {
			t.Run(kind+"/"+reason, func(t *testing.T) {
				ctx := context.Background()
				path := filepath.Join(t.TempDir(), "ams.db")
				st, err := store.Open(ctx, path, store.Options{})
				if err != nil {
					t.Fatal(err)
				}
				defer st.Close()
				lib, err := st.SyncLibrary(ctx, "Allowed", "movies", "tmdb_aired", []string{"/allowed"})
				if err != nil {
					t.Fatal(err)
				}
				blocked, err := st.SyncLibrary(ctx, "Blocked", "movies", "tmdb_aired", []string{"/blocked"})
				if err != nil {
					t.Fatal(err)
				}
				u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
				if err != nil {
					t.Fatal(err)
				}
				switch reason {
				case "library access":
					u.LibraryIDs = []int64{lib}
				case "age limit":
					u.MaxAge = 12
				case "hidden label":
					u.ExcludeLabels = []string{"hidden"}
				}
				// An auxiliary connection controls fixture timestamps without sleeps.
				db, err := sql.Open("sqlite", path)
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				var wanted string
				parents := map[bool]string{}
				if kind == "episode" {
					for _, hidden := range []bool{false, true} {
						library := lib
						if hidden && reason == "library access" {
							library = blocked
						}
						id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: library, Kind: "series", Title: "Fixture show"}, fmt.Sprintf("show:%v", hidden))
						if err != nil {
							t.Fatal(err)
						}
						parents[hidden] = id
					}
				}
				for i := range 31 {
					library := lib
					if i > 0 && reason == "library access" {
						library = blocked
					}
					id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: library, Kind: kind, ParentID: parents[i > 0], Title: fmt.Sprintf("Fixture %d", i)}, fmt.Sprintf("fixture:%d", i))
					if err != nil {
						t.Fatal(err)
					}
					file, err := st.SaveFile(ctx, &store.MediaFile{LibraryID: library, Path: fmt.Sprintf("/fixture-%d.mkv", i)}, 1, 1)
					if err != nil {
						t.Fatal(err)
					}
					if err := st.SetFileItems(ctx, file, []string{id}, 0); err != nil {
						t.Fatal(err)
					}
					if _, err := st.ReportProgress(ctx, u.ID, id, 120000, 7200000); err != nil {
						t.Fatal(err)
					}
					at := 200
					if i == 0 {
						wanted = id
						at = 100
					}
					if _, err := db.Exec(`UPDATE user_item_data SET last_played_at=? WHERE user_id=? AND item_id=?`, at, u.ID, id); err != nil {
						t.Fatal(err)
					}
					rating := "PG"
					controlledID := id
					if kind == "episode" {
						controlledID = parents[i > 0]
					}
					if i > 0 && reason == "age limit" {
						rating = "18"
					}
					if err := st.EditItem(ctx, controlledID, store.ItemEdit{ContentRating: &rating}); err != nil {
						t.Fatal(err)
					}
					if i > 0 {
						switch reason {
						case "dismissed":
							if err := st.DismissContinue(ctx, u.ID, []string{controlledID}); err != nil {
								t.Fatal(err)
							}
						case "hidden label":
							labels := []string{"hidden"}
							if err := st.EditItem(ctx, controlledID, store.ItemEdit{Labels: &labels}); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
				r := httptest.NewRequest("GET", "/api/home/continue", nil)
				r = r.WithContext(context.WithValue(r.Context(), userKey, u))
				s := &Server{store: st}
				items, err := s.inProgress(r, access(r))
				if err != nil || len(items) != 1 || items[0].item.ID != wanted {
					t.Fatalf("older visible resume should survive 30 filtered titles: got %v err=%v", items, err)
				}
			})
		}
	}
}
