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

var errNotFound = errors.New("not found")

// TMDB is a small client for The Movie Database API. It accepts either a v3 API key or a
// v4 read access token.
type TMDB struct {
	key  string
	http *http.Client

	mu   sync.Mutex
	next time.Time
}

func NewTMDB(key string) *TMDB {
	return &TMDB{key: key, http: &http.Client{Timeout: 20 * time.Second}}
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
	if errors.Is(err, errNotFound) {
		return nil, nil
	}
	return m, err
}

type movieResult struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	OriginalTitle string `json:"original_title"`
	ReleaseDate   string `json:"release_date"`
}

func (c *TMDB) searchMovie(ctx context.Context, title string, year int) (int64, error) {
	search := func(withYear bool) ([]movieResult, error) {
		q := url.Values{"query": {title}}
		if withYear {
			q.Set("year", strconv.Itoa(year))
		}
		var resp struct {
			Results []movieResult `json:"results"`
		}
		err := c.get(ctx, "/search/movie", q, &resp)
		return resp.Results, err
	}

	if year > 0 {
		results, err := search(true)
		if err != nil {
			return 0, err
		}
		if len(results) > 0 {
			return bestMatch(title, results), nil
		}
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
			return 0, nil
		}
		return bestMatch(title, near), nil
	}

	results, err := search(false)
	if err != nil || len(results) == 0 {
		return 0, err
	}
	return bestMatch(title, results), nil
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
	if errors.Is(err, errNotFound) || len(resp.MovieResults) == 0 {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return resp.MovieResults[0].ID, nil
}

func (c *TMDB) movieDetails(ctx context.Context, id int64) (*Movie, error) {
	var d struct {
		ID            int64  `json:"id"`
		IMDBID        string `json:"imdb_id"`
		Title         string `json:"title"`
		OriginalTitle string `json:"original_title"`
		Overview      string `json:"overview"`
		ReleaseDate   string `json:"release_date"`
		Runtime       int    `json:"runtime"`
		PosterPath    string `json:"poster_path"`
		BackdropPath  string `json:"backdrop_path"`
	}
	if err := c.get(ctx, "/movie/"+strconv.FormatInt(id, 10), nil, &d); err != nil {
		return nil, err
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
	}, nil
}

func (c *TMDB) get(ctx context.Context, path string, q url.Values, out any) error {
	if q == nil {
		q = url.Values{}
	}
	bearer := strings.HasPrefix(c.key, "eyJ") // v4 tokens are JWTs
	if !bearer {
		q.Set("api_key", c.key)
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
			req.Header.Set("Authorization", "Bearer "+c.key)
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
			return errNotFound
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
