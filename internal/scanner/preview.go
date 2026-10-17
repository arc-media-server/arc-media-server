package scanner

import (
	"context"
	"io/fs"
	"sort"
	"strconv"
	"sync"

	"ams/internal/metadata"
	"ams/internal/store"
)

// Preview is what AMS would find in a set of folders, worked out without adding anything: the
// movies or shows it would create, which it could identify online (and why not, when it
// couldn't), and the files it would skip or couldn't place.
type Preview struct {
	// scanning (reading folders), identifying (looking titles up) or done.
	Stage string `json:"stage"`
	Error string `json:"error,omitempty"`
	// Folders that couldn't be read (e.g. a disconnected drive).
	Unavailable  []string      `json:"unavailable"`
	Files        int           `json:"files"`
	Extras       int           `json:"extras"`
	Unrecognised []string      `json:"unrecognised"`
	Items        []PreviewItem `json:"items"`
	// Progress through identifying Items.
	Checked int `json:"checked"`
}

// PreviewItem is one movie or show the folders hold.
type PreviewItem struct {
	Kind     string `json:"kind"` // movie or series
	Title    string `json:"title"`
	Year     int    `json:"year,omitempty"`
	Files    int    `json:"files"`
	Episodes int    `json:"episodes,omitempty"`
	// An example file, and what was read from its name.
	Example ParsedName `json:"example"`
	// pending until looked up, then identified or unidentified.
	Status string              `json:"status"`
	Match  *metadata.Candidate `json:"match,omitempty"`
	Steps  []metadata.Step     `json:"steps"`
}

// PreviewState holds a preview while it runs, safe to read from other goroutines.
type PreviewState struct {
	mu sync.Mutex
	p  Preview
}

// Snapshot returns a copy of the preview so far.
func (ps *PreviewState) Snapshot() Preview {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	p := ps.p
	p.Items = append([]PreviewItem(nil), ps.p.Items...)
	p.Unrecognised = append([]string{}, ps.p.Unrecognised...)
	p.Unavailable = append([]string{}, ps.p.Unavailable...)
	return p
}

func (ps *PreviewState) update(fn func(p *Preview)) {
	ps.mu.Lock()
	fn(&ps.p)
	ps.mu.Unlock()
}

// How many unrecognised file paths a preview lists; the rest are only counted.
const previewUnrecognisedLimit = 200

// Preview works out what a library of the given kind ("movies" or "shows") with these folders
// would contain. It only reads names and asks the metadata providers; nothing is stored and media
// files aren't opened.
func (s *Scanner) Preview(ctx context.Context, kind, sourceOrdering string, paths []string, ps *PreviewState) {
	ps.update(func(p *Preview) {
		*p = Preview{Stage: "scanning", Unavailable: []string{}, Unrecognised: []string{}, Items: []PreviewItem{}}
	})
	lib := &store.Library{Kind: kind, SourceOrdering: sourceOrdering, Paths: paths}

	type group struct {
		item     PreviewItem
		name     MovieName
		episodes map[string]bool
	}
	groups := map[string]*group{}
	var order []string
	add := func(key string, g *group) *group {
		if existing := groups[key]; existing != nil {
			return existing
		}
		groups[key] = g
		order = append(order, key)
		return g
	}

	for _, root := range paths {
		complete, err := walkVideos(ctx, root, s.log, func(path string, d fs.DirEntry) {
			ps.update(func(p *Preview) { p.Files++ })
			switch kind {
			case "movies":
				n := ParseMovie(path)
				key := "title:" + metadata.Normalize(n.Title) + ":" + strconv.Itoa(n.Year)
				if n.TMDBID != 0 {
					key = "tmdb:" + strconv.FormatInt(n.TMDBID, 10)
				} else if n.IMDBID != "" {
					key = "imdb:" + n.IMDBID
				}
				g := add(key, &group{name: n, item: PreviewItem{Kind: "movie", Title: n.Title, Year: n.Year,
					Example: ExplainMovieFile(path)}})
				g.item.Files++
			case "shows":
				key, name, ep, ok := ParseEpisodePath(root, path)
				if !ok {
					ps.update(func(p *Preview) {
						if len(p.Unrecognised) < previewUnrecognisedLimit {
							p.Unrecognised = append(p.Unrecognised, path)
						}
					})
					return
				}
				g := add(key, &group{name: name, episodes: map[string]bool{}, item: PreviewItem{Kind: "series",
					Title: name.Title, Year: name.Year, Example: ExplainEpisodeFile(lib, path)}})
				g.item.Files++
				if ep.AirDate != "" {
					g.episodes["d"+ep.AirDate] = true
				}
				for _, n := range ep.Episodes {
					g.episodes[strconv.Itoa(ep.Season)+"x"+strconv.Itoa(n)+strconv.FormatBool(ep.Absolute)] = true
				}
			}
		}, func(string) { ps.update(func(p *Preview) { p.Extras++ }) })
		if err != nil {
			ps.update(func(p *Preview) { p.Stage, p.Error = "done", err.Error() })
			return
		}
		if !complete {
			ps.update(func(p *Preview) { p.Unavailable = append(p.Unavailable, root) })
		}
	}

	items := make([]PreviewItem, 0, len(order))
	names := make([]MovieName, 0, len(order))
	for _, k := range order {
		g := groups[k]
		g.item.Episodes = len(g.episodes)
		g.item.Status = "pending"
		g.item.Steps = []metadata.Step{}
		items = append(items, g.item)
		names = append(names, g.name)
	}
	// Show them alphabetically, as the library would.
	idx := make([]int, len(items))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return store.SortTitle(items[idx[a]].Title) < store.SortTitle(items[idx[b]].Title)
	})
	sortedItems := make([]PreviewItem, len(items))
	sortedNames := make([]MovieName, len(items))
	for i, j := range idx {
		sortedItems[i], sortedNames[i] = items[j], names[j]
	}
	ps.update(func(p *Preview) { p.Items, p.Stage = sortedItems, "identifying" })

	// Look each title up, a few at a time (the provider clients pace the requests).
	work := make(chan int)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				match, steps := s.identify(ctx, kind, sortedNames[i])
				ps.update(func(p *Preview) {
					it := &p.Items[i]
					it.Match, it.Steps = match, steps
					it.Status = "unidentified"
					if match != nil {
						it.Status = "identified"
					}
					p.Checked++
				})
			}
		}()
	}
	for i := range sortedItems {
		if ctx.Err() != nil {
			break
		}
		work <- i
	}
	close(work)
	wg.Wait()
	ps.update(func(p *Preview) {
		p.Stage = "done"
		if ctx.Err() != nil && p.Error == "" {
			p.Error = "stopped"
		}
	})
}

// identify looks a movie or show up the way a scan would, without storing anything. It returns
// the match (nil if none) and the lookups made.
func (s *Scanner) identify(ctx context.Context, kind string, n MovieName) (*metadata.Candidate, []metadata.Step) {
	tr := &metadata.Trace{}
	ctx = metadata.WithTrace(ctx, tr)
	steps := func() []metadata.Step {
		if tr.Steps == nil {
			return []metadata.Step{}
		}
		return tr.Steps
	}
	noKey := func(what string) []metadata.Step {
		return []metadata.Step{{Provider: "tmdb", Kind: "search", Query: n.Title, Results: []metadata.Candidate{},
			Note: "Add a " + what + " API key to the config file so Arc can look this up."}}
	}

	if kind == "movies" {
		if !s.tmdb.Configured() {
			return nil, noKey("TMDB")
		}
		m, err := s.tmdb.MatchMovie(ctx, n.Title, n.Year, n.TMDBID, n.IMDBID)
		if err != nil || m == nil {
			return nil, steps()
		}
		c := &metadata.Candidate{Provider: "tmdb", ID: m.TMDBID, Title: m.Title, Year: m.Year}
		if m.PosterPath != "" {
			c.Poster = metadata.ImageURL(m.PosterPath, "w185")
		}
		return c, steps()
	}

	// Shows: TheTVDB first, then TMDB, as a sync does.
	if !s.tvdb.Configured() && !s.tmdb.Configured() {
		return nil, noKey("TheTVDB or TMDB")
	}
	if s.tvdb.Configured() {
		id := n.TVDBID
		if id == 0 {
			var err error
			if id, err = s.tvdb.SearchSeries(ctx, n.Title, n.Year); err != nil {
				return nil, steps()
			}
		}
		if id != 0 {
			if c := pickedCandidate(tr, "tvdb", id); c != nil {
				return c, steps()
			}
			if ser, _, err := s.tvdb.Series(ctx, id); err == nil {
				return &metadata.Candidate{Provider: "tvdb", ID: id, Title: ser.Title, Year: ser.Year, Poster: ser.PosterPath}, steps()
			}
		}
	}
	if s.tmdb.Configured() {
		id := n.TMDBID
		var err error
		if id == 0 && n.IMDBID != "" {
			if id, err = s.tmdb.FindTV(ctx, n.IMDBID, "imdb_id"); err != nil {
				return nil, steps()
			}
		}
		if id == 0 {
			if id, err = s.tmdb.SearchTV(ctx, n.Title, n.Year); err != nil {
				return nil, steps()
			}
		}
		if id != 0 {
			if c := pickedCandidate(tr, "tmdb", id); c != nil {
				return c, steps()
			}
			return &metadata.Candidate{Provider: "tmdb", ID: id, Title: n.Title, Year: n.Year}, steps()
		}
	}
	return nil, steps()
}

// pickedCandidate finds the search result a lookup chose, to describe a match without another request.
func pickedCandidate(tr *metadata.Trace, provider string, id int64) *metadata.Candidate {
	for _, st := range tr.Steps {
		if st.Provider != provider || st.Picked != id {
			continue
		}
		for _, r := range st.Results {
			if r.ID == id {
				c := r
				return &c
			}
		}
	}
	return nil
}
