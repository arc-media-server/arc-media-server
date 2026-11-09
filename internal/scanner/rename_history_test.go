package scanner

import (
	"ams/internal/store"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRenamingMovieOrSeriesKeepsIdentityAndWatchState(t *testing.T) {
	for _, kind := range []string{"movies", "shows"} {
		t.Run(kind, func(t *testing.T) {
			oldName, newName := "Alien (1979).mkv", "Alien (1979) {tmdb-348}.mkv"
			if kind == "shows" {
				oldName = "Firefly/Firefly S01E01.mkv"
				newName = "Firefly (2002)/Firefly S01E01.mkv"
			}
			st, s, lib, root := libraryFixture(t, kind, map[string]string{oldName: "generated rename fixture"})
			ctx := context.Background()
			if err := s.scan(ctx, lib); err != nil {
				t.Fatal(err)
			}
			mediaKind := "movie"
			if kind == "shows" {
				mediaKind = "series"
			}
			items, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: mediaKind})
			if err != nil || len(items) != 1 {
				t.Fatalf("items=%v err=%v", items, err)
			}
			topID := items[0].ID
			watchedID := topID
			if kind == "shows" {
				kids, err := st.Children(ctx, topID)
				if err != nil || len(kids) != 1 {
					t.Fatalf("episodes=%v err=%v", kids, err)
				}
				watchedID = kids[0].ID
			}
			files, err := st.ItemFiles(ctx, watchedID)
			if err != nil || len(files) != 1 {
				t.Fatalf("files=%v err=%v", files, err)
			}
			oldFileID := files[0].ID
			u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if err := st.SetPlayed(ctx, u.ID, []string{watchedID}, true); err != nil {
				t.Fatal(err)
			}
			oldPath, newPath := filepath.Join(root, filepath.FromSlash(oldName)), filepath.Join(root, filepath.FromSlash(newName))
			// Only these generated scratch files are moved; restore before fixture's unchanged-file check.
			if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(oldPath, newPath); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := os.Rename(newPath, oldPath); err != nil {
					t.Error(err)
				}
			})
			if err := s.scan(ctx, lib); err != nil {
				t.Fatal(err)
			}
			after, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: mediaKind})
			if err != nil || len(after) != 1 {
				t.Fatalf("after=%v err=%v", after, err)
			}
			if after[0].ID != topID {
				t.Errorf("%s rename changed item identity: %s -> %s", kind, topID, after[0].ID)
			}
			afterID := after[0].ID
			if kind == "shows" {
				kids, err := st.Children(ctx, afterID)
				if err != nil || len(kids) != 1 {
					t.Fatalf("episodes=%v err=%v", kids, err)
				}
				afterID = kids[0].ID
			}
			state, err := st.UserDataFor(ctx, u.ID, []string{afterID})
			if err != nil {
				t.Fatal(err)
			}
			if afterID != watchedID || !state[afterID].Played {
				t.Errorf("rename lost watched episode/movie identity: same=%v played=%v", afterID == watchedID, state[afterID].Played)
			}
			files, err = st.ItemFiles(ctx, afterID)
			if err != nil || len(files) != 1 {
				t.Fatalf("after files=%v err=%v", files, err)
			}
			if files[0].ID != oldFileID {
				t.Errorf("rename discarded per-file identity: %d -> %d", oldFileID, files[0].ID)
			}
		})
	}
}
