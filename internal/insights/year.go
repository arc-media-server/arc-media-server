package insights

import (
	"cmp"
	"slices"
	"time"

	"ams/internal/store"
)

// Year is a profile's year in review.
type Year struct {
	Year int `json:"year"`
	Bucket
	// Different movies, shows and episodes watched, and days with any watching.
	Movies   int `json:"movies"`
	Shows    int `json:"shows"`
	Episodes int `json:"episodes"`
	Days     int `json:"days"`
	// Longest run of days in a row with some watching.
	Streak    int            `json:"streak"`
	TopMovies []Ranked       `json:"top_movies"`
	TopShows  []Ranked       `json:"top_shows"`
	Genres    []Named        `json:"genres"`
	People    []RankedPerson `json:"people"`
	// Watch time by month (0 = January), weekday (0 = Sunday) and hour.
	Months   [12]int64 `json:"months"`
	Weekdays [7]int64  `json:"weekdays"`
	Hours    [24]int64 `json:"hours"`
	// The most episodes of one show watched in a day, and the year's first viewing.
	Binge *Binge      `json:"binge,omitempty"`
	First *store.Play `json:"first,omitempty"`
}

// RankedPerson is an actor and how much of what they're in was watched.
type RankedPerson struct {
	store.Person
	Titles    int   `json:"titles"`
	WatchedMS int64 `json:"watched_ms"`
}

// Binge is a day spent on one show.
type Binge struct {
	SeriesID string `json:"series_id"`
	Title    string `json:"title"`
	Date     string `json:"date"`
	Episodes int    `json:"episodes"`
}

// Review works out a year in review from one profile's viewings that year. genres and actors
// are by movie or show id.
func Review(year int, plays []store.Play, genres map[string][]string, actors map[string][]store.Person, top int) Year {
	y := Year{Year: year, TopMovies: []Ranked{}, TopShows: []Ranked{}, Genres: []Named{}, People: []RankedPerson{}}
	if len(plays) == 0 {
		return y
	}
	o := Summarise(plays, genres, time.Time{}, time.Time{}, top)
	y.Bucket, y.TopMovies, y.TopShows, y.Genres = o.Total, o.Movies, o.Shows, o.Genres

	movies, shows, episodes := map[string]bool{}, map[string]bool{}, map[string]bool{}
	days := map[string]bool{}
	type bingeKey struct{ series, day string }
	binges := map[bingeKey]map[string]bool{}
	seriesTitles := map[string]string{}
	people := map[int64]*RankedPerson{}
	peopleTitles := map[int64]map[string]bool{}
	for _, p := range plays {
		start := time.Unix(p.StartedAt, 0)
		day := start.Format(time.DateOnly)
		days[day] = true
		y.Months[start.Month()-1] += p.WatchedMS
		y.Weekdays[start.Weekday()] += p.WatchedMS
		y.Hours[start.Hour()] += p.WatchedMS
		if p.SeriesID != "" {
			shows[p.SeriesID], episodes[p.ItemID] = true, true
			seriesTitles[p.SeriesID] = p.SeriesTitle
			k := bingeKey{p.SeriesID, day}
			if binges[k] == nil {
				binges[k] = map[string]bool{}
			}
			binges[k][p.ItemID] = true
		} else {
			movies[p.ItemID] = true
		}
		id := TitleID(p)
		for _, a := range actors[id] {
			rp := people[a.ID]
			if rp == nil {
				rp = &RankedPerson{Person: a}
				people[a.ID], peopleTitles[a.ID] = rp, map[string]bool{}
			}
			rp.WatchedMS += p.WatchedMS
			peopleTitles[a.ID][id] = true
		}
	}
	y.Movies, y.Shows, y.Episodes, y.Days = len(movies), len(shows), len(episodes), len(days)
	y.Streak = longestStreak(days)

	for k, eps := range binges {
		if len(eps) < 2 {
			continue
		}
		if b := y.Binge; b == nil || len(eps) > b.Episodes || (len(eps) == b.Episodes && k.day < b.Date) {
			y.Binge = &Binge{SeriesID: k.series, Title: seriesTitles[k.series], Date: k.day, Episodes: len(eps)}
		}
	}

	// Favourite actors: in more than one title watched, by time spent watching them.
	for id, rp := range people {
		rp.Titles = len(peopleTitles[id])
		if rp.Titles > 1 {
			y.People = append(y.People, *rp)
		}
	}
	slices.SortFunc(y.People, func(a, b RankedPerson) int {
		return cmp.Or(cmp.Compare(b.Titles, a.Titles), cmp.Compare(b.WatchedMS, a.WatchedMS), cmp.Compare(a.Name, b.Name))
	})
	y.People = y.People[:min(top, len(y.People))]

	first := slices.MinFunc(plays, func(a, b store.Play) int { return cmp.Compare(a.StartedAt, b.StartedAt) })
	y.First = &first
	return y
}

// longestStreak is the longest run of consecutive days (YYYY-MM-DD).
func longestStreak(days map[string]bool) int {
	best := 0
	for d := range days {
		t, err := time.Parse(time.DateOnly, d)
		if err != nil || days[t.AddDate(0, 0, -1).Format(time.DateOnly)] {
			continue // not the start of a run
		}
		n := 1
		for days[t.AddDate(0, 0, n).Format(time.DateOnly)] {
			n++
		}
		best = max(best, n)
	}
	return best
}
