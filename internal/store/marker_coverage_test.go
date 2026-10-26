package store

import (
	"context"
	"testing"
)

func TestMarkerCoverageCountsShowsAndMovies(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "Mixed", "shows", "tvdb_aired", []string{"/m"})
	if err != nil {
		t.Fatal(err)
	}
	series, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Grey's Anatomy"}, "series:greys")
	if err != nil {
		t.Fatal(err)
	}
	add := func(path, key, kind, parent string) int64 {
		id, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: kind, ParentID: parent, Title: key}, key)
		if err != nil {
			t.Fatal(err)
		}
		f, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Size: 1, DurationMS: 2_000_000}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetFileItems(ctx, f, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
		return f
	}
	e1 := add("/m/e1.mkv", "ep1", "episode", series)
	e2 := add("/m/e2.mkv", "ep2", "episode", series)
	add("/m/e3.mkv", "ep3", "episode", series)
	movie := add("/m/dune.mkv", "Dune", "movie", "")
	if err := st.SaveMarkers(ctx, e1, []Marker{{Kind: "intro", StartMS: 1000, EndMS: 60000}, {Kind: "credits", StartMS: 1_900_000, EndMS: 2_000_000}}); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveMarkers(ctx, e2, []Marker{{Kind: "intro", StartMS: 1000, EndMS: 60000}}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []int64{e1, e2, movie} {
		if err := st.MarkersChecked(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.MarkerCoverage(ctx, lib)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]MarkerCoverage{
		"Grey's Anatomy": {Kind: "series", Files: 3, Intros: 2, Credits: 1, Checked: 2},
		"Dune":           {Kind: "movie", Files: 1, Intros: 0, Credits: 0, Checked: 1},
	}
	if len(got) != 2 {
		t.Fatalf("coverage = %+v", got)
	}
	for _, c := range got {
		w := want[c.Title]
		if c.Kind != w.Kind || c.Files != w.Files || c.Intros != w.Intros || c.Credits != w.Credits || c.Checked != w.Checked {
			t.Errorf("%s = %+v, want %+v", c.Title, c, w)
		}
	}
}
