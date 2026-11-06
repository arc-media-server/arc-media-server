package scanner

import (
	"ams/internal/events"
	"ams/internal/metadata"
	"ams/internal/store"
	"testing"
)

func TestMovieMetadataHintFollowsCommit(t *testing.T) {
	st, s, lib, _ := libraryFixture(t, "movies", nil)
	id, _, err := st.EnsureItem(t.Context(), &store.Item{LibraryID: lib, Kind: "movie", Title: "Unmatched"}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	item, err := st.GetItem(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	s.Events = events.New()
	_, cursor, _ := s.Events.Read("")
	if err := s.saveMovie(t.Context(), *item, &metadata.Movie{TMDBID: 42, Title: "Matched fixture", Year: 2026}); err != nil {
		t.Fatal(err)
	}
	hints, _, stale := s.Events.Read(cursor)
	saved, err := st.GetItem(t.Context(), id)
	if err != nil || saved.Title != "Matched fixture" {
		t.Fatalf("metadata not saved: %+v %v", saved, err)
	}
	if stale || len(hints) != 1 || hints[0].Topic != "catalog" || hints[0].LibraryID != lib {
		t.Fatalf("hints: %+v", hints)
	}
}
