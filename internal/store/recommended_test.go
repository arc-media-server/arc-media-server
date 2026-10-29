package store

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestRecommendedQueries(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "Movies", "movies", "tmdb_aired", []string{"/m"})
	u, err := st.CreateUser(ctx, "Sam", false, "blue", "", "")
	if err != nil {
		t.Fatal(err)
	}
	ripley := Credit{TMDBID: 10205, Name: "Sigourney Weaver", Role: "actor"}
	scott := Credit{TMDBID: 578, Name: "Ridley Scott", Role: "director"}
	add := func(title string, genre string, credits ...Credit) string {
		id, _, _ := st.EnsureItem(ctx, &Item{LibraryID: lib, Kind: "movie", Title: title}, title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: lib, Path: "/m/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{id}, 0)
		if err := st.SetItemDetails(ctx, id, []string{genre}, credits); err != nil {
			t.Fatal(err)
		}
		pg := "PG"
		if err := st.EditItem(ctx, id, ItemEdit{ContentRating: &pg}); err != nil {
			t.Fatal(err)
		}
		return id
	}
	alien := add("Alien", "Horror", ripley, scott)
	aliens := add("Aliens", "Horror", ripley)
	gladiator := add("Gladiator", "Action", scott)
	avatar := add("Avatar", "Horror", ripley)
	half := add("Half Watched", "Comedy")
	unseen := add("Unseen", "Comedy")
	adult := add("Adult", "Horror", ripley)
	rating := "18"
	if err := st.EditItem(ctx, adult, ItemEdit{ContentRating: &rating}); err != nil {
		t.Fatal(err)
	}

	// Alien a year ago, Gladiator yesterday, Half Watched part-way.
	if err := st.SetPlayed(ctx, u.ID, []string{alien, gladiator}, true); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	st.db.ExecContext(ctx, `UPDATE user_item_data SET last_played_at = ? WHERE item_id = ?`, now-365*86400, alien)
	st.db.ExecContext(ctx, `UPDATE user_item_data SET last_played_at = ? WHERE item_id = ?`, now-86400, gladiator)
	if _, err := st.ReportProgress(ctx, u.ID, half, 3_600_000, 7_200_000); err != nil {
		t.Fatal(err)
	}

	list := func(q ItemQuery) []string {
		t.Helper()
		q.LibraryID, q.Kind, q.UserID = lib, "movie", u.ID
		items, _, err := st.ListItems(ctx, q)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, it := range items {
			out = append(out, it.ID)
		}
		slices.Sort(out)
		return out
	}
	sorted := func(ids ...string) []string { slices.Sort(ids); return ids }
	if got := list(ItemQuery{Unstarted: true}); !slices.Equal(got, sorted(aliens, avatar, unseen, adult)) {
		t.Errorf("not started (part-watched counts as started): %v", got)
	}
	if got := list(ItemQuery{Played: true}); !slices.Equal(got, sorted(alien, gladiator)) {
		t.Errorf("played: %v", got)
	}
	if got := list(ItemQuery{PlayedBefore: now - 180*86400}); !slices.Equal(got, []string{alien}) {
		t.Errorf("finished six months ago: %v", got)
	}
	if got := list(ItemQuery{PersonID: personID(t, st, scott.TMDBID), PersonRole: "director"}); !slices.Equal(got, sorted(alien, gladiator)) {
		t.Errorf("directed by Ridley Scott: %v", got)
	}
	if got := list(ItemQuery{PersonID: personID(t, st, scott.TMDBID), PersonRole: "actor"}); len(got) != 0 {
		t.Errorf("Ridley Scott doesn't act: %v", got)
	}

	genres, err := st.LibraryGenres(ctx, lib, "movie", 3, Access{MaxAge: 12})
	if err != nil || !slices.Equal(genres, []string{"Horror"}) {
		t.Errorf("genres with 3 titles: %v %v", genres, err)
	}
	if genres, _ := st.LibraryGenres(ctx, lib, "movie", 5, Access{MaxAge: 12}); len(genres) != 0 {
		t.Errorf("the 18 doesn't count for a child: %v", genres)
	}

	people, err := st.FeaturedPeople(ctx, u.ID, lib, "movie", 2, Access{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	// Ridley Scott directed what was watched yesterday; Sigourney Weaver is in more but was
	// last watched a year ago.
	if len(people) != 2 || people[0].Name != "Ridley Scott" || people[0].Role != "director" || people[1].Titles != 4 {
		t.Errorf("featured people: %+v", people)
	}
	if people, _ := st.FeaturedPeople(ctx, u.ID, lib, "movie", 4, Access{MaxAge: 12}, 10); len(people) != 0 {
		t.Errorf("only three of Sigourney Weaver's are for a child: %+v", people)
	}
}

func TestUnstartedAndWatchedEpisodes(t *testing.T) {
	ctx := context.Background()
	st := openTest(t)
	lib, _ := st.SyncLibrary(ctx, "TV", "shows", "tvdb_aired", []string{"/tv"})
	other, _ := st.SyncLibrary(ctx, "Other", "shows", "tvdb_aired", []string{"/other"})
	u, _ := st.CreateUser(ctx, "Sam", false, "blue", "", "")
	show := func(library int64, title string) (string, string) {
		sid, _, _ := st.EnsureItem(ctx, &Item{LibraryID: library, Kind: "series", Title: title}, "s:"+title)
		eid, _, _ := st.EnsureItem(ctx, &Item{LibraryID: library, Kind: "episode", ParentID: sid, Title: title + " 1"}, "e:"+title)
		fid, _ := st.SaveFile(ctx, &MediaFile{LibraryID: library, Path: "/tv/" + title, Size: 1, ModTime: 1}, 1, 1)
		st.SetFileItems(ctx, fid, []string{eid}, 0)
		return sid, eid
	}
	watched, ep := show(lib, "Watched")
	fresh, _ := show(lib, "Fresh")
	_, otherEp := show(other, "Elsewhere")
	if err := st.SetPlayed(ctx, u.ID, []string{ep, otherEp}, true); err != nil {
		t.Fatal(err)
	}
	items, _, err := st.ListItems(ctx, ItemQuery{LibraryID: lib, Kind: "series", UserID: u.ID, Unstarted: true})
	if err != nil || len(items) != 1 || items[0].ID != fresh {
		t.Errorf("shows not started: %v %v", items, err)
	}
	items, _, _ = st.ListItems(ctx, ItemQuery{LibraryID: lib, Kind: "series", UserID: u.ID, Played: true})
	if len(items) != 1 || items[0].ID != watched {
		t.Errorf("shows watched: %v", items)
	}
	items, _, _ = st.ListItems(ctx, ItemQuery{LibraryID: lib, Kind: "episode", UserID: u.ID, Played: true, Sort: "watched"})
	if len(items) != 1 || items[0].ID != ep {
		t.Errorf("episodes watched: %v", items)
	}
	series, err := st.RecentlyWatchedSeries(ctx, u.ID, 10, Access{Libraries: []int64{lib}})
	if err != nil || len(series) != 1 || series[0].SeriesID != watched {
		t.Errorf("shows watched in one library: %+v %v", series, err)
	}
	if series, _ := st.RecentlyWatchedSeries(ctx, u.ID, 10, Access{}); len(series) != 2 {
		t.Errorf("shows watched anywhere: %+v", series)
	}
}

func personID(t *testing.T, st *Store, tmdbID int64) int64 {
	t.Helper()
	var id int64
	if err := st.db.QueryRowContext(context.Background(), `SELECT id FROM people WHERE tmdb_id = ?`, tmdbID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
