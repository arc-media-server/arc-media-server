package importer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"ams/internal/store"
)

// present gives fixture titles files, so playlists list them (only present titles are shown).
func present(t *testing.T, st *store.Store, ids ...string) {
	t.Helper()
	ctx := context.Background()
	for i, id := range ids {
		it, _ := st.GetItem(ctx, id)
		fid, err := st.SaveFile(ctx, &store.MediaFile{LibraryID: it.LibraryID, Path: "/media/" + id + strconv.Itoa(i) + ".mkv", Size: 1, ModTime: 1}, 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetFileItems(ctx, fid, []string{id}, 0); err != nil {
			t.Fatal(err)
		}
	}
}

// From Plex: the signed-in account's plex.tv watchlist and playlists go to its profile, and the
// server's collections into the libraries their titles are in. Importing again adds nothing twice,
// and what Arc doesn't have is listed.
func TestPlexLists(t *testing.T) {
	ctx := context.Background()
	st, user, movie, episode := library(t)
	present(t, st, movie, episode)
	series, _ := st.FindImported(ctx, store.ImportRef{Kind: "series", TVDB: 78874})
	if series == "" {
		t.Fatal("the fixture show isn't found by its TheTVDB id")
	}
	movieMeta := map[string]any{"ratingKey": "10", "type": "movie", "title": "Inception", "Guid": []map[string]string{{"id": "tmdb://27205"}}}
	showMeta := map[string]any{"ratingKey": "20", "type": "show", "title": "Firefly", "Guid": []map[string]string{{"id": "tvdb://78874"}}}
	episodeMeta := map[string]any{"ratingKey": "21", "type": "episode", "title": "The Train Job", "grandparentRatingKey": "20",
		"grandparentTitle": "Firefly", "parentIndex": 1, "index": 2, "guid": "com.plexapp.agents.thetvdb://78874/1/2?lang=en"}
	missing := map[string]any{"ratingKey": "30", "type": "movie", "title": "Not In Arc", "Guid": []map[string]string{{"id": "tmdb://1"}}}
	list := func(items ...map[string]any) any {
		return map[string]any{"MediaContainer": map[string]any{"Metadata": items}}
	}
	srv := serve(t, map[string]any{
		"GET /accounts": map[string]any{"MediaContainer": map[string]any{"Account": []map[string]any{{"id": 1, "name": "Jonza"}, {"id": 5, "name": "Sam"}}}},
		"GET /library/sections": map[string]any{"MediaContainer": map[string]any{"Directory": []map[string]any{
			{"key": "1", "type": "movie", "title": "Films"}, {"key": "2", "type": "show", "title": "TV"}}}},
		"GET /library/sections/1/all":          list(movieMeta),
		"GET /library/sections/2/all":          list(showMeta, episodeMeta),
		"GET /status/sessions/history/all":     list(),
		"GET /playlists":                       list(map[string]any{"ratingKey": "p1", "type": "playlist", "title": "Rewatch"}),
		"GET /playlists/p1/items":              list(episodeMeta, movieMeta, missing),
		"GET /library/sections/1/collections":  list(map[string]any{"ratingKey": "c1", "type": "collection", "title": "Mind Benders"}),
		"GET /library/sections/2/collections":  list(map[string]any{"ratingKey": "c2", "type": "collection", "title": "Space Westerns"}),
		"GET /library/collections/c1/children": list(movieMeta, missing),
		"GET /library/collections/c2/children": list(showMeta),
		"GET /library/sections/watchlist/all":  list(showMeta, missing),
	})
	// plex.tv's watchlist service, served by the same stand-in; it must get the account's token.
	t.Setenv("AMS_PLEX_DISCOVER_URL", srv.URL)
	p := NewPlexDirect(srv.Client(), "test", srv.URL, "server-token")
	p.SetAccountToken("account-token")
	accounts, _ := p.Accounts(ctx)
	sam, _ := st.CreateUser(ctx, "Sam", false, "teal", "", "")
	res, lists, err := Run(ctx, st, p, accounts, map[string]string{"1": user, "5": sam.ID}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	owner := res[0]
	if owner.ListsError != "" || owner.Watchlist != 1 || owner.Playlists != 1 || owner.PlaylistItems != 2 || owner.ListsNotFound != 2 {
		t.Errorf("owner's lists = %+v", owner)
	}
	if !strings.Contains(strings.Join(owner.Missing, "|"), "Not In Arc") {
		t.Errorf("titles Arc doesn't have aren't listed: %v", owner.Missing)
	}
	// Plex shows other accounts' playlists and watchlists only to them.
	if res[1].Watchlist != 0 || res[1].Playlists != 0 {
		t.Errorf("another account's lists were read: %+v", res[1])
	}
	if ud, _ := st.UserDataFor(ctx, user, []string{series}); !ud[series].Watchlisted {
		t.Error("the watchlisted show isn't on the profile's watchlist")
	}
	pls, _ := st.Playlists(ctx, user, store.Access{})
	if len(pls) != 1 || pls[0].Title != "Rewatch" {
		t.Fatalf("playlists = %+v", pls)
	}
	_, items, _ := st.PlaylistEntries(ctx, pls[0].ID, store.Access{})
	if len(items) != 2 || items[0].ID != episode || items[1].ID != movie {
		t.Errorf("playlist order = %+v", items)
	}
	if lists.Error != "" || lists.Collections != 2 || lists.CollectionItems != 2 || lists.NotFound != 1 {
		t.Errorf("collections = %+v", lists)
	}

	// Again: nothing is duplicated.
	res, lists, _ = Run(ctx, st, p, accounts, map[string]string{"1": user}, nil, nil)
	if res[0].PlaylistItems != 0 || lists.CollectionItems != 0 {
		t.Errorf("second import added again: %+v %+v", res[0], lists)
	}
	if pls, _ := st.Playlists(ctx, user, store.Access{}); len(pls) != 1 {
		t.Errorf("playlist made twice: %d", len(pls))
	}
}

// A collections failure doesn't lose the watch state that came over.
func TestListsFailureKeepsWatchState(t *testing.T) {
	ctx := context.Background()
	st, user, movie, _ := library(t)
	srv := serve(t, map[string]any{
		"GET /accounts":         map[string]any{"MediaContainer": map[string]any{"Account": []map[string]any{{"id": 1, "name": "Jonza"}}}},
		"GET /library/sections": map[string]any{"MediaContainer": map[string]any{"Directory": []map[string]any{{"key": "1", "type": "movie", "title": "Films"}}}},
		"GET /library/sections/1/all": map[string]any{"MediaContainer": map[string]any{"Metadata": []map[string]any{
			{"ratingKey": "10", "type": "movie", "title": "Inception", "Guid": []map[string]string{{"id": "tmdb://27205"}}, "viewCount": 1}}}},
		"GET /status/sessions/history/all": map[string]any{"MediaContainer": map[string]any{}},
	})
	p := NewPlexDirect(srv.Client(), "test", srv.URL, "token")
	accounts, _ := p.Accounts(ctx)
	res, lists, err := Run(ctx, st, p, accounts, map[string]string{"1": user}, nil, nil)
	if err != nil || res[0].Matched != 1 {
		t.Fatalf("watch state import = %+v, %v", res, err)
	}
	if res[0].ListsError == "" || lists.Error == "" {
		t.Errorf("list failures aren't reported: %+v %+v", res[0], lists)
	}
	if ud, _ := st.UserDataFor(ctx, user, []string{movie}); !ud[movie].Played {
		t.Error("watch state was lost")
	}
}

// From Jellyfin: each user's favourites become their watchlist and their playlists come over;
// the server's box sets become collections.
func TestJellyfinLists(t *testing.T) {
	ctx := context.Background()
	st, user, movie, episode := library(t)
	present(t, st, movie, episode)
	series, _ := st.FindImported(ctx, store.ImportRef{Kind: "series", TVDB: 78874})
	show := map[string]any{"Id": "s1", "Name": "Firefly", "Type": "Series", "ProviderIds": map[string]string{"Tvdb": "78874"}}
	film := map[string]any{"Id": "m1", "Name": "Inception", "Type": "Movie", "ProviderIds": map[string]string{"Tmdb": "27205"}}
	ep := map[string]any{"Id": "e1", "Name": "The Train Job", "Type": "Episode", "SeriesId": "s1", "SeriesName": "Firefly",
		"ParentIndexNumber": 1, "IndexNumber": 2, "ProviderIds": map[string]string{}}
	page := func(items ...map[string]any) any {
		return map[string]any{"TotalRecordCount": len(items), "Items": items}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var out any
		switch {
		case r.URL.Path == "/Users/AuthenticateByName":
			out = map[string]any{"AccessToken": "abc", "User": map[string]any{"Id": "admin", "Policy": map[string]any{"IsAdministrator": true}}}
		case r.URL.Path == "/Users":
			out = []map[string]string{{"Id": "u1", "Name": "Jo"}}
		case r.URL.Path == "/Playlists/pl1/Items" && q.Get("UserId") == "u1":
			out = page(ep, film)
		case r.URL.Path == "/Users/admin/Items" && q.Get("IncludeItemTypes") == "BoxSet":
			out = page(map[string]any{"Id": "b1", "Name": "Nolan", "Type": "BoxSet"})
		case r.URL.Path == "/Users/admin/Items" && q.Get("ParentId") == "b1":
			out = page(film)
		case r.URL.Path == "/Users/u1/Items" && q.Get("Filters") == "IsFavorite":
			out = page(show)
		case r.URL.Path == "/Users/u1/Items" && q.Get("IncludeItemTypes") == "Playlist":
			out = page(map[string]any{"Id": "pl1", "Name": "Weekend", "Type": "Playlist"})
		case r.URL.Path == "/Users/u1/Items" && q.Get("IncludeItemTypes") == "Series":
			out = page(show)
		case r.URL.Path == "/Users/u1/Items":
			out = page() // nothing watched
		default:
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()
	j, err := ConnectJellyfin(ctx, srv.Client(), srv.URL, "dev", "admin", "pw")
	if err != nil {
		t.Fatal(err)
	}
	accounts, _ := j.Accounts(ctx)
	res, lists, err := Run(ctx, st, j, accounts, map[string]string{"u1": user}, nil, nil)
	if err != nil || res[0].ListsError != "" || res[0].Watchlist != 1 || res[0].PlaylistItems != 2 || lists.CollectionItems != 1 {
		t.Fatalf("results = %+v %+v, %v", res, lists, err)
	}
	if ud, _ := st.UserDataFor(ctx, user, []string{series}); !ud[series].Watchlisted {
		t.Error("the favourite show isn't on the watchlist")
	}
	pls, _ := st.Playlists(ctx, user, store.Access{})
	_, items, _ := st.PlaylistEntries(ctx, pls[0].ID, store.Access{})
	if len(items) != 2 || items[0].ID != episode || items[1].ID != movie {
		t.Errorf("playlist = %+v", items)
	}
}
