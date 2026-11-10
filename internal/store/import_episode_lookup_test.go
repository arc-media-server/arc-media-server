package store

import (
	"strings"
	"testing"
)

func TestImportedEpisodeUsesProviderIndex(t *testing.T) {
	s := openTest(t)
	for _, provider := range []string{"tvdb", "tmdb"} {
		t.Run(provider, func(t *testing.T) {
			rows, err := s.db.QueryContext(t.Context(), "EXPLAIN QUERY PLAN "+importedEpisodeQuery(provider), 777)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			indexed := false
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				t.Log(detail)
				if strings.Contains(detail, "SEARCH items USING INDEX items_lookup_"+provider) {
					indexed = true
				}
				if strings.Contains(detail, "SCAN items") {
					t.Error("episode import scans catalog instead of looking up provider ID")
				}
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !indexed {
				t.Error("episode provider-ID index was not used")
			}
		})
	}
}

func TestImportedEpisodeFindsMappedProviderID(t *testing.T) {
	s := openTest(t)
	ctx := t.Context()
	lib, err := s.SyncLibrary(ctx, "Shows", "shows", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	// The file order is TMDB, but cross-provider mapping also knows the TVDB ID.
	id, _, err := s.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "episode", Title: "Mapped", TMDBID: 888, TVDBID: 777}, "ep:show:tmdb:888")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.FindImported(ctx, ImportRef{Kind: "episode", TVDB: 777})
	if err != nil || got != id {
		t.Fatalf("mapped provider got %q, want %q: %v", got, id, err)
	}
}
