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
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	VoteCount    int    `json:"vote_count"`
}

func tvCandidates(results []tmdbTVResult) []Candidate {
	out := make([]Candidate, 0, len(results))
	for _, r := range results {
		c := Candidate{Provider: "tmdb", ID: r.ID, Title: r.Name, Year: yearOf(r.FirstAirDate), Overview: r.Overview, Votes: r.VoteCount}
		if r.OriginalName != r.Name {
			c.OriginalTitle = r.OriginalName
		}
		if r.PosterPath != "" {
			c.Poster = ImageURL(r.PosterPath, "w185")
		}
		out = append(out, c)
	}
	return out
}

func (c *TMDB) searchTV(ctx context.Context, title string, year int) ([]tmdbTVResult, error) {
	q := url.Values{"query": {title}}
	if year > 0 {
		q.Set("first_air_date_year", strconv.Itoa(year))
	}
	var resp struct {
		Results []tmdbTVResult `json:"results"`
	}
	err := c.get(ctx, "/search/tv", q, &resp)
	return resp.Results, err
}

// SearchTVShows searches TMDB for series by title (and first-aired year, if not 0) for someone
// choosing a match by hand.
func (c *TMDB) SearchTVShows(ctx context.Context, title string, year int) ([]Candidate, error) {
	results, err := c.searchTV(ctx, title, year)
	return tvCandidates(results), err
}

// SearchTV finds a TMDB series id by title and optional first-aired year, or 0.
func (c *TMDB) SearchTV(ctx context.Context, title string, year int) (int64, error) {
	pick := func(results []tmdbTVResult) int64 {
		return confidentMatch(title, year, tvCandidates(results))
	}

	results, err := c.searchTV(ctx, title, year)
	if err != nil {
		record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Year: year, Error: err.Error()})
		return 0, err
	}
	if len(results) > 0 {
		id := pick(results)
		record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Year: year, Results: tvCandidates(results), Picked: id})
		return id, nil
	}
	if year == 0 {
		record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Note: "No results."})
		return 0, nil
	}
	record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Year: year,
		Note: "Nothing with this title started in " + strconv.Itoa(year) + ", so Arc searched again without the year."})
	if results, err = c.searchTV(ctx, title, 0); err != nil {
		record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Error: err.Error()})
		return 0, err
	}
	var near []tmdbTVResult
	for _, r := range results {
		if y := yearOf(r.FirstAirDate); y >= year-1 && y <= year+1 {
			near = append(near, r)
		}
	}
	if len(near) == 0 {
		note := "No results."
		if len(results) > 0 {
			note = "None of these started within a year of " + strconv.Itoa(year) + ", so none was used."
		}
		record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Results: tvCandidates(results), Note: note})
		return 0, nil
	}
	id := pick(near)
	record(ctx, Step{Provider: "tmdb", Kind: "search", Query: title, Results: tvCandidates(results), Picked: id})
	return id, nil
}

// FindTV looks up a TMDB series id by an external id. source is "tvdb_id" or "imdb_id".
func (c *TMDB) FindTV(ctx context.Context, externalID, source string) (int64, error) {
	var resp struct {
		TVResults []tmdbTVResult `json:"tv_results"`
	}
	err := c.get(ctx, "/find/"+url.PathEscape(externalID), url.Values{"external_source": {source}}, &resp)
	query := strings.TrimSuffix(source, "_id") + " " + externalID
	if err != nil && !errors.Is(err, ErrNotFound) {
		record(ctx, Step{Provider: "tmdb", Kind: "find", Query: query, Error: err.Error()})
		return 0, err
	}
	if len(resp.TVResults) == 0 {
		record(ctx, Step{Provider: "tmdb", Kind: "find", Query: query, Note: "TMDB doesn't link any show to this id."})
		return 0, nil
	}
	record(ctx, Step{Provider: "tmdb", Kind: "find", Query: query, Results: tvCandidates(resp.TVResults), Picked: resp.TVResults[0].ID})
	return resp.TVResults[0].ID, nil
}

type tmdbEpisode struct {
	ID            int64            `json:"id"`
	Name          string           `json:"name"`
	Overview      string           `json:"overview"`
	AirDate       string           `json:"air_date"`
	Runtime       int              `json:"runtime"`
	StillPath     string           `json:"still_path"`
	SeasonNumber  int              `json:"season_number"`
	EpisodeNumber int              `json:"episode_number"`
	Order         int              `json:"order"`
	GuestStars    []tmdbCastMember `json:"guest_stars"`
	Crew          []tmdbCrewMember `json:"crew"`
}

func (e tmdbEpisode) toEpisode() *Episode {
	return &Episode{Provider: "tmdb", ID: e.ID, Title: e.Name, Overview: e.Overview, AirDate: e.AirDate,
		RuntimeMin: e.Runtime, Still: e.StillPath, AiredSeason: e.SeasonNumber, AiredNumber: e.EpisodeNumber, People: credits(e.GuestStars, e.Crew)}
}

// TVSummary fetches only a show's own record (title, year, overview, artwork and its TheTVDB and
// IMDb ids) in one request, without seasons, episodes or episode groups: enough to show which
// show an item is while TV fetches the rest. ErrNotFound means TMDB has no show with that id.
func (c *TMDB) TVSummary(ctx context.Context, id int64) (*Series, error) {
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
	}
	if err := c.get(ctx, "/tv/"+strconv.FormatInt(id, 10), url.Values{"append_to_response": {"external_ids"}}, &d); err != nil {
		return nil, err
	}
	return &Series{TMDBID: d.ID, TVDBID: d.ExternalIDs.TVDBID, IMDBID: d.ExternalIDs.IMDBID, Title: d.Name,
		OriginalTitle: d.OriginalName, Overview: d.Overview, FirstAired: d.FirstAirDate, Year: yearOf(d.FirstAirDate),
		PosterPath: d.PosterPath, BackdropPath: d.BackdropPath}, nil
}

// TV fetches series metadata plus all of its orderings: TMDB aired order and every episode group.
func (c *TMDB) TV(ctx context.Context, id int64) (*Series, []Ordering, error) {
	var d struct {
		ID               int64  `json:"id"`
		Name             string `json:"name"`
		OriginalName     string `json:"original_name"`
		OriginalLanguage string `json:"original_language"`
		Status           string `json:"status"`
		LastAirDate      string `json:"last_air_date"`
		NextEpisode      *struct {
			AirDate string `json:"air_date"`
		} `json:"next_episode_to_air"`
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
			PosterPath   string `json:"poster_path"`
		} `json:"seasons"`
		EpisodeGroups struct {
			Results []struct {
				ID           string `json:"id"`
				Name         string `json:"name"`
				Type         int    `json:"type"`
				EpisodeCount int    `json:"episode_count"`
			} `json:"results"`
		} `json:"episode_groups"`
		Tagline        string           `json:"tagline"`
		VoteAverage    float64          `json:"vote_average"`
		Genres         []tmdbNamed      `json:"genres"`
		Networks       []tmdbNamed      `json:"networks"`
		CreatedBy      []tmdbCastMember `json:"created_by"`
		ContentRatings struct {
			Results []struct {
				Country string `json:"iso_3166_1"`
				Rating  string `json:"rating"`
			} `json:"results"`
		} `json:"content_ratings"`
		AggregateCredits struct {
			Cast []tmdbCastMember `json:"cast"`
		} `json:"aggregate_credits"`
		Images struct {
			Logos     []tmdbImage `json:"logos"`
			Backdrops []tmdbImage `json:"backdrops"`
		} `json:"images"`
		Recommendations tmdbRecommendations `json:"recommendations"`
		AltTitles       tmdbAltTitles       `json:"alternative_titles"`
		Keywords        tmdbKeywords        `json:"keywords"`
	}
	base := "/tv/" + strconv.FormatInt(id, 10)
	q := url.Values{"append_to_response": {"external_ids,episode_groups,content_ratings,aggregate_credits,images,recommendations,alternative_titles,keywords"},
		"include_image_language": {"en,null"}}
	if err := c.get(ctx, base, q, &d); err != nil {
		return nil, nil, err
	}
	// Untranslated titles and descriptions come from English instead.
	if needsEnglish(ctx) && (d.Overview == "" || untranslatedTitle(ctx, d.Name, d.OriginalName, d.OriginalLanguage)) {
		var en struct {
			Name     string `json:"name"`
			Overview string `json:"overview"`
			Tagline  string `json:"tagline"`
		}
		if err := c.get(english(ctx), "/tv/"+strconv.FormatInt(id, 10), nil, &en); err != nil {
			return nil, nil, err
		}
		if untranslatedTitle(ctx, d.Name, d.OriginalName, d.OriginalLanguage) && en.Name != "" {
			d.Name = en.Name
		}
		if d.Overview == "" {
			d.Overview = en.Overview
		}
		if d.Tagline == "" {
			d.Tagline = en.Tagline
		}
	}
	ratings := map[string]string{}
	for _, r := range d.ContentRatings.Results {
		ratings[r.Country] = r.Rating
	}
	people := credits(d.AggregateCredits.Cast, nil)
	for _, cr := range d.CreatedBy {
		people = append(people, Credit{TMDBID: cr.ID, Name: cr.Name, ProfilePath: cr.ProfilePath, Role: "creator", Order: len(people)})
	}
	s := &Series{TMDBID: d.ID, TVDBID: d.ExternalIDs.TVDBID, IMDBID: d.ExternalIDs.IMDBID, Title: d.Name,
		OriginalTitle: d.OriginalName, Overview: d.Overview, FirstAired: d.FirstAirDate,
		Year: yearOf(d.FirstAirDate), PosterPath: d.PosterPath, BackdropPath: bestBackdrop(d.BackdropPath, d.Images.Backdrops),
		Details: Details{Tagline: d.Tagline, LogoPath: bestLogo(d.Images.Logos), Recommendations: d.Recommendations.ids(),
			AltTitles: d.AltTitles.list(d.Name, d.OriginalName), Keywords: d.Keywords.list(), ContentRating: pickRating(c.ratingRegion(ctx), ratings), Studios: names(d.Networks),
			Rating: d.VoteAverage, Genres: names(d.Genres), People: people,
			OriginalLanguage: d.OriginalLanguage, Status: showStatus(d.Status), LastAired: d.LastAirDate}}
	if d.NextEpisode != nil {
		s.NextAired = d.NextEpisode.AirDate
	}

	// Aired order: fetch seasons 20 at a time using append_to_response.
	aired := Ordering{Source: "tmdb_aired", Name: "Aired Order", Description: "TMDB", Seasons: map[int]string{}, SeasonOverviews: map[int]string{}}
	var nums []int
	s.SeasonPosters = map[int]string{}
	for _, season := range d.Seasons {
		if season.PosterPath != "" {
			s.SeasonPosters[season.SeasonNumber] = season.PosterPath
		}
		nums = append(nums, season.SeasonNumber)
		aired.Seasons[season.SeasonNumber] = season.Name
	}
	for start := 0; start < len(nums); start += 20 {
		batch := nums[start:min(start+20, len(nums))]
		var parts []string
		for _, n := range batch {
			parts = append(parts, "season/"+strconv.Itoa(n))
		}
		seasons, err := c.seasons(ctx, base, parts)
		if err != nil {
			return nil, nil, err
		}
		// Episodes not translated yet get their English title and description.
		if needsEnglish(ctx) && untranslatedEpisodes(seasons) {
			en, err := c.seasons(english(ctx), base, parts)
			if err != nil {
				return nil, nil, err
			}
			fillFromEnglish(seasons, en)
		}
		for _, n := range batch {
			season, ok := seasons["season/"+strconv.Itoa(n)]
			if !ok {
				continue
			}
			if season.Overview != "" {
				aired.SeasonOverviews[n] = season.Overview
			}
			for _, e := range season.Episodes {
				aired.Entries = append(aired.Entries, Entry{Season: e.SeasonNumber, Number: e.EpisodeNumber, Episode: e.toEpisode()})
			}
		}
	}
	orderings := []Ordering{aired}
	if essentialSource(ctx) != "" {
		return s, orderings, nil
	}

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

type tmdbSeason struct {
	Overview string        `json:"overview"`
	Episodes []tmdbEpisode `json:"episodes"`
}

// seasons fetches several seasons of a show at once (parts like "season/1"), keyed by part.
func (c *TMDB) seasons(ctx context.Context, base string, parts []string) (map[string]*tmdbSeason, error) {
	var raw map[string]json.RawMessage
	if err := c.get(ctx, base, url.Values{"append_to_response": {strings.Join(parts, ",")}}, &raw); err != nil {
		return nil, err
	}
	out := map[string]*tmdbSeason{}
	for _, p := range parts {
		data, ok := raw[p]
		if !ok || string(data) == "null" {
			continue
		}
		var s tmdbSeason
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("tmdb %s: %w", p, err)
		}
		out[p] = &s
	}
	return out, nil
}

// untranslatedEpisodes reports whether any season or episode is missing a description or has
// only a stand-in name.
func untranslatedEpisodes(seasons map[string]*tmdbSeason) bool {
	for _, s := range seasons {
		for _, e := range s.Episodes {
			if e.Overview == "" || genericEpisodeName.MatchString(e.Name) {
				return true
			}
		}
	}
	return false
}

// fillFromEnglish gives untranslated seasons and episodes their English titles and descriptions.
func fillFromEnglish(seasons, en map[string]*tmdbSeason) {
	for p, s := range seasons {
		e, ok := en[p]
		if !ok {
			continue
		}
		if s.Overview == "" {
			s.Overview = e.Overview
		}
		byID := map[int64]tmdbEpisode{}
		for _, ep := range e.Episodes {
			byID[ep.ID] = ep
		}
		for i := range s.Episodes {
			ep := &s.Episodes[i]
			eng, ok := byID[ep.ID]
			if !ok {
				continue
			}
			if genericEpisodeName.MatchString(ep.Name) && !genericEpisodeName.MatchString(eng.Name) {
				ep.Name = eng.Name
			}
			if ep.Overview == "" {
				ep.Overview = eng.Overview
			}
		}
	}
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
		if errors.Is(err, ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return d.TVDBID, nil
}
