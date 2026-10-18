package importer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"ams/internal/activity"
	"ams/internal/store"
)

// library makes an Arc library with Inception (TMDB 27205), and Firefly (TheTVDB 78874) with one
// episode keyed by TheTVDB id 297989 and numbered 1x2 in TheTVDB's aired order.
func library(t *testing.T) (st *store.Store, user, movie, episode string) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "ams.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	movies, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	shows, _ := st.SyncLibrary(ctx, "TV", "shows", "tvdb_aired", []string{"/t"})
	movie, _, _ = st.EnsureItem(ctx, &store.Item{LibraryID: movies, Kind: "movie", Title: "Inception", TMDBID: 27205}, "tmdb:27205")
	series, _, _ := st.EnsureItem(ctx, &store.Item{LibraryID: shows, Kind: "series", Title: "Firefly", TVDBID: 78874}, "series:firefly")
	episode, _, _ = st.EnsureItem(ctx, &store.Item{LibraryID: shows, Kind: "episode", ParentID: series, Title: "The Train Job"},
		"ep:"+series+":tvdb:297989")
	other, _, _ := st.EnsureItem(ctx, &store.Item{LibraryID: shows, Kind: "episode", ParentID: series, Title: "Bushwhacked"},
		"ep:"+series+":tvdb:297990")
	ord, _ := st.UpsertOrdering(ctx, series, "tvdb_aired", "", "Aired Order", "TVDB")
	st.ReplaceOrdering(ctx, ord, map[int]string{1: "Season 1"}, nil, []store.OrderingEntry{
		{EpisodeID: episode, Season: 1, Episode: 2}, {EpisodeID: other, Season: 1, Episode: 3}})
	u, err := st.CreateUser(ctx, "Jo", true, "blue", "x", "")
	if err != nil {
		t.Fatal(err)
	}
	return st, u.ID, movie, episode
}

func serve(t *testing.T, routes map[string]any) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPlexImport(t *testing.T) {
	ctx := context.Background()
	st, user, movie, episode := library(t)
	srv := serve(t, map[string]any{
		"GET /accounts": map[string]any{"MediaContainer": map[string]any{"Account": []map[string]any{
			{"id": 0, "name": ""}, {"id": 1, "name": "Jonza"}, {"id": 5, "name": "Sam"}}}},
		"GET /library/sections": map[string]any{"MediaContainer": map[string]any{"Directory": []map[string]any{
			{"key": "1", "type": "movie", "title": "Films"}, {"key": "2", "type": "show", "title": "TV"}}}},
		// One listing serves every type here: a movie, a show and an episode.
		"GET /library/sections/1/all": map[string]any{"MediaContainer": map[string]any{"Metadata": []map[string]any{
			{"ratingKey": "10", "title": "Inception", "Guid": []map[string]string{{"id": "tmdb://27205"}, {"id": "imdb://tt1375666"}},
				"viewOffset": 600000, "userRating": 8.0},
			{"ratingKey": "11", "title": "Not In Arc", "Guid": []map[string]string{{"id": "tmdb://1"}}, "viewCount": 1}}}},
		"GET /library/sections/2/all": map[string]any{"MediaContainer": map[string]any{"Metadata": []map[string]any{
			{"ratingKey": "20", "title": "Firefly", "Guid": []map[string]string{{"id": "tvdb://78874"}}},
			// An old-style guid: the show's TheTVDB id and the episode's number.
			{"ratingKey": "21", "title": "The Train Job", "grandparentRatingKey": "20", "grandparentTitle": "Firefly",
				"parentIndex": 1, "index": 2, "guid": "com.plexapp.agents.thetvdb://78874/1/2?lang=en", "viewCount": 2, "lastViewedAt": 1700000000}}}},
		"GET /status/sessions/history/all": map[string]any{"MediaContainer": map[string]any{"Metadata": []map[string]any{
			{"ratingKey": "21", "viewedAt": 1700000000, "accountID": 1},
			{"ratingKey": "21", "viewedAt": 1600000000, "accountID": 1},
			{"ratingKey": "10", "viewedAt": 1650000000, "accountID": 5}}}},
	})
	p := NewPlexDirect(srv.Client(), "test", srv.URL, "token")
	accounts, err := p.Accounts(ctx)
	if err != nil || len(accounts) != 2 || !accounts[0].Owner {
		t.Fatalf("accounts = %+v, %v", accounts, err)
	}
	sam, _ := st.CreateUser(ctx, "Sam", false, "teal", "", "")
	res, err := Run(ctx, st, p, accounts, map[string]string{"1": user, "5": sam.ID}, nil, activity.New().Start("import", "test"))
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Matched != 2 || res[0].NotFound != 1 || res[0].History != 2 || res[1].Matched != 1 || res[1].History != 1 {
		t.Errorf("results = %+v", res)
	}
	ud, _ := st.UserDataFor(ctx, user, []string{movie, episode})
	if ud[movie].PositionMS != 600000 || ud[movie].Played || ud[movie].Rating != 8 {
		t.Errorf("owner's movie = %+v", ud[movie])
	}
	if !ud[episode].Played || ud[episode].PlayCount != 2 {
		t.Errorf("owner's episode = %+v", ud[episode])
	}
	if samData, _ := st.UserDataFor(ctx, sam.ID, []string{movie}); !samData[movie].Played {
		t.Errorf("Sam's movie from history = %+v", samData[movie])
	}
	// Importing again changes nothing.
	again, _ := Run(ctx, st, p, accounts, map[string]string{"1": user}, nil, nil)
	if again[0].History != 0 {
		t.Errorf("history added twice: %+v", again)
	}
}

func TestJellyfinImport(t *testing.T) {
	ctx := context.Background()
	st, user, movie, episode := library(t)
	items := func(r *http.Request) any {
		switch {
		case r.URL.Query().Get("IncludeItemTypes") == "Series":
			return map[string]any{"TotalRecordCount": 1, "Items": []map[string]any{
				{"Id": "s1", "Name": "Firefly", "Type": "Series", "ProviderIds": map[string]string{"Tvdb": "78874"}}}}
		case r.URL.Query().Get("Filters") == "IsPlayed":
			return map[string]any{"TotalRecordCount": 1, "Items": []map[string]any{
				{"Id": "e1", "Name": "Bushwhacked?", "Type": "Episode", "SeriesId": "s1", "SeriesName": "Firefly",
					"ParentIndexNumber": 1, "IndexNumber": 2, "ProviderIds": map[string]string{},
					"UserData": map[string]any{"Played": true, "PlayCount": 3, "LastPlayedDate": "2024-01-02T03:04:05.0000000Z"}}}}
		default:
			return map[string]any{"TotalRecordCount": 1, "Items": []map[string]any{
				{"Id": "m1", "Name": "Inception", "Type": "Movie", "ProviderIds": map[string]string{"Tmdb": "27205"},
					"UserData": map[string]any{"PlaybackPositionTicks": 12_000_000_000}}}}
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), `Client="Arc Media Server"`) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.URL.Path == "/Users/AuthenticateByName":
			json.NewEncoder(w).Encode(map[string]any{"AccessToken": "abc", "User": map[string]any{"Policy": map[string]any{"IsAdministrator": true}}})
		case r.URL.Path == "/Users":
			json.NewEncoder(w).Encode([]map[string]string{{"Id": "u1", "Name": "Jo"}})
		case r.URL.Path == "/Users/u1/Items":
			json.NewEncoder(w).Encode(items(r))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	j, err := ConnectJellyfin(ctx, srv.Client(), srv.URL, "dev", "admin", "pw")
	if err != nil {
		t.Fatal(err)
	}
	accounts, _ := j.Accounts(ctx)
	res, err := Run(ctx, st, j, accounts, map[string]string{"u1": user}, nil, nil)
	if err != nil || res[0].Matched != 2 {
		t.Fatalf("results = %+v, %v", res, err)
	}
	ud, _ := st.UserDataFor(ctx, user, []string{movie, episode})
	if ud[movie].PositionMS != 1_200_000 {
		t.Errorf("movie resume point = %d", ud[movie].PositionMS)
	}
	if !ud[episode].Played || ud[episode].PlayCount != 3 {
		t.Errorf("episode = %+v", ud[episode])
	}
}
