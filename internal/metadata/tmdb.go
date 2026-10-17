// Package metadata talks to online metadata providers.
package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const tmdbBase = "https://api.themoviedb.org/3"

// ErrNotFound means the provider has nothing with the requested id.
var ErrNotFound = errors.New("not found")

// TMDB is a small client for The Movie Database API. It accepts either a v3 API key or a
// v4 read access token.
type TMDB struct {
	http *http.Client

	mu     sync.Mutex
	key    string
	region string
	next   time.Time
}

// SetRegion sets the country (ISO 3166-1, e.g. "GB") whose age ratings are used.
func (c *TMDB) SetRegion(region string) {
	c.mu.Lock()
	c.region = region
	c.mu.Unlock()
}

// Region is the country whose age ratings are used ("US" by default).
func (c *TMDB) Region() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.region == "" {
		return "US"
	}
	return c.region
}

func NewTMDB(key string) *TMDB {
	return &TMDB{key: key, http: &http.Client{Timeout: 20 * time.Second}}
}

// SetKey changes the API key (e.g. after it's edited in Settings). "" turns TMDB off.
func (c *TMDB) SetKey(key string) {
	c.mu.Lock()
	c.key = key
	c.mu.Unlock()
}

// Configured reports whether there's an API key to use.
func (c *TMDB) Configured() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.key != ""
}

func (c *TMDB) apiKey() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.key
}

// CheckTMDBKey makes a request with key to see whether TMDB accepts it.
func CheckTMDBKey(ctx context.Context, key string) error {
	err := NewTMDB(key).get(ctx, "/configuration", nil, &struct{}{})
	if err != nil && strings.Contains(err.Error(), "401") {
		return errors.New("TMDB didn't accept this key")
	}
	return err
}

// ImageURL builds a URL for an image path: TMDB paths such as "/abc.jpg" go to TMDB's CDN,
// full URLs (TVDB artwork) are returned as they are.
func ImageURL(path, size string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return "https://image.tmdb.org/t/p/" + size + path
}

type Movie struct {
	TMDBID        int64
	IMDBID        string
	Title         string
	OriginalTitle string
	Overview      string
	ReleaseDate   string
	Year          int
	RuntimeMin    int
	PosterPath    string
	BackdropPath  string
	Details
}

// MatchMovie identifies a movie, preferring explicit ids over a title search. It returns nil
// when nothing plausible is found.
func (c *TMDB) MatchMovie(ctx context.Context, title string, year int, tmdbID int64, imdbID string) (*Movie, error) {
	if tmdbID == 0 && imdbID != "" {
		id, err := c.findByIMDB(ctx, imdbID)
		if err != nil {
			return nil, err
		}
		tmdbID = id
	}
	if tmdbID == 0 {
		if title == "" {
			record(ctx, Step{Provider: "tmdb", Kind: "search", Note: "No title could be read from the file name, so there was nothing to search for."})
			return nil, nil
		}
		id, err := c.searchMovie(ctx, title, year)
		if err != nil {
			return nil, err
		}
		tmdbID = id
	}
	if tmdbID == 0 {
		return nil, nil
	}
	m, err := c.movieDetails(ctx, tmdbID)
	if errors.Is(err, ErrNotFound) {
		record(ctx, Step{Provider: "tmdb", Kind: "lookup", Query: strconv.FormatInt(tmdbID, 10),
			Note: "TMDB has no movie with this id."})
		return nil, nil
	}
	return m, err
}

type movieResult struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	ReleaseDate   string `json:"release_date"`
	Overview      string `json:"overview"`
	PosterPath    string `json:"poster_path"`
}

func (r movieResult) candidate() Candidate {
	c := Candidate{Provider: "tmdb", ID: r.ID, Title: r.Title, Year: yearOf(r.ReleaseDate), Overview: r.Overview}
	if r.OriginalTitle != r.Title {
		c.OriginalTitle = r.OriginalTitle
	}
	if r.PosterPath != "" {
		c.Poster = ImageURL(r.PosterPath, "w185")
	}
	return c
}

func movieCandidates(results []movieResult) []Candidate {
	out := make([]Candidate, 0, len(results))
	for _, r := range results {
		out = append(out, r.candidate())
	}
	return out
}

func (c *TMDB) searchMovies(ctx context.Context, title string, year int) ([]movieResult, error) {
	q := url.Values{"query": {title}}
	if year > 0 {
		q.Set("year", strconv.Itoa(year))
	}
	var resp struct {
		Results []movieResult `json:"results"`
	}
	err := c.get(ctx, "/search/movie", q, &resp)
	return resp.Results, err
}

// SearchMovies searches TMDB by title (and year, if not 0) for someone choosing a match by hand.
func (c *TMDB) SearchMovies(ctx context.Context, title string, year int) ([]Candidate, error) {
	results, err := c.searchMovies(ctx, title, year)
	return movieCandidates(results), err
}

func (c *TMDB) searchMovie(ctx context.Context, title string, year int) (int64, error) {
	search := func(withYear bool) ([]movieResult, error) {
		y := 0
		if withYear {
			y = year
		}
		results, err := c.searchMovies(ctx, title, y)
		if err != nil {
			record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Year: y, Error: err.Error()})
		}
		return results, err
	}

	if year > 0 {
		results, err := search(true)
		if err != nil {
			return 0, err
		}
		if len(results) > 0 {
			id := bestMatch(title, results)
			record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Year: year, Results: movieCandidates(results), Picked: id})
			return id, nil
		}
		record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Year: year, Results: []Candidate{},
			Note: "Nothing with this title came out in " + strconv.Itoa(year) + ", so Arc searched again without the year."})
		// Release years differ between countries; accept a one-year drift.
		results, err = search(false)
		if err != nil {
			return 0, err
		}
		var near []movieResult
		for _, r := range results {
			if y := yearOf(r.ReleaseDate); y >= year-1 && y <= year+1 {
				near = append(near, r)
			}
		}
		if len(near) == 0 {
			note := "No results."
			if len(results) > 0 {
				note = "None of these came out within a year of " + strconv.Itoa(year) + ", so none was used."
			}
			record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Results: movieCandidates(results), Note: note})
			return 0, nil
		}
		id := bestMatch(title, near)
		record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Results: movieCandidates(results), Picked: id})
		return id, nil
	}

	results, err := search(false)
	if err != nil {
		return 0, err
	}
	if len(results) == 0 {
		record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Note: "No results."})
		return 0, nil
	}
	id := bestMatch(title, results)
	record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Results: movieCandidates(results), Picked: id})
	return id, nil
}

// bestMatch prefers an exact (normalised) title match, otherwise TMDB's top relevance result.
func bestMatch(title string, results []movieResult) int64 {
	want := Normalize(title)
	for _, r := range results {
		if Normalize(r.Title) == want || Normalize(r.OriginalTitle) == want {
			return r.ID
		}
	}
	return results[0].ID
}

func (c *TMDB) findByIMDB(ctx context.Context, imdbID string) (int64, error) {
	var resp struct {
		MovieResults []movieResult `json:"movie_results"`
	}
	err := c.get(ctx, "/find/"+url.PathEscape(imdbID), url.Values{"external_source": {"imdb_id"}}, &resp)
	if err != nil && !errors.Is(err, ErrNotFound) {
		record(ctx, Step{Provider: "tmdb", Kind: "find", Query: imdbID, Error: err.Error()})
		return 0, err
	}
	if len(resp.MovieResults) == 0 {
		record(ctx, Step{Provider: "tmdb", Kind: "find", Query: imdbID,
			Note: "TMDB doesn't know this IMDb id, so Arc searched by title instead."})
		return 0, nil
	}
	record(ctx, Step{Provider: "tmdb", Kind: "find", Query: imdbID, Results: movieCandidates(resp.MovieResults),
		Picked: resp.MovieResults[0].ID})
	return resp.MovieResults[0].ID, nil
}

func (c *TMDB) movieDetails(ctx context.Context, id int64) (*Movie, error) {
	var d struct {
		ID                  int64       `json:"id"`
		IMDBID              string      `json:"imdb_id"`
		Title               string      `json:"title"`
		OriginalTitle       string      `json:"original_title"`
		Overview            string      `json:"overview"`
		Tagline             string      `json:"tagline"`
		ReleaseDate         string      `json:"release_date"`
		Runtime             int         `json:"runtime"`
		PosterPath          string      `json:"poster_path"`
		BackdropPath        string      `json:"backdrop_path"`
		VoteAverage         float64     `json:"vote_average"`
		Genres              []tmdbNamed `json:"genres"`
		ProductionCompanies []tmdbNamed `json:"production_companies"`
		Credits             struct {
			Cast []tmdbCastMember `json:"cast"`
			Crew []tmdbCrewMember `json:"crew"`
		} `json:"credits"`
		ReleaseDates struct {
			Results []struct {
				Country string `json:"iso_3166_1"`
				Dates   []struct {
					Certification string `json:"certification"`
					Type          int    `json:"type"`
				} `json:"release_dates"`
			} `json:"results"`
		} `json:"release_dates"`
	}
	q := url.Values{"append_to_response": {"credits,release_dates"}}
	if err := c.get(ctx, "/movie/"+strconv.FormatInt(id, 10), q, &d); err != nil {
		return nil, err
	}
	ratings := map[string]string{}
	for _, r := range d.ReleaseDates.Results {
		for _, rd := range r.Dates {
			// Prefer the theatrical rating; any other release's if there's no theatrical one.
			if rd.Certification != "" && (ratings[r.Country] == "" || rd.Type == 3) {
				ratings[r.Country] = rd.Certification
			}
		}
	}
	return &Movie{
		TMDBID:        d.ID,
		IMDBID:        d.IMDBID,
		Title:         d.Title,
		OriginalTitle: d.OriginalTitle,
		Overview:      d.Overview,
		ReleaseDate:   d.ReleaseDate,
		Year:          yearOf(d.ReleaseDate),
		RuntimeMin:    d.Runtime,
		PosterPath:    d.PosterPath,
		BackdropPath:  d.BackdropPath,
		Details: Details{
			Tagline:       d.Tagline,
			ContentRating: pickRating(c.Region(), ratings),
			Studios:       names(d.ProductionCompanies),
			Rating:        d.VoteAverage,
			Genres:        names(d.Genres),
			People:        credits(d.Credits.Cast, d.Credits.Crew),
		},
	}, nil
}

func (c *TMDB) get(ctx context.Context, path string, q url.Values, out any) error {
	if q == nil {
		q = url.Values{}
	}
	key := c.apiKey()
	bearer := strings.HasPrefix(key, "eyJ") // v4 tokens are JWTs
	if !bearer {
		q.Set("api_key", key)
	}
	u := tmdbBase + path + "?" + q.Encode()

	for attempt := 0; ; attempt++ {
		if err := c.throttle(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		if bearer {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		switch {
		case resp.StatusCode == http.StatusTooManyRequests && attempt < 3:
			resp.Body.Close()
			wait, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
			if err := sleep(ctx, time.Duration(max(wait, 1))*time.Second); err != nil {
				return err
			}
			continue
		case resp.StatusCode == http.StatusNotFound:
			resp.Body.Close()
			return ErrNotFound
		case resp.StatusCode != http.StatusOK:
			resp.Body.Close()
			return fmt.Errorf("tmdb %s: %s", path, resp.Status)
		}
		err = json.NewDecoder(resp.Body).Decode(out)
		resp.Body.Close()
		return err
	}
}

// throttle keeps requests well under TMDB's rate limit.
func (c *TMDB) throttle(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	wait := c.next.Sub(now)
	c.next = now.Add(max(wait, 0) + 30*time.Millisecond)
	c.mu.Unlock()
	if wait > 0 {
		return sleep(ctx, wait)
	}
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func yearOf(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, _ := strconv.Atoi(date[:4])
	return y
}

// Normalize reduces a title to lowercase letters and digits for comparison.
func Normalize(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "&", "and")
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
