package insights

import (
	"testing"
	"time"

	"ams/internal/store"
)

func at(s string) int64 {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t.Unix()
}

var plays = []store.Play{
	{UserID: "a", ItemID: "m1", Kind: "movie", LibraryID: 1, Title: "Heat", StartedAt: at("2026-03-02 20:00"), WatchedMS: 9_000_000, Device: "Chrome on Windows", Method: "direct"},
	{UserID: "b", ItemID: "m1", Kind: "movie", LibraryID: 1, Title: "Heat", StartedAt: at("2026-03-03 21:00"), WatchedMS: 3_000_000, Device: "Safari on iPhone", Method: "transcode"},
	{UserID: "a", ItemID: "e1", Kind: "episode", LibraryID: 2, SeriesID: "s1", SeriesTitle: "Firefly", Title: "Serenity", StartedAt: at("2026-03-03 19:00"), WatchedMS: 2_600_000, Method: "direct"},
	{UserID: "a", ItemID: "e2", Kind: "episode", LibraryID: 2, SeriesID: "s1", SeriesTitle: "Firefly", Title: "The Train Job", StartedAt: at("2026-03-03 20:00"), WatchedMS: 2_600_000, Method: "direct"},
	{UserID: "a", ItemID: "e3", Kind: "episode", LibraryID: 2, SeriesID: "s1", SeriesTitle: "Firefly", Title: "Bushwhacked", StartedAt: at("2026-03-04 20:00"), WatchedMS: 2_600_000, Method: "direct"},
}

var genres = map[string][]string{"m1": {"Crime", "Drama"}, "s1": {"Drama", "Sci-Fi"}}

func TestSummarise(t *testing.T) {
	o := Summarise(plays, genres, time.Unix(at("2026-03-01 00:00"), 0), time.Unix(at("2026-03-08 00:00"), 0), 10)
	if o.Total.Plays != 5 || o.Total.WatchedMS != 19_800_000 || o.Titles != 2 || o.Profiles != 2 {
		t.Fatalf("totals: %+v titles %d profiles %d", o.Total, o.Titles, o.Profiles)
	}
	if len(o.Movies) != 1 || o.Movies[0].Plays != 2 || o.Movies[0].Profiles != 2 {
		t.Errorf("movies: %+v", o.Movies)
	}
	if len(o.Shows) != 1 || o.Shows[0].Title != "Firefly" || o.Shows[0].Episodes != 3 || o.Shows[0].Profiles != 1 {
		t.Errorf("shows: %+v", o.Shows)
	}
	if o.Genres[0].Name != "Drama" || o.Genres[0].Plays != 5 {
		t.Errorf("genres: %+v", o.Genres)
	}
	if o.Step != "day" || len(o.Timeline) != 7 || o.Timeline[1].Plays != 1 || o.Timeline[2].Plays != 3 || o.Timeline[2].Date != "2026-03-03" {
		t.Errorf("timeline: %s %+v", o.Step, o.Timeline)
	}
	if o.ByLibrary[0].ID != "1" || o.Methods[0].Name != "direct" || o.Methods[0].Plays != 4 {
		t.Errorf("libraries %+v methods %+v", o.ByLibrary, o.Methods)
	}
	if o.Hours[time.Monday][20] != 9_000_000 {
		t.Errorf("Monday 8pm: %d", o.Hours[time.Monday][20])
	}

	long := Summarise(plays, nil, time.Unix(at("2025-01-01 00:00"), 0), time.Unix(at("2026-12-31 00:00"), 0), 10)
	if long.Step != "month" || long.Timeline[0].Date != "2025-01-01" {
		t.Errorf("two years: %s %v", long.Step, long.Timeline[0])
	}
	var total int
	for _, p := range long.Timeline {
		total += p.Plays
	}
	if total != 5 {
		t.Errorf("monthly timeline lost viewings: %d", total)
	}
}

func TestReview(t *testing.T) {
	var mine []store.Play
	for _, p := range plays {
		if p.UserID == "a" {
			mine = append(mine, p)
		}
	}
	actors := map[string][]store.Person{
		"m1": {{ID: 1, Name: "Al Pacino"}, {ID: 3, Name: "Both"}},
		"s1": {{ID: 2, Name: "Nathan Fillion"}, {ID: 3, Name: "Both"}},
	}
	y := Review(2026, mine, genres, actors, 5)
	if y.Plays != 4 || y.Movies != 1 || y.Shows != 1 || y.Episodes != 3 || y.Days != 3 || y.Streak != 3 {
		t.Fatalf("counts: %+v", y)
	}
	if y.Binge == nil || y.Binge.Episodes != 2 || y.Binge.Date != "2026-03-03" || y.Binge.Title != "Firefly" {
		t.Errorf("binge: %+v", y.Binge)
	}
	if len(y.People) != 1 || y.People[0].Name != "Both" || y.People[0].Titles != 2 {
		t.Errorf("people: %+v", y.People)
	}
	if y.First == nil || y.First.Title != "Heat" || y.Months[2] != 16_800_000 {
		t.Errorf("first %+v march %d", y.First, y.Months[2])
	}
	if empty := Review(2025, nil, nil, nil, 5); empty.Plays != 0 || empty.TopMovies == nil {
		t.Errorf("empty year: %+v", empty)
	}
}
