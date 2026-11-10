package scanner

import (
	"ams/internal/store"
	"os"
	"path/filepath"
	"testing"
)

func TestEmptyMountPointDoesNotHideLibrary(t *testing.T) {
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"One (2000).mkv": "one fixture", "Two (2001).mkv": "second fixture"})
	if err := s.scan(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	before, _, err := st.ListItems(t.Context(), store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(before) != 2 {
		t.Fatalf("items=%v err=%v", before, err)
	}
	// Simulate an unmounted share with an empty but readable mount point, using scratch media only.
	offline := filepath.Join(t.TempDir(), "offline")
	if err := os.Rename(root, offline); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(root); err != nil {
			t.Error(err)
		}
		if err := os.Rename(offline, root); err != nil {
			t.Error(err)
		}
	})
	if err := s.scan(t.Context(), lib); err == nil {
		t.Error("empty mount point reported successful reconciliation")
	}
	after, _, err := st.ListItems(t.Context(), store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(after) != 2 {
		t.Errorf("empty mount hid the library: count=%d err=%v", len(after), err)
	}
	missing, err := st.MissingFiles(t.Context(), lib)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Errorf("empty mount marked %d files missing", len(missing))
	}
	if len(s.TrashCandidates()) != 0 {
		t.Error("empty mount authorised automatic trash")
	}
}

func TestReadableKnownAnchorStillAllowsAnOrdinaryRemoval(t *testing.T) {
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"One (2000).mkv": "one fixture", "Two (2001).mkv": "second fixture"})
	if err := s.scan(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(root, "One (2000).mkv")
	offline := filepath.Join(t.TempDir(), "removed.mkv")
	if err := os.Rename(original, offline); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Rename(offline, original); err != nil {
			t.Error(err)
		}
	})
	if err := s.scan(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	missing, err := st.MissingFiles(t.Context(), lib)
	if err != nil || len(missing) != 1 {
		t.Fatalf("ordinary removal not reconciled: %v err=%v", missing, err)
	}
}
func TestPartialScanEmptyMountCannotUseNeighbourAnchor(t *testing.T) {
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{"Mount A/One (2000).mkv": "one fixture", "Mount B/Two (2001).mkv": "second fixture"})
	if _, err := st.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{filepath.Join(root, "Mount A"), filepath.Join(root, "Mount B")}); err != nil {
		t.Fatal(err)
	}
	if err := s.scan(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(root, "Mount A")
	offline := filepath.Join(t.TempDir(), "offline")
	if err := os.Rename(original, offline); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(original, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(original); err != nil {
			t.Error(err)
		}
		if err := os.Rename(offline, original); err != nil {
			t.Error(err)
		}
	})
	if err := s.scanPaths(t.Context(), lib, []string{original}); err == nil {
		t.Error("empty submount treated as online")
	}
	missing, err := st.MissingFiles(t.Context(), lib)
	if err != nil || len(missing) != 0 {
		t.Fatalf("neighbour anchor authorised missing state: %v err=%v", missing, err)
	}
}
