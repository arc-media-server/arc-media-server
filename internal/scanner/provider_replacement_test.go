package scanner

import (
	"ams/internal/metadata"
	"testing"
)

func TestCanonicalProviderEpisodeReplacementKeepsFileWatchState(t *testing.T) {
	for _, mode := range []string{"refresh", "fix-match"} {
		t.Run(mode, func(t *testing.T) {
			st, s, lib, show, f := matchSetup(t, 0)
			if _, err := s.SetMatch(t.Context(), show, "tmdb", 303); err != nil {
				t.Fatal(err)
			}
			s.Wait()
			old, err := st.GetItemByKey(t.Context(), lib, episodeKey(show.ID, "tmdb", 100101))
			if err != nil || old == nil {
				t.Fatalf("first episode missing: %v", err)
			}
			files, err := st.ItemFiles(t.Context(), old.ID)
			if err != nil || len(files) != 1 {
				t.Fatalf("files=%v err=%v", files, err)
			}
			fileID := files[0].ID
			user, err := st.CreateUser(t.Context(), "Viewer", false, "blue", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.ReportProgress(t.Context(), user.ID, old.ID, 120000, 3600000); err != nil {
				t.Fatal(err)
			}
			f.episodeOffset.Store(1000000)
			// A new client observes the provider's new response, not its previous response cache.
			s.tmdb = metadata.NewTMDB("fixture-only")
			s.tvdb = metadata.NewTVDB("fixture-only")
			cfg, err := st.GetLibrary(t.Context(), lib)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "refresh" {
				if err := s.RefreshSeries(t.Context(), cfg, show.ID, false, nil); err != nil {
					t.Fatal(err)
				}
			} else {
				current, err := st.GetItem(t.Context(), show.ID)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := s.SetMatch(t.Context(), current, "tmdb", 303); err != nil {
					t.Fatal(err)
				}
				s.Wait()
			}
			replacement, err := st.GetItemByKey(t.Context(), lib, episodeKey(show.ID, "tmdb", 1100101))
			if err != nil || replacement == nil {
				t.Fatalf("replacement missing: %v", err)
			}
			state, err := st.UserDataFor(t.Context(), user.ID, []string{replacement.ID})
			if err != nil {
				t.Fatal(err)
			}
			if replacement.ID != old.ID || state[replacement.ID].PositionMS != 120000 {
				t.Errorf("%s lost identity/resume: same=%v position=%d", mode, replacement.ID == old.ID, state[replacement.ID].PositionMS)
			}
			files, err = st.ItemFiles(t.Context(), replacement.ID)
			if err != nil || len(files) != 1 || files[0].ID != fileID {
				t.Fatalf("replacement files=%v err=%v", files, err)
			}
		})
	}
}

func TestProviderReplacementDoesNotStealRetainedEpisodeAtAnotherPosition(t *testing.T) {
	st, s, lib, show, f := matchSetup(t, 0)
	if _, err := s.SetMatch(t.Context(), show, "tmdb", 303); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	original, err := st.GetItemByKey(t.Context(), lib, episodeKey(show.ID, "tmdb", 100101))
	if err != nil || original == nil {
		t.Fatal("initial identity missing")
	}
	// S01E01 gets a new record; its previous ID is still valid, now at S01E02.
	f.replaceFirstAndMove.Store(true)
	s.tmdb = metadata.NewTMDB("fixture-only")
	s.tvdb = metadata.NewTVDB("fixture-only")
	cfg, err := st.GetLibrary(t.Context(), lib)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshSeries(t.Context(), cfg, show.ID, false, nil); err != nil {
		t.Fatal(err)
	}
	retained, err := st.GetItemByKey(t.Context(), lib, episodeKey(show.ID, "tmdb", 100101))
	if err != nil || retained == nil || retained.ID != original.ID {
		t.Fatalf("retained provider identity stolen: %+v err=%v", retained, err)
	}
	fresh, err := st.GetItemByKey(t.Context(), lib, episodeKey(show.ID, "tmdb", 1000101))
	if err != nil || fresh == nil || fresh.ID == original.ID {
		t.Fatalf("new record incorrectly inherited retained episode: %+v err=%v", fresh, err)
	}
}
