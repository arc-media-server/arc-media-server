package api

import (
	"ams/internal/store"
	"context"
	"testing"
)

func TestEpisodeContextsCacheParentsAndMissingRowsPerRequest(t *testing.T) {
	s, _ := securityServer(t)
	ctx := t.Context()
	lib, err := s.store.SyncLibrary(ctx, "Episodes", "shows", "tmdb_aired", nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "Parent"}, "context-parent")
	if err != nil {
		t.Fatal(err)
	}
	ep, _, err := s.store.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "episode", ParentID: id, Title: "Episode"}, "context-episode")
	if err != nil {
		t.Fatal(err)
	}
	oid, err := s.store.UpsertOrdering(ctx, id, "tmdb_aired", "", "Aired", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.store.ReplaceOrdering(ctx, oid, nil, nil, []store.OrderingEntry{{EpisodeID: ep, Season: 2, Episode: 3}}); err != nil {
		t.Fatal(err)
	}
	cache := s.episodeContexts(ctx)
	first, err := cache.get(id)
	if err != nil || first.series == nil {
		t.Fatalf("parent: %+v %v", first, err)
	}
	if got := first.positions[ep]; got != [2]int{2, 3} {
		t.Fatalf("display position: %v", got)
	}
	missing, err := cache.get("missing-parent")
	if err != nil || missing.series != nil {
		t.Fatalf("missing: %+v %v", missing, err)
	}
	// Once the read connection is unavailable, already-resolved parents and missing rows
	// still use this request's snapshot. A new request must report the database failure.
	s.store.Close()
	again, err := cache.get(id)
	if err != nil || again.series != first.series || again.positions[ep] != [2]int{2, 3} {
		t.Fatal("parent was reloaded instead of cached", err)
	}
	if _, err := cache.get("missing-parent"); err != nil {
		t.Fatal("missing parent was not cached", err)
	}
	if _, err := s.episodeContexts(context.Background()).get(id); err == nil {
		t.Fatal("database error hidden as missing metadata")
	}
}
