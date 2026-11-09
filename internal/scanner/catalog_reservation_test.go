package scanner

import "testing"

func TestCatalogReservationWaitsForBackgroundMatchAndResumesRequests(t *testing.T) {
	st, s, lib, show, f := matchSetup(t, 0)
	unblock := f.hold()
	defer unblock()
	if _, err := s.SetMatch(t.Context(), show, "tmdb", 303); err != nil {
		t.Fatal(err)
	}
	if release, ok := st.ReserveCatalogMutation(); ok {
		release()
		t.Error("reservation ignored a background match")
	}
	unblock()
	s.Wait()
	release, ok := st.ReserveCatalogMutation()
	if !ok {
		t.Fatal("finished match retained its lease")
	}
	if s.Start(lib) {
		t.Fatal("scan admitted during reservation")
	}
	s.Request(lib)
	release()
	s.ResumeQueued()
	s.Wait()
	if s.Status(lib).FilesSeen != fixtureSeasons*fixtureEpisodes {
		t.Fatalf("queued scan did not resume: %+v", s.Status(lib))
	}
}
func TestCatalogReservationDropsDeletedLibraryRequests(t *testing.T) {
	st, s, lib, _ := libraryFixture(t, "movies", map[string]string{"Film (2000).mkv": "fixture"})
	release, ok := st.ReserveCatalogMutation()
	if !ok {
		t.Fatal("reservation unavailable")
	}
	s.Request(lib)
	if err := st.DeleteLibrary(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	release()
	s.ResumeQueued()
	s.Wait()
	if s.Running() || s.again[lib] || len(s.pendingPaths[lib]) != 0 {
		t.Fatal("deleted library retained/resumed stale discovery")
	}
}
