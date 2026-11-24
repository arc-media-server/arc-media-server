package store

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestConfiguredLibrarySavesAllOptionsAndRollsBackLatePathFailure(t *testing.T) {
	s := openTest(t)
	expected := Library{Name: "Complete", Kind: "shows", SourceOrdering: "tvdb_aired", Paths: []string{"/fixture/complete"}, DetailsProvider: "tvdb", Trickplay: true, CollectionMin: 4, MetadataLanguage: "fr-FR", RatingRegion: "GB", SkipIntros: false, SkipCredits: true, CollectionGrid: "both", SeasonLabelMode: "name"}
	id, err := s.CreateConfiguredLibrary(t.Context(), expected)
	if err != nil {
		t.Fatal(err)
	}
	expected.ID = id
	actual, err := s.GetLibrary(t.Context(), id)
	if err != nil || !reflect.DeepEqual(actual, &expected) {
		t.Fatal("complete options not persisted", actual, expected, err)
	}
	failed := Library{Name: "Must roll back", Kind: "movies", SourceOrdering: "tmdb_aired", Paths: []string{"/fixture/new", "/fixture/new"}}
	if id, err = s.CreateConfiguredLibrary(t.Context(), failed); err == nil || id != 0 {
		t.Fatal("late root write failure ignored", id, err)
	}
	libraries, err := s.ListLibraries(t.Context())
	if err != nil || len(libraries) != 1 {
		t.Fatal("late failure left partial configuration", libraries, err)
	}
}

func TestConcurrentConfiguredLibraryRechecksNameAndRootOwnership(t *testing.T) {
	for _, conflict := range []string{"name", "folder"} {
		t.Run(conflict, func(t *testing.T) {
			s := openTest(t)
			a := Library{Name: "Shared name", Kind: "movies", SourceOrdering: "tmdb_aired", Paths: []string{"/fixture/one"}}
			b := Library{Name: "shared NAME", Kind: "movies", SourceOrdering: "tmdb_aired", Paths: []string{"/fixture/two"}}
			expected := ErrDuplicateName
			if conflict == "folder" {
				b.Name = "Different name"
				b.Paths = []string{"/fixture/one/child"}
				expected = ErrLibraryPathConflict
			}
			gate := make(chan struct{})
			results := make(chan error, 2)
			var workers sync.WaitGroup
			for _, lib := range []Library{a, b} {
				workers.Go(func() { <-gate; _, err := s.CreateConfiguredLibrary(t.Context(), lib); results <- err })
			}
			close(gate)
			workers.Wait()
			close(results)
			successes, rejected := 0, 0
			for err := range results {
				if err == nil {
					successes++
				} else if errors.Is(err, expected) {
					rejected++
				} else {
					t.Fatal(err)
				}
			}
			if successes != 1 || rejected != 1 {
				t.Fatal("conflicting libraries both admitted", successes, rejected)
			}
			libraries, err := s.ListLibraries(t.Context())
			if err != nil || len(libraries) != 1 {
				t.Fatal("conflict changed existing library", libraries, err)
			}
		})
	}
}
