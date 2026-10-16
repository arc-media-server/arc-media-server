package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TMDB episode group types.
var tmdbGroupTypes = map[int]string{
	1: "Original Air Date", 2: "Absolute", 3: "DVD", 4: "Digital", 5: "Story Arc", 6: "Production", 7: "TV",
}

type tmdbTVResult struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	OriginalName string `json:"original_name"`
	FirstAirDate string `json:"first_air_date"`
}

// SearchTV finds a TMDB series id by title and optional first-aired year, or 0.
func (c *TMDB) SearchTV(ctx context.Context, title string, year int) (int64, error) {
	search := func(withYear bool) ([]tmdbTVResult, error) {
		q := url.Values{"query": {title}}
		if withYear {
			q.Set("first_air_date_year", strconv.Itoa(year))
		}
		var resp struct {
			Results []tmdbTVResult `json:"results"`
		}
		err := c.get(ctx, "/search/tv", q, &resp)
		return resp.Results, err
	}
	pick := func(results []tmdbTVResult) int64 {
		want := Normalize(title)
		for _, r := range results {
			if Normalize(r.Name) == want || Normalize(r.OriginalName) == want {
				return r.ID
			}
		}
		return results[0].ID
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
	var near []tmdbTVResult
	for _, r := range results {
		if y := yearOf(r.FirstAirDate); y >= year-1 && y <= year+1 {
			near = append(near, r)
		}
	}
	if len(near) == 0 {
		return 0, nil
	}
	return pick(near), nil
}

// FindTV looks up a TMDB series id by an external id. source is "tvdb_id" or "imdb_id".
func (c *TMDB) FindTV(ctx context.Context, externalID, source string) (int64, error) {
	var resp struct {
		TVResults []tmdbTVResult `json:"tv_results"`
	}
	err := c.get(ctx, "/find/"+url.PathEscape(externalID), url.Values{"external_source": {source}}, &resp)
	if errors.Is(err, errNotFound) || (err == nil && len(resp.TVResults) == 0) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return resp.TVResults[0].ID, nil
}

type tmdbEpisode struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Overview      string `json:"overview"`
	AirDate       string `json:"air_date"`
	Runtime       int    `json:"runtime"`
	StillPath     string `json:"still_path"`
	SeasonNumber  int    `json:"season_number"`
	EpisodeNumber int    `json:"episode_number"`
	Order         int    `json:"order"`
}

func (e tmdbEpisode) toEpisode() *Episode {
	return &Episode{Provider: "tmdb", ID: e.ID, Title: e.Name, Overview: e.Overview, AirDate: e.AirDate,
		RuntimeMin: e.Runtime, Still: e.StillPath, AiredSeason: e.SeasonNumber, AiredNumber: e.EpisodeNumber}
}

// TV fetches series metadata plus all of its orderings: TMDB aired order and every episode group.
func (c *TMDB) TV(ctx context.Context, id int64) (*Series, []Ordering, error) {
	var d struct {
		ID           int64  `json:"id"`
		Name         string `json:"name"`
		OriginalName string `json:"original_name"`
		Overview     string `json:"overview"`
		FirstAirDate string `json:"first_air_date"`
		PosterPath   string `json:"poster_path"`
		BackdropPath string `json:"backdrop_path"`
		ExternalIDs  struct {
			IMDBID string `json:"imdb_id"`
			TVDBID int64  `json:"tvdb_id"`
		} `json:"external_ids"`
		Seasons []struct {
			SeasonNumber int    `json:"season_number"`
			Name         string `json:"name"`
		} `json:"seasons"`
		EpisodeGroups struct {
			Results []struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				Type         int    `json:"type"`
				EpisodeCount int    `json:"episode_count"`
			} `json:"results"`
		} `json:"episode_groups"`
	}
	base := "/tv/" + strconv.FormatInt(id, 10)
	if err := c.get(ctx, base, url.Values{"append_to_response": {"external_ids,episode_groups"}}, &d); err != nil {
		return nil, nil, err
	}
	s := &Series{TMDBID: d.ID, TVDBID: d.ExternalIDs.TVDBID, IMDBID: d.ExternalIDs.IMDBID, Title: d.Name,
		OriginalTitle: d.OriginalName, Overview: d.Overview, FirstAired: d.FirstAirDate,
		Year: yearOf(d.FirstAirDate), PosterPath: d.PosterPath, BackdropPath: d.BackdropPath}

	// Aired order: fetch seasons 20 at a time using append_to_response.
	aired := Ordering{Source: "tmdb_aired", Name: "Aired Order", Description: "TMDB", Seasons: map[int]string{}}
	var nums []int
	for _, season := range d.Seasons {
		nums = append(nums, season.SeasonNumber)
		aired.Seasons[season.SeasonNumber] = season.Name
	}
	for start := 0; start < len(nums); start += 20 {
		batch := nums[start:min(start+20, len(nums))]
		var parts []string
		for _, n := range batch {
			parts = append(parts, "season/"+strconv.Itoa(n))
		}
		var raw map[string]json.RawMessage
		if err := c.get(ctx, base, url.Values{"append_to_response": {strings.Join(parts, ",")}}, &raw); err != nil {
			return nil, nil, err
		}
		for _, n := range batch {
			data, ok := raw["season/"+strconv.Itoa(n)]
			if !ok {
				continue
			}
			var season struct {
				Episodes []tmdbEpisode `json:"episodes"`
			}
			if err := json.Unmarshal(data, &season); err != nil {
				return nil, nil, fmt.Errorf("tmdb season %d: %w", n, err)
			}
			for _, e := range season.Episodes {
				aired.Entries = append(aired.Entries, Entry{Season: e.SeasonNumber, Number: e.EpisodeNumber, Episode: e.toEpisode()})
			}
		}
	}
	orderings := []Ordering{aired}

	for _, g := range d.EpisodeGroups.Results {
		o, err := c.episodeGroup(ctx, g.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("tmdb episode group %s: %w", g.ID, err)
		}
		o.Name = g.Name
		o.Description = "TMDB · " + tmdbGroupTypes[g.Type]
		orderings = append(orderings, *o)
	}
	return s, orderings, nil
}

var specialsRe = regexp.MustCompile(`(?i)^\s*(specials?|extras?)\s*$`)

func (c *TMDB) episodeGroup(ctx context.Context, groupID string) (*Ordering, error) {
	var d struct {
		Groups []struct {
			Name     string        `json:"name"`
			Order    int           `json:"order"`
			Episodes []tmdbEpisode `json:"episodes"`
		} `json:"groups"`
	}
	if err := c.get(ctx, "/tv/episode_group/"+url.PathEscape(groupID), nil, &d); err != nil {
		return nil, err
	}
	sort.SliceStable(d.Groups, func(i, j int) bool { return d.Groups[i].Order < d.Groups[j].Order })

	o := &Ordering{Source: "tmdb_group", SourceRef: groupID, Seasons: map[int]string{}}
	// Groups become seasons 1..n in order; a group named "Specials" becomes season 0.
	next := 1
	for _, g := range d.Groups {
		season := 0
		if !specialsRe.MatchString(g.Name) {
			season = next
			next++
		}
		o.Seasons[season] = g.Name
		sort.SliceStable(g.Episodes, func(i, j int) bool { return g.Episodes[i].Order < g.Episodes[j].Order })
		for i, e := range g.Episodes {
			o.Entries = append(o.Entries, Entry{Season: season, Number: i + 1, Episode: e.toEpisode()})
		}
	}
	return o, nil
}

// EpisodeTVDBID returns the TVDB id TMDB records for an episode, or 0.
func (c *TMDB) EpisodeTVDBID(ctx context.Context, seriesID int64, season, episode int) (int64, error) {
	var d struct {
		TVDBID int64 `json:"tvdb_id"`
	}
	path := fmt.Sprintf("/tv/%d/season/%d/episode/%d/external_ids", seriesID, season, episode)
	if err := c.get(ctx, path, nil, &d); err != nil {
		if errors.Is(err, errNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return d.TVDBID, nil
}
