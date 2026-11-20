package store

import (
	"context"
	"testing"
)

func TestFileActivityLabelNamesEpisodesAndMovies(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, err := st.SyncLibrary(ctx, "TV", "shows", "tvdb_aired", []string{"/tv"})
	if err != nil {
		t.Fatal(err)
	}
	series, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "series", Title: "Grey's Anatomy"}, "series:greys")
	if err != nil {
		t.Fatal(err)
	}
	episode, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", ParentID: series, Title: "Where the Boys Are"}, "ep:greys:12")
	if err != nil {
		t.Fatal(err)
	}
	movie, _, err := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: "Dune"}, "title:dune:2021")
	if err != nil {
		t.Fatal(err)
	}
	link := func(path, item string) int64 {
		id, err := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: path, Size: 1}, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetFileItems(ctx, id, []string{item}, 0); err != nil {
			t.Fatal(err)
		}
		return id
	}
	epFile, movieFile := link("/tv/greys.s04e12.mkv", episode), link("/tv/dune.mkv", movie)

	// Without an aired order the show's title is still given.
	if got, err := st.FileActivityLabel(ctx, epFile); err != nil || got != "Grey's Anatomy" {
		t.Fatalf("unordered episode = %q, %v", got, err)
	}
	if _, err := st.db.ExecContext(ctx, `INSERT INTO orderings (id, series_id, source, name, updated_at) VALUES (1, ?, 'tvdb_aired', 'Aired', 1)`, series); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.ExecContext(ctx, `INSERT INTO ordering_entries (ordering_id, episode_id, season_number, episode_number) VALUES (1, ?, 4, 12)`, episode); err != nil {
		t.Fatal(err)
	}
	if got, err := st.FileActivityLabel(ctx, epFile); err != nil || got != "Grey's Anatomy · S4 · E12" {
		t.Fatalf("episode = %q, %v", got, err)
	}
	if got, err := st.FileActivityLabel(ctx, movieFile); err != nil || got != "Dune" {
		t.Fatalf("movie = %q, %v", got, err)
	}
}
