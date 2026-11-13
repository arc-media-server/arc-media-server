package api

import (
	"ams/internal/store"
	"context"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRecommendationDataReusesCardsAndIsolatesProfiles(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	lib, err := s.store.SyncLibrary(ctx, "TV", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	sid, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Show"}, "show")
	if err != nil {
		t.Fatal(err)
	}
	eid, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "episode", Title: "Episode", ParentID: sid}, "episode")
	if err != nil {
		t.Fatal(err)
	}
	fid, err := s.store.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: "/fixture/episode.mkv"}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.SetFileItems(ctx, fid, []string{eid}, 0); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	data := s.itemData(r)
	card := itemDTO{Item: store.Item{ID: sid, Kind: "series"}}
	if err = data.fill([]*itemDTO{&card, &card}); err != nil {
		t.Fatal(err)
	}
	if card.UnwatchedCount == nil || *card.UnwatchedCount != 1 {
		t.Fatal("initial unwatched count", card.UnwatchedCount)
	}
	if err = s.store.SetPlayed(ctx, u.ID, []string{eid, sid}, true); err != nil {
		t.Fatal(err)
	}
	repeated := itemDTO{Item: card.Item}
	if err = data.fill([]*itemDTO{&repeated}); err != nil {
		t.Fatal(err)
	}
	if *repeated.UnwatchedCount != 1 || repeated.UserData.Played {
		t.Fatal("request-local values should be reused")
	}
	fresh := itemDTO{Item: card.Item}
	if err = s.itemData(r).fill([]*itemDTO{&fresh}); err != nil {
		t.Fatal(err)
	}
	if *fresh.UnwatchedCount != 0 || !fresh.UserData.Played {
		t.Fatal("next request must see updated state")
	}
	other, err := s.store.CreateUser(ctx, "Other", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	r = r.WithContext(context.WithValue(r.Context(), userKey, other))
	fresh = itemDTO{Item: card.Item}
	if err = s.itemData(r).fill([]*itemDTO{&fresh}); err != nil {
		t.Fatal(err)
	}
	if *fresh.UnwatchedCount != 1 || fresh.UserData.Played {
		t.Fatal("watch state leaked between profiles")
	}
	// Once every value is cached, no database access is required for repeated cards.
	if err = s.store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = data.fill([]*itemDTO{&repeated}); err != nil {
		t.Fatalf("cached cards queried the closed database: %v", err)
	}
}

func TestRecommendedSkipsHiddenFamiliesBeforeQueries(t *testing.T) {
	s, u := securityServer(t)
	ctx := t.Context()
	libID, err := s.store.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	lib, err := s.store.GetLibrary(ctx, libID)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	layout := []recommendedLook{}
	for _, kind := range recommendedTypes {
		layout = append(layout, recommendedLook{Type: kind, Hidden: true})
	}
	var logs strings.Builder
	s.log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// With layout already loaded, an all-hidden page needs no row or prerequisite queries.
	if err = s.store.Close(); err != nil {
		t.Fatal(err)
	}
	rows, err := s.recommendedRowsWithLayout(r, lib, 20, "", layout)
	if err != nil || len(rows) != 0 {
		t.Fatalf("hidden families did work: %v %v", rows, err)
	}
	if strings.Contains(logs.String(), "_query=") || strings.Contains(logs.String(), "featured_people=") {
		t.Fatal(logs.String())
	}
}
