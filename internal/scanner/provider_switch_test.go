package scanner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ams/internal/metadata"
	"ams/internal/store"
)

func TestFullProviderRefreshAndSourceSwitchPreservesEpisode(t *testing.T) {
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
			body = `{"data":{"episodes":[{"id":200,"name":"Pilot","seasonNumber":1,"number":1,"aired":"2020-01-01","overview":""}]},"links":{"next":null}}`
		case "/3/tv/303":
			if r.URL.Query().Get("append_to_response") == "season/1" {
				body = `{"season/1":{"episodes":[{"id":100,"name":"Pilot","season_number":1,"episode_number":1,"air_date":"2020-01-01","overview":"TMDB episode","runtime":43,"still_path":"/tmdb-still.jpg"}]}}`
			} else {
				body = `{"id":303,"name":"Example","original_name":"Example","first_air_date":"2020-01-01","overview":"","external_ids":{"tvdb_id":404},"seasons":[{"season_number":1,"name":"Season 1"}]}`
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
	st, s, lib, _ := libraryFixture(t, "shows", map[string]string{"Example/Example S01E01.mkv": "episode"})
	if err := s.scan(ctx, lib); err != nil {
		t.Fatal(err)
	}
	shows, _, err := st.ListItems(ctx, store.ItemQuery{LibraryID: lib, Kind: "series"})
	if err != nil || len(shows) != 1 {
		t.Fatalf("shows=%v err=%v", shows, err)
	}
	show := shows[0].ID
	kids, err := st.Children(ctx, show)
	if err != nil || len(kids) != 1 {
		t.Fatalf("episodes=%v err=%v", kids, err)
	}
	id := kids[0].ID
	files, err := st.ItemFiles(ctx, id)
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%v err=%v", files, err)
	}
	fid := files[0].ID
	title := "My pilot"
	if err := st.EditItem(ctx, id, store.ItemEdit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	orders, err := st.ListOrderings(ctx, show)
	if err != nil || len(orders) != 1 {
		t.Fatalf("orders=%v err=%v", orders, err)
	}
	custom, err := st.CopyOrdering(ctx, &orders[0], "My display order")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetEntry(ctx, custom, id, 3, 7); err != nil {
		t.Fatal(err)
	}
	if err := st.SetDisplayOrdering(ctx, show, "", custom); err != nil {
		t.Fatal(err)
	}
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReportProgress(ctx, u.ID, id, 120000, 3600000); err != nil {
		t.Fatal(err)
	}
	if err := st.AdoptIDs(ctx, show, 303, 404, ""); err != nil {
		t.Fatal(err)
	}
	s.tmdb = metadata.NewTMDB("fixture-only")
	s.tvdb = metadata.NewTVDB("fixture-only")
	library, err := st.GetLibrary(ctx, lib)
	if err != nil {
		t.Fatal(err)
	}
	for index, source := range []string{"tvdb_aired", "tmdb_aired", "tvdb_aired"} {
		if err := st.SetSourceOrdering(ctx, show, source); err != nil {
			t.Fatal(err)
		}
		if index == 1 {
			if err := st.SetLibraryLanguage(ctx, lib, ""); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := s.SyncSeriesMetadata(ctx, library, show, false); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.scan(ctx, lib); err != nil {
			t.Fatal(err)
		}
		provider, number := "tvdb", int64(200)
		if strings.HasPrefix(source, "tmdb") {
			provider, number = "tmdb", 100
		}
		it, err := st.GetItemByKey(ctx, lib, episodeKey(show, provider, number))
		if err != nil || it == nil || it.ID != id || it.Title != title {
			t.Fatalf("%s lost identity/edit: %+v %v", source, it, err)
		}
		// The owner retained one provider per metadata scope. Mapping the other
		// provider must not silently fill numbering-provider episode gaps (#104).
		wantOverview, wantStill, wantRuntime := "", "", 0
		if provider == "tmdb" {
			wantOverview, wantStill, wantRuntime = "TMDB episode", "/tmdb-still.jpg", 43
		}
		if it.Overview != wantOverview || it.PosterPath != wantStill || it.RuntimeMin != wantRuntime {
			t.Fatalf("%s mixed episode providers: %+v", source, it)
		}
		series, err := st.GetItem(ctx, show)
		if err != nil || series.Overview != "" {
			t.Fatalf("series TMDB gap backfilled from TVDB: %+v %v", series, err)
		}
		children, err := st.Children(ctx, show)
		if err != nil || len(children) != 1 {
			t.Fatalf("%s duplicated episode: %+v %v", source, children, err)
		}
		linked, err := st.ItemFiles(ctx, id)
		if err != nil || len(linked) != 1 || linked[0].ID != fid {
			t.Fatalf("%s lost file: %+v %v", source, linked, err)
		}
		data, err := st.UserDataFor(ctx, u.ID, []string{id})
		if err != nil || data[id].PositionMS != 120000 {
			t.Fatalf("%s lost resume: %+v %v", source, data, err)
		}
		entries, err := st.OrderingEntries(ctx, custom)
		if err != nil || len(entries) != 1 || entries[0].EpisodeID != id || !entries[0].Locked || entries[0].Season != 3 || entries[0].Episode != 7 {
			t.Fatalf("%s lost manual placement: %+v %v", source, entries, err)
		}
		got, err := st.DisplayOrdering(ctx, show, "", source)
		if err != nil || got != custom {
			t.Fatalf("%s lost display order: %d %v", source, got, err)
		}
	}
}
