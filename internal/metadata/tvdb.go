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
	key  string
	http *http.Client

	mu      sync.Mutex
	token   string
	tokenAt time.Time
	next    time.Time
}

func NewTVDB(key string) *TVDB {
	return &TVDB{key: key, http: &http.Client{Timeout: 30 * time.Second}}
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

// SearchSeries finds a series id by title and optional first-aired year. It returns 0 when
// nothing plausible is found.
func (c *TVDB) SearchSeries(ctx context.Context, title string, year int) (int64, error) {
	type result struct {
		TVDBID       string            `json:"tvdb_id"`
		Name         string            `json:"name"`
		Year         string            `json:"year"`
		Translations map[string]string `json:"translations"`
		Aliases      []string          `json:"aliases"`
	}
	search := func(withYear bool) ([]result, error) {
		q := url.Values{"query": {title}, "type": {"series"}}
		if withYear {
			q.Set("year", strconv.Itoa(year))
		}
		var resp struct {
			Data []result `json:"data"`
		}
		err := c.get(ctx, "/search", q, &resp)
		return resp.Data, err
	}
	pick := func(results []result) int64 {
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

	results, err := search(year > 0)
	if err != nil {
		return 0, err
	}
	if len(results) > 0 {
		return pick(results), nil
	}
	if year == 0 {
		return 0, nil
	}
	if results, err = search(false); err != nil {
		return 0, err
	}
	var near []result
	for _, r := range results {
		if y, _ := strconv.Atoi(r.Year); y >= year-1 && y <= year+1 {
			near = append(near, r)
		}
	}
	if len(near) == 0 {
		return 0, nil
	}
	return pick(near), nil
}

// Series fetches series metadata and the season types (orderings) it has.
func (c *TVDB) Series(ctx context.Context, id int64) (*Series, []string, error) {
	var resp struct {
		Data struct {
			ID          int64  `json:"id"`
			Name        string `json:"name"`
			Overview    string `json:"overview"`
			FirstAired  string `json:"firstAired"`
			Year        string `json:"year"`
			Image       string `json:"image"`
			SeasonTypes []struct {
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
	s.Year, _ = strconv.Atoi(d.Year)
	for _, r := range d.RemoteIDs {
		switch r.Type {
		case 2:
			s.IMDBID = r.ID
		case 12:
			s.TMDBID, _ = strconv.ParseInt(r.ID, 10, 64)
		}
	}

	// Prefer English title and overview when available.
	var tr struct {
		Data struct {
			Name     string `json:"name"`
			Overview string `json:"overview"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/series/"+strconv.FormatInt(id, 10)+"/translations/eng", nil, &tr); err == nil {
		if tr.Data.Name != "" {
			s.Title = tr.Data.Name
		}
		if tr.Data.Overview != "" {
			s.Overview = tr.Data.Overview
		}
	} else if !errors.Is(err, errNotFound) {
		return nil, nil, err
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

func (c *TVDB) episodes(ctx context.Context, id int64, seasonType string) ([]Entry, error) {
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
		path := fmt.Sprintf("/series/%d/episodes/%s/eng", id, seasonType)
		if err := c.get(ctx, path, url.Values{"page": {strconv.Itoa(page)}}, &resp); err != nil {
			return nil, err
		}
		for _, e := range resp.Data.Episodes {
			title := e.Name
			if title == "" {
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
			return errNotFound
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
