package metadata

// Details are the extras a movie or show has beyond its title and overview.
type Details struct {
	Tagline string
	// Age rating for the configured region (e.g. "15", "PG-13"), or another region's if it has none.
	ContentRating string
	// Studios (movies) or networks (shows).
	Studios []string
	// Audience score out of 10.
	Rating float64
	Genres []string
	People []Credit
}

// Credit is a person's part in a movie or show.
type Credit struct {
	TMDBID      int64
	Name        string
	ProfilePath string // TMDB image path
	Role        string // actor, director, writer or creator
	Character   string
	Order       int
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
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
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
		out = append(out, Credit{TMDBID: c.ID, Name: c.Name, ProfilePath: c.ProfilePath, Role: "actor", Character: character, Order: i})
	}
	seen := map[[2]any]bool{}
	for _, c := range crew {
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
