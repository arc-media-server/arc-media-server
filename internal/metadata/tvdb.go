package metadata

import (
	"bytes"
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
)

const tvdbBase = "https://api4.thetvdb.com/v4"

// TVDB is a client for TheTVDB v4 API.
type TVDB struct {
	http *http.Client

	mu      sync.Mutex
	key     string
	token   string
	tokenAt time.Time
	next    time.Time
}

func NewTVDB(key string) *TVDB {
	return &TVDB{key: key, http: &http.Client{Timeout: 30 * time.Second}}
}

// SetKey changes the API key (e.g. after it's edited in Settings). "" turns TheTVDB off.
func (c *TVDB) SetKey(key string) {
	c.mu.Lock()
	if key != c.key {
		c.key, c.token = key, ""
	}
	c.mu.Unlock()
}

// Configured reports whether there's an API key to use.
func (c *TVDB) Configured() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.key != ""
}

// CheckTVDBKey logs in with key to see whether TheTVDB accepts it.
func CheckTVDBKey(ctx context.Context, key string) error {
	if _, err := NewTVDB(key).ensureToken(ctx, true); err != nil {
		return errors.New("TheTVDB didn't accept this key")
	}
	return nil
}

// TVDB season types we expose, in display order, with friendly names.
var tvdbSeasonTypes = []struct{ typ, source, name string }{
	{"official", "tvdb_aired", "Aired Order"},
	{"dvd", "tvdb_dvd", "DVD Order"},
	{"absolute", "tvdb_absolute", "Absolute Order"},
	{"alternate", "tvdb_alternate", "Alternate Order"},
	{"regional", "tvdb_regional", "Regional Order"},
	{"altdvd", "tvdb_altdvd", "Alternate DVD Order"},
}

type tvdbSearchResult struct {
	TVDBID       string            `json:"tvdb_id"`
	Name         string            `json:"name"`
	Year         string            `json:"year"`
	Overview     string            `json:"overview"`
	ImageURL     string            `json:"image_url"`
	Translations map[string]string `json:"translations"`
	Overviews    map[string]string `json:"overviews"`
	Aliases      []string          `json:"aliases"`
}

func tvdbCandidates(results []tvdbSearchResult) []Candidate {
	out := make([]Candidate, 0, len(results))
	for _, r := range results {
		id, _ := strconv.ParseInt(r.TVDBID, 10, 64)
		c := Candidate{Provider: "tvdb", ID: id, Title: r.Name, Overview: r.Overview, Poster: tvdbImage(r.ImageURL)}
		c.Year, _ = strconv.Atoi(r.Year)
		if eng := r.Translations["eng"]; eng != "" && eng != r.Name {
			c.Title, c.OriginalTitle = eng, r.Name
		}
		if eng := r.Overviews["eng"]; eng != "" {
			c.Overview = eng
		}
		out = append(out, c)
	}
	return out
}

func (c *TVDB) searchSeries(ctx context.Context, title string, year int) ([]tvdbSearchResult, error) {
	q := url.Values{"query": {title}, "type": {"series"}}
	if year > 0 {
		q.Set("year", strconv.Itoa(year))
	}
	var resp struct {
		Data []tvdbSearchResult `json:"data"`
	}
	err := c.get(ctx, "/search", q, &resp)
	return resp.Data, err
}

// SearchShows searches TheTVDB for series by title (and year, if not 0) for someone choosing a
// match by hand.
func (c *TVDB) SearchShows(ctx context.Context, title string, year int) ([]Candidate, error) {
	results, err := c.searchSeries(ctx, title, year)
	return tvdbCandidates(results), err
}

// SearchSeries finds a series id by title and optional first-aired year. It returns 0 when
// nothing plausible is found.
func (c *TVDB) SearchSeries(ctx context.Context, title string, year int) (int64, error) {
	pick := func(results []tvdbSearchResult) int64 {
		want := Normalize(title)
		for _, r := range results {
			if Normalize(r.Name) == want || Normalize(r.Translations["eng"]) == want {
				id, _ := strconv.ParseInt(r.TVDBID, 10, 64)
				return id
			}
		}
		id, _ := strconv.ParseInt(results[0].TVDBID, 10, 64)
		return id
	}

	results, err := c.searchSeries(ctx, title, year)
	if err != nil {
		record(ctx, Step{Provider: "tvdb", Kind: "search", Query: title, Year: year, Error: err.Error()})
		return 0, err
	}
	if len(results) > 0 {
		id := pick(results)
		record(ctx, Step{Provider: "tvdb", Kind: "search", Query: title, Year: year, Results: tvdbCandidates(results), Picked: id})
		return id, nil
	}
	if year == 0 {
		record(ctx, Step{Provider: "tvdb", Kind: "search", Query: title, Note: "No results."})
		return 0, nil
	}
	record(ctx, Step{Provider: "tvdb", Kind: "search", Query: title, Year: year,
		Note: "Nothing with this title started in " + strconv.Itoa(year) + ", so Arc searched again without the year."})
	if results, err = c.searchSeries(ctx, title, 0); err != nil {
		record(ctx, Step{Provider: "tvdb", Kind: "search", Query: title, Error: err.Error()})
		return 0, err
	}
	var near []tvdbSearchResult
	for _, r := range results {
		if y, _ := strconv.Atoi(r.Year); y >= year-1 && y <= year+1 {
			near = append(near, r)
		}
	}
	if len(near) == 0 {
		note := "No results."
		if len(results) > 0 {
			note = "None of these started within a year of " + strconv.Itoa(year) + ", so none was used."
		}
		record(ctx, Step{Provider: "tvdb", Kind: "search", Query: title, Results: tvdbCandidates(results), Note: note})
		return 0, nil
	}
	id := pick(near)
	record(ctx, Step{Provider: "tvdb", Kind: "search", Query: title, Results: tvdbCandidates(results), Picked: id})
	return id, nil
}

// Series fetches series metadata and the season types (orderings) it has.
func (c *TVDB) Series(ctx context.Context, id int64) (*Series, []string, error) {
	var resp struct {
		Data struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Overview   string `json:"overview"`
			FirstAired string `json:"firstAired"`
			LastAired  string `json:"lastAired"`
			NextAired  string `json:"nextAired"`
			Status     struct {
				Name string `json:"name"`
			} `json:"status"`
			OriginalLanguage string `json:"originalLanguage"`
			Year             string `json:"year"`
			Image            string `json:"image"`
			SeasonTypes      []struct {
				Type string `json:"type"`
			} `json:"seasonTypes"`
			RemoteIDs []struct {
				ID   string `json:"id"`
				Type int    `json:"type"`
			} `json:"remoteIds"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/series/"+strconv.FormatInt(id, 10)+"/extended", url.Values{"short": {"true"}}, &resp); err != nil {
		return nil, nil, err
	}
	d := resp.Data
	s := &Series{TVDBID: d.ID, Title: d.Name, OriginalTitle: d.Name, Overview: d.Overview,
		FirstAired: d.FirstAired, PosterPath: tvdbImage(d.Image)}
	s.OriginalLanguage, s.Status, s.LastAired, s.NextAired = d.OriginalLanguage, showStatus(d.Status.Name), d.LastAired, d.NextAired
	s.Year, _ = strconv.Atoi(d.Year)
	for _, r := range d.RemoteIDs {
		switch r.Type {
		case 2:
			s.IMDBID = r.ID
		case 12:
			s.TMDBID, _ = strconv.ParseInt(r.ID, 10, 64)
		}
	}

	// The title and overview in the library's language, then English, when TheTVDB has them.
	langs := []string{"eng"}
	if l := tvdbLanguage(ctx); l != "eng" {
		langs = []string{l, "eng"}
	}
	for i := len(langs) - 1; i >= 0; i-- { // English first, so the library's language wins
		var tr struct {
			Data struct {
				Name     string `json:"name"`
				Overview string `json:"overview"`
			} `json:"data"`
		}
		err := c.get(ctx, "/series/"+strconv.FormatInt(id, 10)+"/translations/"+langs[i], nil, &tr)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return nil, nil, err
		}
		if tr.Data.Name != "" {
			s.Title = tr.Data.Name
		}
		if tr.Data.Overview != "" {
			s.Overview = tr.Data.Overview
		}
	}

	var types []string
	for _, t := range d.SeasonTypes {
		types = append(types, t.Type)
	}
	return s, types, nil
}

// Orderings fetches every supported season type of a series as an ordering.
func (c *TVDB) Orderings(ctx context.Context, id int64, seasonTypes []string) ([]Ordering, error) {
	has := map[string]bool{}
	for _, t := range seasonTypes {
		has[t] = true
	}
	var out []Ordering
	for _, st := range tvdbSeasonTypes {
		if !has[st.typ] {
			continue
		}
		entries, err := c.episodes(ctx, id, st.typ)
		if err != nil {
			return nil, fmt.Errorf("tvdb %s episodes: %w", st.typ, err)
		}
		if len(entries) == 0 {
			continue
		}
		o := Ordering{Source: st.source, Name: st.name, Description: "TVDB", Seasons: map[int]string{}, Entries: entries}
		for _, e := range entries {
			o.Seasons[e.Season] = seasonName(e.Season)
		}
		out = append(out, o)
	}
	return out, nil
}

// episodes fetches a season type's episodes in the library's language, with English for any
// episode that isn't translated.
func (c *TVDB) episodes(ctx context.Context, id int64, seasonType string) ([]Entry, error) {
	lang := tvdbLanguage(ctx)
	entries, err := c.episodesIn(ctx, id, seasonType, lang)
	if err != nil || lang == "eng" {
		return entries, err
	}
	missing := false
	for _, e := range entries {
		missing = missing || e.Episode.Title == "" || e.Episode.Overview == ""
	}
	if !missing {
		return entries, nil
	}
	en, err := c.episodesIn(ctx, id, seasonType, "eng")
	if err != nil {
		return nil, err
	}
	byID := map[int64]*Episode{}
	for _, e := range en {
		byID[e.Episode.ID] = e.Episode
	}
	for _, e := range entries {
		if eng := byID[e.Episode.ID]; eng != nil {
			if e.Episode.Title == "" {
				e.Episode.Title = eng.Title
			}
			if e.Episode.Overview == "" {
				e.Episode.Overview = eng.Overview
			}
		}
	}
	for _, e := range entries {
		if e.Episode.Title == "" {
			e.Episode.Title = "Episode " + strconv.Itoa(e.Number)
		}
	}
	return entries, nil
}

func (c *TVDB) episodesIn(ctx context.Context, id int64, seasonType, lang string) ([]Entry, error) {
	var entries []Entry
	for page := 0; ; page++ {
		var resp struct {
			Data struct {
				Episodes []struct {
					ID           int64  `json:"id"`
					SeasonNumber int    `json:"seasonNumber"`
					Number       int    `json:"number"`
					Name         string `json:"name"`
					Overview     string `json:"overview"`
					Aired        string `json:"aired"`
					Runtime      int    `json:"runtime"`
					Image        string `json:"image"`
				} `json:"episodes"`
			} `json:"data"`
			Links struct {
				Next *string `json:"next"`
			} `json:"links"`
		}
		path := fmt.Sprintf("/series/%d/episodes/%s/%s", id, seasonType, lang)
		if err := c.get(ctx, path, url.Values{"page": {strconv.Itoa(page)}}, &resp); err != nil {
			return nil, err
		}
		for _, e := range resp.Data.Episodes {
			title := e.Name
			if title == "" && lang == "eng" {
				title = "Episode " + strconv.Itoa(e.Number)
			}
			entries = append(entries, Entry{Season: e.SeasonNumber, Number: e.Number, Episode: &Episode{
				Provider: "tvdb", ID: e.ID, Title: title, Overview: e.Overview, AirDate: e.Aired,
				RuntimeMin: e.Runtime, Still: tvdbImage(e.Image),
			}})
		}
		if resp.Links.Next == nil || *resp.Links.Next == "" || len(resp.Data.Episodes) == 0 {
			return entries, nil
		}
	}
}

// tvdbImage turns TVDB's relative artwork paths ("/banners/...") into full URLs.
func tvdbImage(path string) string {
	if path == "" || strings.HasPrefix(path, "http") {
		return path
	}
	return "https://artworks.thetvdb.com" + path
}

func (c *TVDB) get(ctx context.Context, path string, q url.Values, out any) error {
	u := tvdbBase + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	for attempt := 0; ; attempt++ {
		token, err := c.ensureToken(ctx, attempt > 0)
		if err != nil {
			return err
		}
		if err := c.throttle(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		switch {
		case resp.StatusCode == http.StatusUnauthorized && attempt == 0:
			resp.Body.Close()
			continue // token expired; log in again
		case resp.StatusCode == http.StatusTooManyRequests && attempt < 3:
			resp.Body.Close()
			if err := sleep(ctx, 2*time.Second); err != nil {
				return err
			}
			continue
		case resp.StatusCode == http.StatusNotFound:
			resp.Body.Close()
			return ErrNotFound
		case resp.StatusCode != http.StatusOK:
			resp.Body.Close()
			return fmt.Errorf("tvdb %s: %s", path, resp.Status)
		}
		err = json.NewDecoder(resp.Body).Decode(out)
		resp.Body.Close()
		return err
	}
}

// ensureToken logs in when there is no token, it is older than 25 days (they last a month),
// or force is set.
func (c *TVDB) ensureToken(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && time.Since(c.tokenAt) < 25*24*time.Hour {
		return c.token, nil
	}
	body, _ := json.Marshal(map[string]string{"apikey": c.key})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tvdbBase+"/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
		Message string `json:"message"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != http.StatusOK || out.Data.Token == "" {
		return "", fmt.Errorf("tvdb login: %s %s", resp.Status, strings.TrimSpace(out.Message))
	}
	c.token, c.tokenAt = out.Data.Token, time.Now()
	return c.token, nil
}

func (c *TVDB) throttle(ctx context.Context) error {
	c.mu.Lock()
	now := time.Now()
	wait := c.next.Sub(now)
	c.next = now.Add(max(wait, 0) + 50*time.Millisecond)
	c.mu.Unlock()
	if wait > 0 {
		return sleep(ctx, wait)
	}
	return nil
}
