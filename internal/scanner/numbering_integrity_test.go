package scanner

import (
	"context"
	"fmt"
	"testing"

	"ams/internal/store"
)

func TestAbsoluteAndDateFilesUseCachedEpisodeIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, file, source string
		number             int
		date               string
	}{
		{"anime absolute", "One Piece/[SubGroup] One Piece - 012v2.mkv", "tvdb_absolute", 12, ""},
		{"date based", "One Piece/One Piece 2024-03-05.mkv", "tmdb_aired", 1, "2024-03-05"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			st, s, lib, _ := libraryFixture(t, "shows", map[string]string{tc.file: "synthetic episode"})
			show, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "series", Title: "One Piece"}, seriesMatchKey(MovieName{Title: "One Piece"}))
			if err != nil {
				t.Fatal(err)
			}
			key := episodeKey(show, "tmdb", 100)
			id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: lib, Kind: "episode", ParentID: show, Title: "Known episode", TMDBID: 100}, key)
			if err != nil {
				t.Fatal(err)
			}
			if err := st.UpdateMetadata(ctx, &store.Item{ID: id, Title: "Known episode", ReleaseDate: tc.date, MetadataState: "matched", TMDBID: 100}); err != nil {
				t.Fatal(err)
			}
			ord, err := st.UpsertOrdering(ctx, show, tc.source, "", "Cached order", "")
			if err != nil {
				t.Fatal(err)
			}
			if err := st.AddEntry(ctx, ord, id, 1, tc.number); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				if err := s.scan(ctx, lib); err != nil {
					t.Fatal(err)
				}
			}
			kids, err := st.Children(ctx, show)
			if err != nil || len(kids) != 1 || kids[0].ID != id {
				t.Fatalf("created duplicate or changed identity: %v %v", kids, err)
			}
			files, err := st.ItemFiles(ctx, id)
			if err != nil || len(files) != 1 || files[0].Missing {
				t.Fatalf("cached episode not linked: %v %v", files, err)
			}
		})
	}
}

// Measures scanner grouping/storage work independently of provider latency and probing.
// Run explicitly with -run '^$' -bench BenchmarkLibrary1800 -benchtime=1x -benchmem.
func BenchmarkLibrary1800(b *testing.B) {
	files := make(map[string]string, 1800)
	for i := range 1800 {
		files[fmt.Sprintf("Fixture %04d (2000)/Fixture %04d (2000).mkv", i, i)] = "seeded read file"
	}
	_, s, lib, _ := libraryFixture(b, "movies", files)
	b.ResetTimer()
	for range b.N {
		if err := s.scan(context.Background(), lib); err != nil {
			b.Fatal(err)
		}
	}
}
