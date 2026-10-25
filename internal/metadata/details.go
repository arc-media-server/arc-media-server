package metadata

import "strings"

// Details are the extras a movie or show has beyond its title and overview.
type Details struct {
	Tagline string
	// The title as transparent artwork (a clear logo), TMDB path; "" if there's none.
	LogoPath string
	// Age rating for the configured region (e.g. "15", "PG-13"), or another region's if it has none.
	ContentRating string
	// Studios (movies) or networks (shows).
	Studios []string
	// Audience score out of 10.
	Rating float64
	Genres []string
	People []Credit
	// TMDB's recommendations (TMDB ids, best first), for "More like this".
	Recommendations []int64
	// Other names it's known by (other countries' and languages' titles), and TMDB keywords.
	AltTitles []string
	Keywords  []string
	// The language it was made in, as the provider gives it ("en" from TMDB, "eng" from TVDB).
	OriginalLanguage string
	// Shows only: continuing, ended, cancelled or upcoming ("" if unknown), and the dates the
	// last episode aired and the next one airs (YYYY-MM-DD).
	Status               string
	LastAired, NextAired string
}

// tmdbAltTitles is TMDB's alternative titles: "titles" for movies, "results" for shows.
type tmdbAltTitles struct {
	Titles []struct {
		Title string `json:"title"`
	} `json:"titles"`
	Results []struct {
		Title string `json:"title"`
	} `json:"results"`
}

func (a tmdbAltTitles) list(except ...string) []string {
	skip := map[string]bool{}
	for _, e := range except {
		skip[e] = true
	}
	var out []string
	for _, t := range append(a.Titles, a.Results...) {
		if t.Title != "" && !skip[t.Title] {
			skip[t.Title] = true
			out = append(out, t.Title)
		}
	}
	return out
}

// tmdbKeywords is TMDB's keywords: "keywords" for movies, "results" for shows.
type tmdbKeywords struct {
	Keywords []tmdbNamed `json:"keywords"`
	Results  []tmdbNamed `json:"results"`
}

func (k tmdbKeywords) list() []string { return names(append(k.Keywords, k.Results...)) }

// showStatus turns TMDB's and TheTVDB's show statuses into Arc's.
func showStatus(s string) string {
	switch strings.ToLower(s) {
	case "returning series", "continuing":
		return "continuing"
	case "ended":
		return "ended"
	case "canceled", "cancelled":
		return "cancelled"
	case "in production", "planned", "pilot", "upcoming":
		return "upcoming"
	}
	return ""
}

type tmdbRecommendations struct {
	Results []struct {
		ID int64 `json:"id"`
	} `json:"results"`
}

func (r tmdbRecommendations) ids() []int64 {
	out := make([]int64, 0, len(r.Results))
	for _, x := range r.Results {
		out = append(out, x.ID)
	}
	return out
}

// Credit is a person's part in a movie or show.
type Credit struct {
	TMDBID       int64
	Name         string
	ProfilePath  string // TMDB image path
	Role         string // actor, director, writer or creator
	Character    string
	Order        int
	EpisodeCount int // Provider-supplied aggregate total; 0 is unknown.
}

// How many actors are kept per title.
const maxCast = 20

// pickRating chooses the age rating for region, falling back to the US and then to any rating.
func pickRating(region string, byRegion map[string]string) string {
	for _, r := range []string{region, "US"} {
		if v := byRegion[r]; v != "" {
			return v
		}
	}
	for _, v := range byRegion {
		if v != "" {
			return v
		}
	}
	return ""
}

type tmdbNamed struct {
	Name string `json:"name"`
}

func names(list []tmdbNamed) []string {
	out := make([]string, 0, len(list))
	for _, n := range list {
		if n.Name != "" {
			out = append(out, n.Name)
		}
	}
	return out
}

type tmdbCastMember struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	ProfilePath       string `json:"profile_path"`
	Character         string `json:"character"`
	Order             int    `json:"order"`
	TotalEpisodeCount int    `json:"total_episode_count"`
	// TV aggregate credits list each actor's roles instead.
	Roles []struct {
		Character string `json:"character"`
	} `json:"roles"`
}

type tmdbCrewMember struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	Jobs        []struct {
		Job string `json:"job"`
	} `json:"jobs"`
}

// credits turns TMDB cast and crew into Credits: the first actors, and directors and writers.
func credits(cast []tmdbCastMember, crew []tmdbCrewMember) []Credit {
	var out []Credit
	for i, c := range cast {
		if i >= maxCast {
			break
		}
		character := c.Character
		if character == "" && len(c.Roles) > 0 {
			character = c.Roles[0].Character
		}
		if c.ID <= 0 || strings.TrimSpace(c.Name) == "" {
			continue
		}
		out = append(out, Credit{TMDBID: c.ID, Name: c.Name, ProfilePath: c.ProfilePath, Role: "actor", Character: character, Order: i, EpisodeCount: max(0, c.TotalEpisodeCount)})
	}
	seen := map[[2]any]bool{}
	for _, c := range crew {
		if c.ID <= 0 || strings.TrimSpace(c.Name) == "" {
			continue
		}
		jobs := []string{c.Job}
		for _, j := range c.Jobs {
			jobs = append(jobs, j.Job)
		}
		for _, job := range jobs {
			role := ""
			switch job {
			case "Director":
				role = "director"
			case "Screenplay", "Writer", "Story", "Teleplay":
				role = "writer"
			}
			if role == "" || seen[[2]any{c.ID, role}] {
				continue
			}
			seen[[2]any{c.ID, role}] = true
			out = append(out, Credit{TMDBID: c.ID, Name: c.Name, ProfilePath: c.ProfilePath, Role: role, Order: len(out)})
		}
	}
	return out
}
