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
	match := search.NewMatcher(q)

	// Movies and shows, by title, original title or other names. Only their names are loaded
	// to score; full details just for the titles shown.
	titles, err := s.store.SearchTitleNames(ctx, acc)
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
		t     store.TitleName
		score int
	}
	var hits []scored
	for _, t := range titles {
		sc := max(search.Score(q, t.Title), search.Score(q, t.OriginalTitle)-5)
		// Other names (e.g. "Sen to Chihiro") count a little less than the title.
		for _, a := range alt[t.ID] {
			sc = max(sc, search.Score(q, a)-5)
		}
		if sc > 0 {
			hits = append(hits, scored{t, sc})
		}
	}
	slices.SortStableFunc(hits, func(a, b scored) int {
		return cmp.Or(b.score-a.score, strings.Compare(a.t.SortTitle, b.t.SortTitle))
	})
	var movieIDs, showIDs []string
	for _, h := range hits {
		if h.t.Kind == "movie" && len(movieIDs) < 60 {
			movieIDs = append(movieIDs, h.t.ID)
		} else if h.t.Kind == "series" && len(showIDs) < 60 {
			showIDs = append(showIDs, h.t.ID)
		}
	}
	found, err := s.store.ItemsByID(ctx, append(slices.Clone(movieIDs), showIDs...))
	if err != nil {
		s.fail(w, err)
		return
	}
	inOrder := func(ids []string) []store.Item {
		out := []store.Item{}
		for _, id := range ids {
			if it, ok := found[id]; ok {
				out = append(out, it)
			}
		}
		return out
	}
	res.Movies, res.Shows = toDTOs(inOrder(movieIDs)), toDTOs(inOrder(showIDs))
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

	// People in the cast and crew; the busiest first among equal matches. Names are scored from
	// the in-memory list; only the best matches have their present credits counted.
	people, names, err := s.people.get(ctx, s.store)
	if err != nil {
		s.fail(w, err)
		return
	}
	type scoredPerson struct {
		p     store.Person
		score int
	}
	var ph []scoredPerson
	for i, name := range names {
		if sc := match.Score(name); sc > 0 {
			ph = append(ph, scoredPerson{people[i], sc})
		}
	}
	// Credits in all titles stand in for present ones until those are counted.
	byBusiest := func(a, b scoredPerson) int {
		return cmp.Or(b.score-a.score, b.p.Count-a.p.Count, strings.Compare(a.p.Name, b.p.Name))
	}
	slices.SortStableFunc(ph, byBusiest)
	candidates := make([]store.Person, 0, min(len(ph), 200))
	scores := map[int64]int{}
	for _, c := range ph[:min(len(ph), 200)] {
		candidates = append(candidates, c.p)
		scores[c.p.ID] = c.score
	}
	matched, err := s.store.PeopleCredits(ctx, candidates)
	if err != nil {
		s.fail(w, err)
		return
	}
	ph = ph[:0]
	for _, p := range matched {
		ph = append(ph, scoredPerson{p, scores[p.ID]})
	}
	slices.SortStableFunc(ph, byBusiest)
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
