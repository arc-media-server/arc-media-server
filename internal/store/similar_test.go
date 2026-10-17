package store

import (
	"context"
	"testing"
)

func TestSimilar(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	add := func(title string, tmdbID int64, genres ...string) *Item {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		st.db.ExecContext(ctx, `UPDATE items SET tmdb_id = ? WHERE id = ?`, tmdbID, id)
		st.SetItemDetails(ctx, id, genres, nil)
		it, _ := st.GetItem(ctx, id)
		return it
	}
	alien := add("Alien", 348, "Horror", "Science Fiction")
	aliens := add("Aliens", 679, "Action", "Science Fiction")
	thing := add("The Thing", 1091, "Horror", "Science Fiction")
	add("Notting Hill", 509, "Romance", "Comedy")

	// TMDB recommends Aliens: it comes first; The Thing shares both genres; Notting Hill nothing.
	st.SetRecommendations(ctx, alien.ID, []int64{679, 99999})
	got, err := st.Similar(ctx, alien, Access{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != aliens.ID || got[1].ID != thing.ID {
		names := []string{}
		for _, g := range got {
			names = append(names, g.Title)
		}
		t.Errorf("similar to Alien: %v", names)
	}
}
