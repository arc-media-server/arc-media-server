package scanner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"ams/internal/metadata"
	"ams/internal/store"
)

func TestUnequalCataloguesFollowFileOrderWithoutDiscovery(t *testing.T) {
	// Redirect every connection made by newly constructed provider clients to the
	// local TLS fixture. No DNS lookup or real-provider connection is possible.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var body string
		switch r.URL.Path {
		case "/v4/login":
			body = `{"data":{"token":"fixture-token"}}`
		case "/v4/series/404/extended":
			body = `{"data":{"id":404,"name":"Example","year":"2020","firstAired":"2020-01-01","overview":"TVDB overview","seasonTypes":[{"type":"official"}],"remoteIds":[{"type":12,"id":"303"}]}}`
		case "/v4/series/404/translations/eng":
			body = `{"data":{}}`
		case "/v4/series/404/episodes/official/eng":
			body = `{"data": {"episodes": [{"id": 201, "name": "Episode 1", "seasonNumber": 3, "number": 1, "aired": "2020-01-01", "overview": "TVDB episode"}, {"id": 202, "name": "Episode 2", "seasonNumber": 3, "number": 2, "aired": "2020-01-02", "overview": "TVDB episode"}, {"id": 203, "name": "Episode 3", "seasonNumber": 3, "number": 3, "aired": "2020-01-03", "overview": "TVDB episode"}, {"id": 204, "name": "Episode 4", "seasonNumber": 3, "number": 4, "aired": "2020-01-04", "overview": "TVDB episode"}, {"id": 205, "name": "Episode 5", "seasonNumber": 3, "number": 5, "aired": "2020-01-05", "overview": "TVDB episode"}, {"id": 206, "name": "Episode 6", "seasonNumber": 3, "number": 6, "aired": "2020-01-06", "overview": "TVDB episode"}, {"id": 207, "name": "Episode 7", "seasonNumber": 3, "number": 7, "aired": "2020-01-07", "overview": "TVDB episode"}, {"id": 208, "name": "Episode 8", "seasonNumber": 3, "number": 8, "aired": "2020-01-08", "overview": "TVDB episode"}, {"id": 209, "name": "Episode 9", "seasonNumber": 3, "number": 9, "aired": "2020-01-09", "overview": "TVDB episode"}, {"id": 210, "name": "Episode 10", "seasonNumber": 3, "number": 10, "aired": "2020-01-10", "overview": "TVDB episode"}, {"id": 211, "name": "Episode 11", "seasonNumber": 3, "number": 11, "aired": "2020-01-11", "overview": "TVDB episode"}, {"id": 212, "name": "Episode 12", "seasonNumber": 3, "number": 12, "aired": "2020-01-12", "overview": "TVDB episode"}, {"id": 213, "name": "Episode 13", "seasonNumber": 3, "number": 13, "aired": "2020-01-13", "overview": "TVDB episode"}, {"id": 214, "name": "Episode 14", "seasonNumber": 3, "number": 14, "aired": "2020-01-14", "overview": "TVDB episode"}, {"id": 215, "name": "Episode 15", "seasonNumber": 3, "number": 15, "aired": "2020-01-15", "overview": "TVDB episode"}, {"id": 216, "name": "Episode 16", "seasonNumber": 3, "number": 16, "aired": "2020-01-16", "overview": "TVDB episode"}]}, "links": {"next": null}}`
		case "/3/tv/303":
			if r.URL.Query().Get("append_to_response") == "season/3" {
				body = `{"season/3": {"episodes": [{"id": 101, "name": "Episode 1", "season_number": 3, "episode_number": 1, "air_date": "2020-01-01", "overview": "TMDB episode", "runtime": 43}, {"id": 102, "name": "Episode 2", "season_number": 3, "episode_number": 2, "air_date": "2020-01-02", "overview": "TMDB episode", "runtime": 43}, {"id": 103, "name": "Episode 3", "season_number": 3, "episode_number": 3, "air_date": "2020-01-03", "overview": "TMDB episode", "runtime": 43}, {"id": 104, "name": "Episode 4", "season_number": 3, "episode_number": 4, "air_date": "2020-01-04", "overview": "TMDB episode", "runtime": 43}, {"id": 105, "name": "Episode 5", "season_number": 3, "episode_number": 5, "air_date": "2020-01-05", "overview": "TMDB episode", "runtime": 43}, {"id": 106, "name": "Episode 6", "season_number": 3, "episode_number": 6, "air_date": "2020-01-06", "overview": "TMDB episode", "runtime": 43}, {"id": 107, "name": "Episode 7", "season_number": 3, "episode_number": 7, "air_date": "2020-01-07", "overview": "TMDB episode", "runtime": 43}, {"id": 108, "name": "Episode 8", "season_number": 3, "episode_number": 8, "air_date": "2020-01-08", "overview": "TMDB episode", "runtime": 43}, {"id": 109, "name": "Episode 9", "season_number": 3, "episode_number": 9, "air_date": "2020-01-09", "overview": "TMDB episode", "runtime": 43}, {"id": 110, "name": "Episode 10", "season_number": 3, "episode_number": 10, "air_date": "2020-01-10", "overview": "TMDB episode", "runtime": 43}, {"id": 111, "name": "Episode 11", "season_number": 3, "episode_number": 11, "air_date": "2020-01-11", "overview": "TMDB episode", "runtime": 43}]}}`
			} else {
				body = `{"id":303,"name":"Example","original_name":"Example","first_air_date":"2020-01-01","overview":"","external_ids":{"tvdb_id":404},"seasons":[{"season_number":3,"name":"Season 3"}]}`
			}
		default:
			t.Errorf("unexpected fixture request: %s", r.URL.Path)
			http.Error(w, "unexpected request", 500)
			return
		}
		fmt.Fprint(w, body)
	}))
	defer srv.Close()
	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })

	ctx := context.Background()
	files := map[string]string{}
	for i := 1; i <= 16; i++ {
		files[fmt.Sprintf("Example/Example S03E%02d.mkv", i)] = "episode"
	}
	st, s, lib, _ := libraryFixture(t, "shows", files)
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	shows, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	if err != nil || len(shows) != 1 {
		t.Fatal(err)
	}
	show := shows[0].ID
	if err := st.AdoptIDs(ctx, show, 303, 404, ""); err != nil {
		t.Fatal(err)
	}
	s.tmdb = metadata.NewTMDB("fixture-only")
	s.tvdb = metadata.NewTVDB("fixture-only")
	library, err := st.GetLibrary(ctx, lib)
	if err != nil {
		t.Fatal(err)
	}

	original, err := st.GetItemByKey(ctx, lib, fileEpisodeKey(show, false, 3, 12))
	if err != nil || original == nil {
		t.Fatal("missing file-only identity")
	}
	title := "My extra episode"
	if err := st.EditItem(ctx, original.ID, store.ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	viewer, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, viewer.ID, original.ID, 70000, 120000); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []struct {
		name, details, source string
		want                  int
	}{
		{"TMDB numbering/details, TVDB view", "", "tmdb_aired", 11},
		{"TVDB details only, TMDB numbering", "tvdb", "tmdb_aired", 11},
		{"TVDB numbering, TMDB details", "", "tvdb_aired", 16},
	} {
		if err := st.SetLibraryDetailsProvider(ctx, lib, phase.details); err != nil {
			t.Fatal(err)
		}
		if err := st.SetSourceOrdering(ctx, show, phase.source); err != nil {
			t.Fatal(err)
		}
		library, err = st.GetLibrary(ctx, lib)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.RefreshSeries(ctx, library, show, true, nil); err != nil {
			t.Fatal(err)
		}
		if s.Status(lib).FilesSeen != 0 || s.Status(lib).FilesProbed != 0 {
			t.Fatal("scoped refresh discovered/probed files")
		}
		orders, err := st.ListOrderings(ctx, show)
		if err != nil {
			t.Fatal(err)
		}
		placed := map[string]bool{}
		for _, o := range orders {
			if o.Source == "tvdb_aired" {
				entries, err := st.DisplayEntries(ctx, o.ID, "")
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range entries {
					placed[e.EpisodeID] = true
				}
			}
		}
		available, err := st.AvailableChildren(ctx, show)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: present=%d TVDB-view-placed=%d unplaced=%d", phase.name, len(available), len(placed), len(available)-len(placed))
		if len(placed) != phase.want || len(available) != 16 {
			t.Fatalf("unexpected reproduction: placed=%d available=%d", len(placed), len(available))
		}
		saved, err := st.GetItem(ctx, original.ID)
		if err != nil || saved == nil || saved.Title != title {
			t.Fatal("identity/manual edit changed")
		}
		history, err := st.UserDataFor(ctx, viewer.ID, []string{original.ID})
		if err != nil || history[original.ID].PositionMS != 70000 {
			t.Fatal("watch history changed")
		}

	}
}
