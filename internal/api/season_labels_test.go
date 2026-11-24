package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"ams/internal/store"
)

func TestSeasonLabelsInheritAndPreserveManualNames(t *testing.T) {
	s, viewer := securityServer(t)
	ctx := t.Context()
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	id, err := s.store.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", nil)
	check(err)
	library, err := s.store.GetLibrary(ctx, id)
	check(err)
	show, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: id, Kind: "series", Title: "Show"}, "show")
	check(err)
	order, err := s.store.UpsertOrdering(ctx, show, "tmdb_group", "fixture", "Cours", "")
	check(err)
	var entries []store.OrderingEntry
	for i, name := range []string{"Special", "First episode"} {
		episode, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: id, Kind: "episode", ParentID: show, Title: name}, name)
		check(err)
		entries = append(entries, store.OrderingEntry{EpisodeID: episode, Season: i, Episode: 1})
	}
	check(s.store.ReplaceOrdering(ctx, order, map[int]string{0: "Provider specials", 1: "Cour One"}, nil, entries))
	check(s.store.SetDisplayOrdering(ctx, show, "", order))
	assert := func(name, metadataName string) {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1/series/"+show+"/seasons?missing=1", nil)
		r.SetPathValue("id", show)
		r = r.WithContext(context.WithValue(r.Context(), userKey, viewer))
		w := httptest.NewRecorder()
		s.seasons(w, r)
		var res struct{ Seasons []seasonDTO }
		if w.Code != 200 {
			t.Fatalf("get seasons: %d %s", w.Code, w.Body.String())
		}
		check(json.Unmarshal(w.Body.Bytes(), &res))
		if len(res.Seasons) != 2 || res.Seasons[0].Name != name || res.Seasons[0].MetadataName != metadataName ||
			res.Seasons[1].Name != "Specials" || len(res.Seasons[0].Episodes) != 1 {
			t.Fatalf("expected %q and Specials, got %+v", name, res.Seasons)
		}
		episode := res.Seasons[0].Episodes[0]
		if episode.ID != entries[1].EpisodeID || episode.SeasonNumber != 1 || episode.EpisodeNumber != 1 {
			t.Fatalf("display label changed episode identity or numbering: %+v", episode)
		}
	}
	assert("Season 1", "Cour One")
	named := *library
	named.SeasonLabelMode = "name"
	check(s.store.SaveLibrarySettings(ctx, *library, named, false))
	assert("Cour One", "")
	number := "number"
	check(s.store.EditItem(ctx, show, store.ItemEdit{SeasonLabelMode: &number}))
	assert("Season 1", "Cour One")
	inherit := ""
	check(s.store.EditItem(ctx, show, store.ItemEdit{SeasonLabelMode: &inherit}))
	assert("Cour One", "")
	check(s.store.SaveLibrarySettings(ctx, named, *library, false))
	check(s.store.EditItem(ctx, show, store.ItemEdit{SeasonLabelMode: &named.SeasonLabelMode}))
	assert("Cour One", "")
	check(s.store.EditItem(ctx, show, store.ItemEdit{SeasonLabelMode: &inherit}))
	manual := "My Season"
	check(s.store.EditSeason(ctx, order, 1, &manual, nil, nil))
	assert("My Season", "")
}
