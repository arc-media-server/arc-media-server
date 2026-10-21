package scanner

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/store"
)

func libraryFixture(t *testing.T, kind string, files map[string]string) (*store.Store, *Scanner, int64, string) {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	root := filepath.Join(dir, "media")
	st, err := store.Open(ctx, filepath.Join(dir, "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lib, err := st.SyncLibrary(ctx, "Fixture", kind, "tmdb_aired", []string{root})
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		if videoExts[filepath.Ext(path)] {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := st.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}, 1, 1); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() {
		for name, body := range files {
			got, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil || string(got) != body {
				t.Errorf("source file changed: %s (%v)", name, err)
			}
		}
	})
	s := New(ctx, st, probe.New(""), metadata.NewTMDB(""), metadata.NewTVDB(""), slog.New(slog.DiscardHandler))
	s.status[lib] = &Status{}
	return st, s, lib, root
}

func TestTVRescanPreservesMultiEpisodeLinksAndCustomOrder(t *testing.T) {
	ctx := context.Background()
	st, s, lib, _ := libraryFixture(t, "shows", map[string]string{
		"Example Show/Season 01/Example Show S01E01E02.mkv": "two episodes",
		"Example Show/Season 00/Example Show S00E01.mkv":    "special",
		"Example Show/Season 01/Example Show S01E20.mkv":    "unknown episode",
	})
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	shows, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	if err != nil || len(shows) != 1 {
		t.Fatalf("shows=%v err=%v", shows, err)
	}
	show := shows[0]
	kids, err := st.Children(ctx, show.ID)
	if err != nil || len(kids) != 4 {
		t.Fatalf("episodes=%v err=%v", kids, err)
	}
	orders, err := st.ListOrderings(ctx, show.ID)
	if err != nil || len(orders) != 1 {
		t.Fatalf("orders=%v err=%v", orders, err)
	}
	entries, err := st.OrderingEntries(ctx, orders[0].ID)
	if err != nil || len(entries) != 4 {
		t.Fatalf("entries=%v err=%v", entries, err)
	}
	var first, second, special, unknown string
	for _, e := range entries {
		switch {
		case e.Season == 0:
			special = e.EpisodeID
		case e.Episode == 1:
			first = e.EpisodeID
		case e.Episode == 2:
			second = e.EpisodeID
		case e.Episode == 20:
			unknown = e.EpisodeID
		}
	}
	if first == "" || second == "" || special == "" || unknown == "" {
		t.Fatalf("positions missing: %v", entries)
	}
	f1, err := st.ItemFiles(ctx, first)
	if err != nil || len(f1) != 1 {
		t.Fatalf("first files=%v err=%v", f1, err)
	}
	f2, err := st.ItemFiles(ctx, second)
	if err != nil || len(f2) != 1 || f1[0].ID != f2[0].ID {
		t.Fatalf("multi-episode link lost: %v %v %v", f1, f2, err)
	}
	custom, err := st.CopyOrdering(ctx, &orders[0], "My order")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetEntry(ctx, custom, second, 2, 1); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDisplayOrdering(ctx, show.ID, "", custom); err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, u.ID, second, 120000, 3600000); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := s.scan(ctx, lib); err != nil {
			t.Fatal(err)
		}
	}
	kidsAfter, err := st.Children(ctx, show.ID)
	if err != nil || len(kidsAfter) != 4 {
		t.Fatalf("rescan duplicated episodes: %v %v", kidsAfter, err)
	}
	for _, id := range []string{first, second, special, unknown} {
		if it, err := st.GetItem(ctx, id); err != nil || it == nil {
			t.Fatalf("episode identity lost: %s %v", id, err)
		}
	}
	got, err := st.OrderingEntries(ctx, custom)
	if err != nil {
		t.Fatal(err)
	}
	placed := false
	for _, e := range got {
		if e.EpisodeID == second && e.Season == 2 && e.Episode == 1 && e.Locked {
			placed = true
		}
	}
	if !placed {
		t.Fatalf("manual placement lost: %v", got)
	}
	display, err := st.DisplayOrdering(ctx, show.ID, "", "tmdb_aired")
	if err != nil || display != custom {
		t.Fatalf("display=%d err=%v", display, err)
	}
	data, err := st.UserDataFor(ctx, u.ID, []string{second})
	if err != nil || data[second].PositionMS != 120000 {
		t.Fatalf("resume=%v err=%v", data, err)
	}
	f2, err = st.ItemFiles(ctx, second)
	if err != nil || len(f2) != 1 || f2[0].ID != f1[0].ID {
		t.Fatalf("rescan changed file linkage: %v %v", f2, err)
	}
}

func TestMovieVersionsLocalAssetsAndRescan(t *testing.T) {
	ctx := context.Background()
	st, s, lib, root := libraryFixture(t, "movies", map[string]string{
		"The Matrix (1999)/The Matrix (1999).mkv":                    "original version",
		"The Matrix (1999)/The Matrix (1999) {edition-Extended}.mp4": "extended version",
		"The Matrix (1999)/poster.jpg":                               "synthetic poster",
		"The Matrix (1999)/movie.nfo":                                `<movie><uniqueid type="tmdb">603</uniqueid></movie>`,
		"Suspiria (1977)/Suspiria (1977).mkv":                        "original film",
		"Suspiria (2018)/Suspiria (2018).mkv":                        "remake",
	})
	for range 3 {
		if err := s.scan(ctx, lib); err != nil {
			t.Fatal(err)
		}
	}
	items, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "movie"})
	if err != nil || len(items) != 3 {
		t.Fatalf("expected versions grouped and remakes separate: %v %v", items, err)
	}
	var matrix *store.Item
	years := map[int]bool{}
	for i := range items {
		if items[i].TMDBID == 603 {
			matrix = &items[i]
		}
		years[items[i].Year] = true
	}
	if matrix == nil || !years[1977] || !years[2018] {
		t.Fatalf("NFO/remake identity wrong: %v", items)
	}
	if matrix.LocalPoster != filepath.Join(root, "The Matrix (1999)", "poster.jpg") {
		t.Fatalf("poster=%q", matrix.LocalPoster)
	}
	files, err := st.ItemFiles(ctx, matrix.ID)
	if err != nil || len(files) != 2 {
		t.Fatalf("versions=%v err=%v", files, err)
	}
	if store.EditionOf(files[0].Path) == "" && store.EditionOf(files[1].Path) == "" {
		t.Fatal("extended edition unrecognised")
	}
}
