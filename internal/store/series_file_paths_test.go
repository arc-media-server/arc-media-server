package store

import (
	"slices"
	"testing"
)

func TestSeriesFilePathsPreferPresentAndDeduplicate(t *testing.T) {
	ctx := t.Context()
	s := openTest(t)
	lib, err := s.SyncLibrary(ctx, "Series", "shows", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}

	addItem := func(kind, title, parent string) string {
		t.Helper()
		id, _, err := s.EnsureItem(ctx, &Item{LibraryID: lib, Kind: kind, Title: title, ParentID: parent}, title)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	series := addItem("series", "Show", "")
	e1, e2 := addItem("episode", "First", series), addItem("episode", "Second", series)
	other := addItem("episode", "Other episode", addItem("series", "Other show", ""))

	const (
		missing = "/fixture/Show/A-missing.mkv"
		shared  = "/fixture/Show/B-shared.mkv"
		present = "/fixture/Show/C-present.mkv"
	)
	addFile := func(path string, seen int64, episodes ...string) {
		t.Helper()
		id, err := s.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Size: 1}, 1, seen)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetFileItems(ctx, id, episodes, 0); err != nil {
			t.Fatal(err)
		}
	}
	addFile(missing, 1, e1)
	addFile(shared, 2, e1, e2)
	addFile(present, 2, e2)
	addFile("/fixture/Other show/0-present.mkv", 2, other)
	if _, err := s.MarkMissing(ctx, lib, 2, 2); err != nil {
		t.Fatal(err)
	}

	want := []string{shared, present, missing}
	for _, limit := range []int{1, 2, 3} {
		got, err := s.SeriesFilePaths(ctx, series, limit)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, want[:limit]) {
			t.Fatalf("limit %d: got %v, want %v", limit, got, want[:limit])
		}
	}
}
