package api

import (
	"cmp"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"ams/internal/artwork"
	"ams/internal/search"
	"ams/internal/store"
)

type personDTO struct {
	store.Person
	Image string `json:"image,omitempty"`
}

func toPersonDTO(p store.Person) personDTO {
	d := personDTO{Person: p}
	if p.ProfilePath != "" {
		d.Image = "/api/people/" + strconv.FormatInt(p.ID, 10) + "/image?v=" + artwork.Key(artwork.Source("person", "primary", p.ProfilePath))
	}
	return d
}

type genreResult struct {
	store.GenreCount
	LibraryName string `json:"library_name"`
}

// searchAll: GET /api/search?q=... finds movies, shows, episodes, people, collections and genres,
// best matches first. Accents, case and punctuation don't matter, and small typos are forgiven.
func (s *Server) searchAll(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("q"))
	q := search.Normalize(raw)
	type out struct {
		Movies      []itemDTO       `json:"movies"`
		Shows       []itemDTO       `json:"shows"`
		Episodes    []itemDTO       `json:"episodes"`
		People      []personDTO     `json:"people"`
		Collections []collectionDTO `json:"collections"`
		Genres      []genreResult   `json:"genres"`
	}
	res := out{Movies: []itemDTO{}, Shows: []itemDTO{}, Episodes: []itemDTO{}, People: []personDTO{}, Collections: []collectionDTO{}, Genres: []genreResult{}}
	if q == "" {
		writeJSON(w, http.StatusOK, res)
		return
	}
	ctx := r.Context()
	acc := access(r)

	// Movies and shows, by title or original title.
	titles, err := s.store.SearchTitles(ctx, acc)
	if err != nil {
		s.fail(w, err)
		return
	}
	alt, err := s.store.AltTitles(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	type scored struct {
		it    store.Item
		score int
	}
	var hits []scored
	for _, it := range titles {
		sc := max(search.Score(q, it.Title), search.Score(q, it.OriginalTitle)-5)
		// Other names (e.g. "Sen to Chihiro") count a little less than the title.
		for _, t := range alt[it.ID] {
			sc = max(sc, search.Score(q, t)-5)
		}
		if sc > 0 {
			hits = append(hits, scored{it, sc})
		}
	}
	slices.SortStableFunc(hits, func(a, b scored) int {
		return cmp.Or(b.score-a.score, strings.Compare(a.it.SortTitle, b.it.SortTitle))
	})
	var movies, shows []store.Item
	for _, h := range hits {
		if h.it.Kind == "movie" && len(movies) < 60 {
			movies = append(movies, h.it)
		} else if h.it.Kind == "series" && len(shows) < 60 {
			shows = append(shows, h.it)
		}
	}
	res.Movies, res.Shows = toDTOs(movies), toDTOs(shows)
	all := append(ptrs(res.Movies), ptrs(res.Shows)...)
	if err := s.fillUserData(r, all); err != nil {
		s.fail(w, err)
		return
	}

	// Episodes, by title (not for one or two letters, which match nearly everything).
	if len([]rune(q)) >= 3 {
		eps, err := s.store.SearchEpisodes(ctx, raw, acc, 20)
		if err != nil {
			s.fail(w, err)
			return
		}
		series := map[string]*store.Item{}
		for _, ep := range eps {
			sr, ok := series[ep.ParentID]
			if !ok {
				sr, _ = s.store.GetItem(ctx, ep.ParentID)
				series[ep.ParentID] = sr
			}
			var pos map[string][2]int
			if sr != nil {
				pos, _, _ = s.orderPositions(ctx, currentUserID(r), sr)
			}
			res.Episodes = append(res.Episodes, s.episodeDTO(ep, sr, pos))
		}
		if err := s.fillUserData(r, ptrs(res.Episodes)); err != nil {
			s.fail(w, err)
			return
		}
	}

	// People in the cast and crew; the busiest first among equal matches.
	people, err := s.store.PeopleNames(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	scores := map[int64]int{}
	var matched []store.Person
	for _, p := range people {
		if sc := search.Score(q, p.Name); sc > 0 {
			scores[p.ID] = sc
			matched = append(matched, p)
		}
	}
	if matched, err = s.store.PeopleCredits(ctx, matched); err != nil {
		s.fail(w, err)
		return
	}
	type scoredPerson struct {
		p     store.Person
		score int
	}
	var ph []scoredPerson
	for _, p := range matched {
		ph = append(ph, scoredPerson{p, scores[p.ID]})
	}
	slices.SortStableFunc(ph, func(a, b scoredPerson) int {
		return cmp.Or(b.score-a.score, b.p.Count-a.p.Count, strings.Compare(a.p.Name, b.p.Name))
	})
	for i, p := range ph {
		if i == 30 {
			break
		}
		res.People = append(res.People, toPersonDTO(p.p))
	}

	// Collections and genres, across libraries.
	libs, err := s.store.ListLibraries(ctx)
	if err != nil {
		s.fail(w, err)
		return
	}
	libNames := map[int64]string{}
	for _, lib := range libs {
		libNames[lib.ID] = lib.Name
		if !acc.LibraryAllowed(lib.ID) {
			continue
		}
		cols, err := s.store.Collections(ctx, lib.ID, acc)
		if err != nil {
			s.fail(w, err)
			return
		}
		for _, c := range cols {
			if search.Score(q, c.Title) == 0 {
				continue
			}
			first, total, err := s.collectionItems(r, &c, 1)
			if err != nil || total == 0 {
				continue
			}
			c.Count = total
			res.Collections = append(res.Collections, s.collectionDTO(c, &first[0]))
		}
	}
	genres, err := s.store.GenreCounts(ctx, acc)
	if err != nil {
		s.fail(w, err)
		return
	}
	for _, g := range genres {
		if search.Score(q, g.Name) >= 70 {
			res.Genres = append(res.Genres, genreResult{GenreCount: g, LibraryName: libNames[g.LibraryID]})
		}
	}
	slices.SortFunc(res.Genres, func(a, b genreResult) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.LibraryName, b.LibraryName))
	})
	writeJSON(w, http.StatusOK, res)
}

type personCreditDTO struct {
	Item      itemDTO  `json:"item"`
	Roles     []string `json:"roles"`
	Character string   `json:"character,omitempty"`
}

// getPerson: GET /api/people/{id} is someone and the titles in the libraries they're in.
func (s *Server) getPerson(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid person id")
		return
	}
	p, err := s.store.GetPerson(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	if p == nil {
		writeError(w, http.StatusNotFound, "person not found")
		return
	}
	credits, err := s.store.PersonCredits(r.Context(), id, access(r))
	if err != nil {
		s.fail(w, err)
		return
	}
	out := make([]personCreditDTO, 0, len(credits))
	for _, c := range credits {
		out = append(out, personCreditDTO{Item: toDTO(c.Item), Roles: c.Roles, Character: c.Character})
	}
	items := make([]*itemDTO, len(out))
	for i := range out {
		items[i] = &out[i].Item
	}
	if err := s.fillUserData(r, items); err != nil {
		s.fail(w, err)
		return
	}
	p.Count = len(out)
	writeJSON(w, http.StatusOK, map[string]any{"person": toPersonDTO(*p), "credits": out})
}
