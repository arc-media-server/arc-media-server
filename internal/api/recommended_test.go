package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ams/internal/store"
)

func TestRecommendedRows(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ams.db")
	st, err := store.Open(ctx, path, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, _ := st.SyncLibrary(ctx, "Films", "movies", "tmdb_aired", []string{"/films"})
	other, _ := st.SyncLibrary(ctx, "Other", "movies", "tmdb_aired", []string{"/other"})
	u, err := st.CreateUser(ctx, "Viewer", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	u.MaxAge = 12
	director := store.Credit{TMDBID: 578, Name: "Ridley Scott", Role: "director"}
	add := func(library int64, title, rating string) string {
		id, _, err := st.EnsureItem(ctx, &store.Item{LibraryID: library, Kind: "movie", Title: title}, title)
		if err != nil {
			t.Fatal(err)
		}
		file, _ := st.SaveFile(ctx, &store.MediaFile{LibraryID: library, Path: "/" + title + ".mkv"}, 1, 1)
		st.SetFileItems(ctx, file, []string{id}, 0)
		st.SetItemDetails(ctx, id, []string{"Drama"}, []store.Credit{director})
		st.EditItem(ctx, id, store.ItemEdit{ContentRating: &rating})
		return id
	}
	var ids []string
	for i := range 6 {
		ids = append(ids, add(lib, fmt.Sprintf("Film %d", i), "PG"))
	}
	adult := add(lib, "Adult", "18")
	elsewhere := add(other, "Elsewhere", "PG")
	// Film 0 part-way (and one in another library), Film 1 watched yesterday, Film 2 a year ago.
	st.ReportProgress(ctx, u.ID, ids[0], 600_000, 7_200_000)
	st.ReportProgress(ctx, u.ID, elsewhere, 600_000, 7_200_000)
	st.SetPlayed(ctx, u.ID, []string{ids[1], ids[2]}, true)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.Exec(`UPDATE user_item_data SET last_played_at = ? WHERE item_id = ?`, time.Now().Add(-365*24*time.Hour).Unix(), ids[2])

	r := httptest.NewRequest("GET", "/api/libraries/1/recommended", nil)
	r = r.WithContext(context.WithValue(r.Context(), userKey, u))
	s := &Server{store: st}
	l, _ := st.GetLibrary(ctx, lib)
	rows, err := s.recommendedRows(r, l)
	if err != nil {
		t.Fatal(err)
	}
	// A library starts without Rediscover, Top in Genre, the featured person and Top Unwatched.
	var types []string
	for _, row := range rows {
		types = append(types, row.Type)
	}
	if want := []string{"continue", "recently_released", "recently_added", "top_rated", "recently_watched", "more_in_genre"}; !slices.Equal(types, want) {
		t.Errorf("default rows %v, want %v", types, want)
	}
	// Turned on in Edit Recommended, they're built as before.
	all := `[{"type":"rediscover"},{"type":"top_in_genre"},{"type":"top_by_person"},{"type":"top_unwatched"}]`
	if err := st.SetRecommendedLayout(ctx, lib, all); err != nil {
		t.Fatal(err)
	}
	if rows, err = s.recommendedRows(r, l); err != nil {
		t.Fatal(err)
	}
	types = nil
	for _, row := range rows {
		filtered, err := s.recommendedRowsLimitTarget(r, l, recommendedLimit, row.ID)
		if err != nil || len(filtered) != 1 {
			t.Fatalf("target %s: %d rows, %v", row.Type, len(filtered), err)
		}
		want, _ := json.Marshal(row)
		got, _ := json.Marshal(filtered[0])
		if !bytes.Equal(got, want) {
			t.Errorf("target %s changed the full-view row", row.Type)
		}
		types = append(types, row.Type)
		for _, it := range row.Items {
			if it.ID == adult || it.ID == elsewhere {
				t.Errorf("%s row has %s, which the profile may not see here", row.Type, it.Title)
			}
		}
		switch row.Type {
		case "continue":
			if len(row.Items) != 1 || row.Items[0].ID != ids[0] {
				t.Errorf("continue watching here: %v", row.Items)
			}
		case "recently_watched":
			if len(row.Items) != 2 || row.Items[0].ID != ids[1] {
				t.Errorf("recently watched, newest first: %v", row.Items)
			}
		case "rediscover":
			if len(row.Items) != 1 || row.Items[0].ID != ids[2] {
				t.Errorf("rediscover: %v", row.Items)
			}
		case "top_by_person":
			if row.Name != "Ridley Scott" || row.Role != "director" || row.PersonID == 0 || len(row.Items) != 6 {
				t.Errorf("top by person: %+v", row)
			}
		case "recently_added":
			if row.Name != "Films" {
				t.Errorf("recently added names the library: %q", row.Name)
			}
		}
	}
	// Drama is the favourite genre, so it isn't also the day's top genre.
	want := []string{"continue", "recently_released", "recently_added", "top_rated", "recently_watched", "rediscover",
		"more_in_genre", "top_by_person", "top_unwatched"}
	if !slices.Equal(types, want) {
		t.Errorf("rows %v, want %v", types, want)
	}
}
