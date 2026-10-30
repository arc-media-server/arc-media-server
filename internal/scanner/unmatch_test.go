package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ams/internal/store"
)

func TestUnmatchPreservesFileIdentitiesAndStaysSuppressed(t *testing.T) {
	st, s, lib, it, provider := matchSetup(t, 0)
	if _, err := s.SetMatch(t.Context(), it, "tvdb", 404); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	kids := checkEpisodes(t, st, lib, it.ID)
	before := map[string]int64{}
	for _, kid := range kids {
		fs, err := st.ItemFiles(t.Context(), kid.ID)
		if err != nil {
			t.Fatal(err)
		}
		before[kid.ID] = fs[0].ID
	}
	user, err := st.CreateUser(t.Context(), "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	watched := kids[0].ID
	if _, err = st.ReportProgress(t.Context(), user.ID, watched, 120000, 3600000); err != nil {
		t.Fatal(err)
	}
	playlist, err := st.CreatePlaylist(t.Context(), user.ID, "Saved")
	if err != nil {
		t.Fatal(err)
	}
	if err = st.AddToPlaylist(t.Context(), playlist.ID, []string{watched}); err != nil {
		t.Fatal(err)
	}
	custom := "My edit"
	if err = st.EditItem(t.Context(), it.ID, store.ItemEdit{Title: &custom}); err != nil {
		t.Fatal(err)
	}
	calls := provider.requests.Load()
	if err = s.Unmatch(t.Context(), it); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetItem(t.Context(), it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.MatchSuppressed || got.MetadataState != "unmatched" || got.TMDBID != 0 || got.TVDBID != 0 || got.Overview != "" || len(got.LockedFields) != 0 || got.Title == custom {
		t.Fatalf("reset failed %+v", got)
	}
	orders, err := st.ListOrderings(t.Context(), it.ID)
	if err != nil || len(orders) != 1 || orders[0].Source != "file" {
		t.Fatalf("orders %+v %v", orders, err)
	}
	for i := 0; i < 2; i++ {
		if err = st.RetryUnmatched(t.Context(), lib, time.Now().Add(24*time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
		if err = s.scan(t.Context(), lib); err != nil {
			t.Fatal(err)
		}
	}
	if provider.requests.Load() != calls {
		t.Fatal("unmatch/rescan fetched provider metadata")
	}
	for id, fileID := range before {
		fs, err := st.ItemFiles(t.Context(), id)
		if err != nil || len(fs) != 1 || fs[0].ID != fileID {
			t.Fatalf("identity lost %s %+v %v", id, fs, err)
		}
	}
	data, err := st.UserDataFor(t.Context(), user.ID, []string{watched})
	if err != nil || data[watched].PositionMS != 120000 {
		t.Fatal("resume lost", err)
	}
	entries, _, err := st.PlaylistEntries(t.Context(), playlist.ID, store.Access{})
	if err != nil || len(entries) != 1 || entries[0].ItemID != watched {
		t.Fatal("playlist lost", err)
	}
	if _, err = s.SetMatch(t.Context(), got, "tvdb", 404); err != nil {
		t.Fatal(err)
	}
	s.Wait()
	matched, err := st.GetItem(t.Context(), it.ID)
	if err != nil || matched.MatchSuppressed || matched.MetadataState != "matched" {
		t.Fatalf("rematch failed %+v %v", matched, err)
	}
	data, err = st.UserDataFor(t.Context(), user.ID, []string{watched})
	if err != nil || data[watched].PositionMS != 120000 {
		t.Fatal("rematch lost resume", err)
	}
}

func TestUnmatchMultiEpisodeDateFilesAndBusyLease(t *testing.T) {
	st, s, lib, root := libraryFixture(t, "shows", map[string]string{"Example/Example S01E01E02.mkv": "multi", "Example/Example 2025-01-02.mkv": "dated"})
	if err := s.scan(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	shows, _, err := st.ListItems(t.Context(), store.ItemQuery{LibraryID: lib, Kind: "series"})
	if err != nil || len(shows) != 1 {
		t.Fatal("missing show", err)
	}
	it := shows[0]
	_, release, err := s.beginScopedWork(t.Context(), lib, "test", "Held")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Unmatch(t.Context(), &it); !errors.Is(err, ErrLibraryBusy) {
		t.Fatalf("busy %v", err)
	}
	release(nil)
	if err = s.Unmatch(t.Context(), &it); err != nil {
		t.Fatal(err)
	}
	kids, err := st.Children(t.Context(), it.ID)
	if err != nil || len(kids) < 2 {
		t.Fatalf("episodes %v %v", kids, err)
	}
	ids := map[string]bool{}
	for _, kid := range kids {
		ids[kid.ID] = true
	}
	if err = os.WriteFile(filepath.Join(root, "Example/Example S01E03.mkv"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "Example/Example 2025-02-03.mkv"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = s.scan(t.Context(), lib); err != nil {
		t.Fatal(err)
	}
	after, err := st.Children(t.Context(), it.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, kid := range after {
		delete(ids, kid.ID)
	}
	if len(ids) > 0 {
		t.Fatal("multi episode identities changed", ids)
	}
	got, _ := st.GetItem(t.Context(), it.ID)
	if !got.MatchSuppressed {
		t.Fatal("scan cleared suppression")
	}
}
