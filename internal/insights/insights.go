// Package insights turns watch history into stats: watch time over time, the most watched
// titles and genres, the busiest times, and a profile's year in review.
package insights

import (
	"cmp"
	"slices"
	"strconv"
	"time"

	"ams/internal/store"
)

// Bucket is how much was watched: viewings and time spent playing.
type Bucket struct {
	Plays     int   `json:"plays"`
	WatchedMS int64 `json:"watched_ms"`
}

func (b *Bucket) add(p store.Play) {
	b.Plays++
	b.WatchedMS += p.WatchedMS
}

// Point is one step of a chart over time, starting on Date (YYYY-MM-DD).
type Point struct {
	Date string `json:"date"`
	Bucket
}

// Ranked is a movie or show and how much it was watched.
type Ranked struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Bucket
	// How many profiles watched it, and (shows) how many different episodes.
	Profiles int `json:"profiles"`
	Episodes int `json:"episodes,omitempty"`
}

// Named is a genre, device or the like and how much was watched.
type Named struct {
	Name string `json:"name"`
	Bucket
}

// Keyed is a profile or library (by id) and how much it watched.
type Keyed struct {
	ID string `json:"id"`
	Bucket
}

// Overview is watch activity over a period.
type Overview struct {
	Total Bucket `json:"total"`
	// Different movies and shows watched, and profiles watching.
	Titles   int `json:"titles"`
	Profiles int `json:"profiles_watching"`
	// Watch time over the period, by day, week or month (Step) depending on its length.
	Step      string   `json:"step"`
	Timeline  []Point  `json:"timeline"`
	ByProfile []Keyed  `json:"by_profile"`
	ByLibrary []Keyed  `json:"by_library"`
	Movies    []Ranked `json:"movies"`
	Shows     []Ranked `json:"shows"`
	Genres    []Named  `json:"genres"`
	Devices   []Named  `json:"devices"`
	Methods   []Named  `json:"methods"`
	// Watch time by weekday (0 = Sunday) and hour, by when viewings started.
	Hours [7][24]int64 `json:"hours"`
}

// TitleID is the movie or show a viewing counts towards.
func TitleID(p store.Play) string {
	if p.SeriesID != "" {
		return p.SeriesID
	}
	return p.ItemID
}

func titleName(p store.Play) string {
	if p.SeriesTitle != "" {
		return p.SeriesTitle
	}
	return p.Title
}

// Summarise works out an Overview of viewings started between from and to. A zero from means
// since the first viewing. genres are by movie or show id.
func Summarise(plays []store.Play, genres map[string][]string, from, to time.Time, top int) Overview {
	o := Overview{Timeline: []Point{}}
	titles := map[string]*rankAcc{}
	profiles := map[string]*Bucket{}
	libraries := map[string]*Bucket{}
	genreAcc := map[string]*Bucket{}
	devices := map[string]*Bucket{}
	methods := map[string]*Bucket{}
	for _, p := range plays {
		o.Total.add(p)
		acc(profiles, p.UserID).add(p)
		acc(libraries, strconv.FormatInt(p.LibraryID, 10)).add(p)
		if p.Device != "" {
			acc(devices, p.Device).add(p)
		}
		if p.Method != "" {
			acc(methods, p.Method).add(p)
		}
		id := TitleID(p)
		t := titles[id]
		if t == nil {
			t = &rankAcc{Ranked: Ranked{ID: id, Title: titleName(p)}, show: p.SeriesID != "", profiles: map[string]bool{}, episodes: map[string]bool{}}
			titles[id] = t
		}
		t.add(p)
		t.profiles[p.UserID] = true
		if p.SeriesID != "" {
			t.episodes[p.ItemID] = true
		}
		for _, g := range genres[id] {
			acc(genreAcc, g).add(p)
		}
		start := time.Unix(p.StartedAt, 0)
		o.Hours[start.Weekday()][start.Hour()] += p.WatchedMS
	}
	o.Titles, o.Profiles = len(titles), len(profiles)

	for _, t := range titles {
		t.Profiles, t.Episodes = len(t.profiles), len(t.episodes)
		if t.show {
			o.Shows = append(o.Shows, t.Ranked)
		} else {
			o.Movies = append(o.Movies, t.Ranked)
		}
	}
	o.Movies, o.Shows = topRanked(o.Movies, top), topRanked(o.Shows, top)
	o.ByProfile, o.ByLibrary = keyed(profiles), keyed(libraries)
	o.Genres = topNamed(named(genreAcc), top)
	o.Devices, o.Methods = named(devices), named(methods)

	if from.IsZero() && len(plays) > 0 {
		from = time.Unix(slices.MinFunc(plays, func(a, b store.Play) int { return cmp.Compare(a.StartedAt, b.StartedAt) }).StartedAt, 0)
	}
	if !from.IsZero() {
		o.Step, o.Timeline = timeline(plays, from, to)
	}
	return o
}

type rankAcc struct {
	Ranked
	show     bool
	profiles map[string]bool
	episodes map[string]bool
}

// timeline buckets watch time by day for up to three months, by week up to a year and a half,
// and by month beyond that.
func timeline(plays []store.Play, from, to time.Time) (string, []Point) {
	start := midnight(from)
	step, next := "day", func(t time.Time) time.Time { return t.AddDate(0, 0, 1) }
	switch days := to.Sub(start).Hours() / 24; {
	case days > 550:
		step, next = "month", func(t time.Time) time.Time { return t.AddDate(0, 1, 0) }
		start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	case days > 92:
		step, next = "week", func(t time.Time) time.Time { return t.AddDate(0, 0, 7) }
		start = start.AddDate(0, 0, -int((start.Weekday()+6)%7)) // weeks start on Monday
	}
	var points []Point
	var bounds []int64
	for t := start; t.Before(to); t = next(t) {
		points = append(points, Point{Date: t.Format(time.DateOnly)})
		bounds = append(bounds, t.Unix())
	}
	for _, p := range plays {
		i, found := slices.BinarySearch(bounds, p.StartedAt)
		if !found {
			i--
		}
		if i >= 0 && i < len(points) {
			points[i].add(p)
		}
	}
	if points == nil {
		points = []Point{}
	}
	return step, points
}

func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func acc(m map[string]*Bucket, key string) *Bucket {
	b := m[key]
	if b == nil {
		b = &Bucket{}
		m[key] = b
	}
	return b
}

func byWatched(a, b Bucket) int {
	return cmp.Or(cmp.Compare(b.WatchedMS, a.WatchedMS), cmp.Compare(b.Plays, a.Plays))
}

func topRanked(list []Ranked, n int) []Ranked {
	slices.SortFunc(list, func(a, b Ranked) int {
		return cmp.Or(cmp.Compare(b.Plays, a.Plays), byWatched(a.Bucket, b.Bucket), cmp.Compare(a.Title, b.Title))
	})
	if list == nil {
		list = []Ranked{}
	}
	return list[:min(n, len(list))]
}

func keyed(m map[string]*Bucket) []Keyed {
	out := []Keyed{}
	for k, b := range m {
		out = append(out, Keyed{ID: k, Bucket: *b})
	}
	slices.SortFunc(out, func(a, b Keyed) int { return cmp.Or(byWatched(a.Bucket, b.Bucket), cmp.Compare(a.ID, b.ID)) })
	return out
}

func named(m map[string]*Bucket) []Named {
	out := []Named{}
	for k, b := range m {
		out = append(out, Named{Name: k, Bucket: *b})
	}
	slices.SortFunc(out, func(a, b Named) int { return cmp.Or(byWatched(a.Bucket, b.Bucket), cmp.Compare(a.Name, b.Name)) })
	return out
}

func topNamed(list []Named, n int) []Named {
	return list[:min(n, len(list))]
}
