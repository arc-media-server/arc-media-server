package scanner

import (
	"ams/internal/metadata"
	"testing"
)

func TestSuspiciousProviderEpisodeResponsesKeepSavedOrdering(t *testing.T) {
	for mode, name := range map[int64]string{1: "empty", 2: "less-than-half", 3: "source-empty-with-other-orders"} {
		t.Run(name, func(t *testing.T) {
			st, s, lib, show, f := matchSetup(t, 0)
			if _, err := s.SetMatch(t.Context(), show, "tmdb", 303); err != nil {
				t.Fatal(err)
			}
			s.Wait()
			orders, err := st.ListOrderings(t.Context(), show.ID)
			if err != nil {
				t.Fatal(err)
			}
			var source int64
			for _, order := range orders {
				if order.Source == "tmdb_aired" {
					source = order.ID
				}
			}
			entries, err := st.OrderingEntries(t.Context(), source)
			if err != nil || len(entries) != fixtureSeasons*fixtureEpisodes {
				t.Fatalf("source=%d count=%d err=%v", source, len(entries), err)
			}
			// A provider omission must not destroy manual placement in the base source ordering.
			if err := st.SetEntry(t.Context(), source, entries[0].EpisodeID, entries[0].Season, entries[0].Episode); err != nil {
				t.Fatal(err)
			}
			f.episodeMode.Store(mode)
			s.tmdb = metadata.NewTMDB("fixture-only")
			s.tvdb = metadata.NewTVDB("fixture-only")
			_, offered, fetchErr := s.tmdb.TV(t.Context(), 303)
			if fetchErr != nil {
				t.Fatal(fetchErr)
			}
			counts := map[string]int{}
			for _, order := range offered {
				counts[order.Source] = len(order.Entries)
			}
			t.Logf("fixture mode=%d offered counts=%v requests=%d", mode, counts, f.requests.Load())
			cfg, err := st.GetLibrary(t.Context(), lib)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.RefreshSeries(t.Context(), cfg, show.ID, false, nil); err == nil {
				t.Error("suspicious response was accepted")
			}
			after, err := st.OrderingEntries(t.Context(), source)
			if err != nil || len(after) != len(entries) {
				t.Errorf("saved source damaged: count=%d want=%d err=%v", len(after), len(entries), err)
			}
			beforeIDs := map[[2]int]string{}
			for _, entry := range entries {
				beforeIDs[[2]int{entry.Season, entry.Episode}] = entry.EpisodeID
			}
			changed := 0
			for _, entry := range after {
				if beforeIDs[[2]int{entry.Season, entry.Episode}] != entry.EpisodeID {
					changed++
				}
			}
			if changed > 0 {
				t.Errorf("provider response replaced %d stored episode identities with fallback identities", changed)
			}
			foundLocked := false
			for _, entry := range after {
				if entry.EpisodeID == entries[0].EpisodeID && entry.Locked {
					foundLocked = true
				}
			}
			if !foundLocked {
				t.Error("locked placement was lost")
			}
		})
	}
}

func TestProviderOmissionKeepsEditedAlternateOrdering(t *testing.T) {
	st, s, lib, show, f := matchSetup(t, 0)
	if _, err := s.SetMatch(t.Context(), show, "tmdb", 303); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	order, err := st.FindOrdering(t.Context(), show.ID, "tvdb_dvd", "")
	if err != nil || order == 0 {
		t.Fatalf("DVD order missing: %v", err)
	}
	entries, err := st.OrderingEntries(t.Context(), order)
	if err != nil || len(entries) == 0 {
		t.Fatalf("entries missing: %v", err)
	}
	if err := st.SetEntry(t.Context(), order, entries[0].EpisodeID, 2, 99); err != nil {
		t.Fatal(err)
	}
	f.omitDVD.Store(true)
	s.tmdb = metadata.NewTMDB("fixture-only")
	s.tvdb = metadata.NewTVDB("fixture-only")
	cfg, err := st.GetLibrary(t.Context(), lib)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshSeries(t.Context(), cfg, show.ID, false, nil); err != nil {
		t.Fatal(err)
	}
	kept, err := st.GetOrdering(t.Context(), order)
	if err != nil || kept == nil {
		t.Fatalf("edited alternate ordering was deleted: %v", err)
	}
	after, err := st.OrderingEntries(t.Context(), order)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range after {
		if entry.EpisodeID == entries[0].EpisodeID && entry.Locked && entry.Season == 2 && entry.Episode == 99 {
			found = true
		}
	}
	if !found {
		t.Fatal("edited alternate placement lost")
	}
}
