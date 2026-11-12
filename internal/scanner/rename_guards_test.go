package scanner

import (
	"ams/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenameDoesNotAdoptPresentOrAmbiguousFiles(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		t.Run(map[bool]string{false: "present", true: "ambiguous"}[ambiguous], func(t *testing.T) {
			st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Alien (1979) [a].mkv": "fixture", "Alien (1979) [b].mkv": "fixture"})
			cfg, err := st.GetLibrary(t.Context(), lib)
			if err != nil {
				t.Fatal(err)
			}
			stamp := time.Unix(1000000, 0)
			for _, suffix := range []string{"a", "b"} {
				path := filepath.Join(root, "Alien (1979) ["+suffix+"].mkv")
				if err := os.Chtimes(path, stamp, stamp); err != nil {
					t.Fatal(err)
				}
				if _, err := st.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: path, Size: 7, ModTime: stamp.Unix()}, 1, 1); err != nil {
					t.Fatal(err)
				}
			}
			target := filepath.Join(root, "Alien (1979) {tmdb-348}.mkv")
			if err := os.WriteFile(target, []byte("fixture"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(target, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			if ambiguous {
				for _, suffix := range []string{"a", "b"} {
					old := filepath.Join(root, "Alien (1979) ["+suffix+"].mkv")
					moved := old + ".moved"
					if err := os.Rename(old, moved); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.Rename(moved, old); err != nil {
							t.Error(err)
						}
					})
				}
			}
			info, err := os.Stat(target)
			if err != nil {
				t.Fatal(err)
			}
			reused, err := s.reuseRenamedFile(t.Context(), cfg, target, info)
			if err != nil || reused != nil {
				t.Fatalf("unsafe reuse: %v err=%v", reused, err)
			}
		})
	}
}
