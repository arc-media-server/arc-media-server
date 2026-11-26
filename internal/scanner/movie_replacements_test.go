package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"ams/internal/store"
)

func TestMovieReplacementRecentlyAdded(t *testing.T) {
	for _, tc := range []struct {
		name     string
		newPath  string
		replace  bool
		rename   bool
		scanBoth bool
		promoted bool
	}{
		{name: "replacement in scoped scan", newPath: "Arrival (2016)/Arrival (2016) - 2160p.mkv", replace: true, promoted: true},
		{name: "parallel version", newPath: "Arrival (2016)/Arrival (2016) - 2160p.mkv"},
		{name: "remove after scanning both versions", newPath: "Arrival (2016)/Arrival (2016) - 2160p.mkv", scanBoth: true},
		{name: "pure rename", newPath: "Arrival (2016)/Arrival (2016) - BluRay.mkv", rename: true},
		{name: "between folders", newPath: "Archive/Arrival (2016) - 2160p.mkv", replace: true},
		{name: "same path change", newPath: "Arrival (2016)/Arrival (2016) - 1080p.mkv"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Anchor (2000).mkv": "storage anchor"})
			oldPath := filepath.Join(root, "Arrival (2016)", "Arrival (2016) - 1080p.mkv")
			write := func(path, body string) {
				t.Helper()
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			scan := func(paths ...string) {
				t.Helper()
				if err := s.scanPaths(ctx, lib, paths); err != nil {
					t.Fatal(err)
				}
			}
			list := func() []store.Item {
				t.Helper()
				items, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "movie", Sort: "added"})
				if err != nil {
					t.Fatal(err)
				}
				return items
			}
			write(oldPath, "old movie")
			scan()
			items := list()
			if len(items) != 2 {
				t.Fatalf("expected the movie and storage anchor, got %d items", len(items))
			}
			old := items[0]
			if old.Title != "Arrival" {
				t.Fatalf("unexpected first arrival: %+v", old)
			}
			profile, err := st.CreateUser(ctx, "Replacement fixture", true, "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.ReportProgress(ctx, profile.ID, old.ID, 75000, 1000000); err != nil {
				t.Fatal(err)
			}
			newerPath := filepath.Join(root, "Memento (2000).mkv")
			write(newerPath, "unrelated new movie")
			scan()
			if got := list()[0].Title; got != "Memento" {
				t.Fatalf("unexpected newest movie before replacement: %s", got)
			}

			freshPath := filepath.Join(root, filepath.FromSlash(tc.newPath))
			switch {
			case tc.rename:
				if err := os.Rename(oldPath, freshPath); err != nil {
					t.Fatal(err)
				}
			default:
				write(freshPath, "different size for the new source")
				if tc.scanBoth {
					scan()
				}
				if (tc.replace || tc.scanBoth) && oldPath != freshPath {
					if err := os.Remove(oldPath); err != nil {
						t.Fatal(err)
					}
				}
			}
			if tc.promoted {
				scan(freshPath)
			} else {
				scan()
			}
			want := "Memento"
			if tc.promoted {
				want = "Arrival"
			}
			assertOrder := func() {
				t.Helper()
				items := list()
				if len(items) != 3 || items[0].Title != want {
					t.Fatalf("expected %s first among three movies, got %+v", want, items)
				}
				for _, item := range items {
					if item.Title == old.Title && (item.ID != old.ID || item.AddedAt != old.AddedAt) {
						t.Fatalf("original identity and arrival changed: before=%+v after=%+v", old, item)
					}
				}
			}
			assertOrder()

			data, err := st.UserDataFor(ctx, profile.ID, []string{old.ID})
			if err != nil || data[old.ID].PositionMS != 75000 {
				t.Fatalf("watch progress changed: %v, %v", data, err)
			}
			// Rescan after another movie arrives to detect a repeated promotion.
			write(filepath.Join(root, "Solaris (1972).mkv"), "another new movie")
			scan()
			if got := list()[0].Title; got != "Solaris" {
				t.Fatalf("routine scan promoted a movie again: %s", got)
			}
		})
	}
}
