package api

import (
	"cmp"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"ams/internal/artwork"
	"ams/internal/metadata"
	"ams/internal/scanner"
	"ams/internal/store"
)

// seasonPosterURL is the link to a season's poster, or "" if it has none.
func seasonPosterURL(seriesID string, season int, art store.SeasonArt) string {
	base := "/api/series/" + seriesID + "/seasons/" + strconv.Itoa(season) + "/image"
	switch {
	case strings.HasPrefix(art.Chosen, metadata.UploadPrefix):
		return base + "?v=" + artwork.Key(art.Chosen)
	case art.Chosen != "":
		return base + "?v=" + artwork.Key(artwork.Source("season", "primary", art.Chosen))
	case art.Local != "":
		return base + "?local=" + artwork.Key(art.Local)
	case art.Path != "":
		return base + "?v=" + artwork.Key(artwork.Source("season", "primary", art.Path))
	}
	return ""
}

// seasonImage: GET /api/series/{id}/seasons/{n}/image is a season's poster: the file in the show's
// folder if there is one, otherwise the provider's.
func (s *Server) seasonImage(w http.ResponseWriter, r *http.Request) {
	series, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid season number")
		return
	}
	posters, err := s.store.SeasonPosters(r.Context(), series.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	art := posters[n]
	if name, ok := strings.CutPrefix(art.Chosen, metadata.UploadPrefix); ok {
		art.Local = filepath.Join(s.uploadsDir, filepath.Base(name))
	} else if art.Chosen != "" {
		art.Local, art.Path = "", art.Chosen
	}
	if art.Local != "" {
		if f, err := os.Open(art.Local); err == nil {
			defer f.Close()
			if info, err := f.Stat(); err == nil {
				w.Header().Set("Cache-Control", "no-cache")
				http.ServeContent(w, r, filepath.Base(art.Local), info.ModTime(), f)
				return
			}
		}
	}
	if art.Path == "" {
		writeError(w, http.StatusNotFound, "no image")
		return
	}
	s.serveArtwork(w, r, artwork.Source("season", "primary", art.Path))
}

// Episodes added within this long of the newest in a group go into the same card.
const recentGroupWindow = 7 * 24 * 3600

// seasonPostersFor returns a show's season posters when the profile shows it in an aired order,
// whose seasons are the ones the posters are for; none for other orders (an anime's "cours"
// season 3 isn't the provider's season 3).
func (s *Server) seasonPostersFor(r *http.Request, series *store.Item) (map[int]store.SeasonArt, error) {
	ctx := r.Context()
	lib, err := s.store.GetLibrary(ctx, series.LibraryID)
	if err != nil || lib == nil {
		return nil, err
	}
	oid, err := s.store.DisplayOrdering(ctx, series.ID, cmp.Or(series.SourceOrdering, lib.SourceOrdering))
	if err != nil || oid == 0 {
		return nil, err
	}
	o, err := s.store.GetOrdering(ctx, oid)
	if err != nil || o == nil || (o.Source != "tmdb_aired" && o.Source != "tvdb_aired") {
		return nil, err
	}
	return s.store.SeasonPosters(ctx, series.ID)
}

// seasonPosterCache remembers shows' season posters while a list of episodes is built.
type seasonPosterCache map[string]map[int]store.SeasonArt

// withSeasonPoster adds an episode's season poster, when its season has one, as
// images["season_primary"].
func (s *Server) withSeasonPoster(r *http.Request, d *itemDTO, series *store.Item, cache seasonPosterCache) error {
	if series == nil || d.SeasonNumber == nil {
		return nil
	}
	art, ok := cache[series.ID]
	if !ok {
		var err error
		if art, err = s.seasonPostersFor(r, series); err != nil {
			return err
		}
		cache[series.ID] = art
	}
	if u := seasonPosterURL(series.ID, *d.SeasonNumber, art[*d.SeasonNumber]); u != "" {
		d.Images["season_primary"] = u
	}
	return nil
}

// recentCard is one card in a TV library's Recently Added: a new show, several new episodes of
// one season, or a single new episode.
type recentCard struct {
	Type string `json:"type"` // series, season or episode
	// The show (for series and season cards) or the episode.
	Item         itemDTO `json:"item"`
	SeasonNumber *int    `json:"season_number,omitempty"`
	// How many new episodes the card stands for.
	Episodes int `json:"episodes"`
	// The season's poster, when it has one.
	SeasonPoster string `json:"season_poster,omitempty"`
	AddedAt      int64  `json:"added_at"`
}

// recentlyAddedShows: GET /api/libraries/{id}/recent-shows?limit=24 is a TV library's newest
// additions, grouped by show: a show that's new appears once; several new episodes of a season
// appear as that season; a single new episode appears as itself.
func (s *Server) recentlyAddedShows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	lib, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid library id")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 24
	}
	eps, err := s.store.RecentEpisodes(ctx, lib, access(r), limit)
	if err != nil {
		s.fail(w, err)
		return
	}

	type group struct {
		series    *store.Item
		season    *int
		newShow   bool // the whole show is new
		episodes  []store.Item
		positions map[string][2]int
	}
	var groups []*group
	byKey := map[string]*group{}
	contexts := s.episodeContexts(ctx)
	for _, ep := range eps {
		context, err := contexts.get(ep.ParentID)
		if err != nil {
			s.fail(w, err)
			return
		}
		sr := context.series
		if sr == nil {
			continue
		}
		// Episodes are grouped by season in the profile's episode order, or all together when the
		// show is new as a whole; ones that aren't in the order (files the provider doesn't know)
		// are grouped by show.
		key := ep.ParentID + ":-"
		var season *int
		newShow := sr.AddedAt >= ep.AddedAt-24*3600
		if p, ok := context.positions[ep.ID]; ok && !newShow {
			n := p[0]
			season = &n
			key = ep.ParentID + ":" + strconv.Itoa(n)
		}
		if newShow {
			key = ep.ParentID + ":new"
		}
		// Only episodes added around the same time go together: a new episode of a season added
		// long ago is a card of its own.
		g := byKey[key]
		if g == nil || ep.AddedAt < g.episodes[0].AddedAt-recentGroupWindow {
			if len(groups) == limit {
				continue
			}
			g = &group{series: sr, season: season, newShow: newShow, positions: context.positions}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.episodes = append(g.episodes, ep)
	}

	posters := seasonPosterCache{}
	out := make([]recentCard, 0, len(groups))
	for _, g := range groups {
		first := g.episodes[0]
		c := recentCard{Episodes: len(g.episodes), AddedAt: first.AddedAt}
		switch {
		case g.newShow || g.season == nil && len(g.episodes) > 1:
			c.Type, c.Item = "series", toDTO(*g.series)
		case len(g.episodes) > 1:
			c.Type, c.Item, c.SeasonNumber = "season", toDTO(*g.series), g.season
			art, ok := posters[g.series.ID]
			if !ok {
				if art, err = s.seasonPostersFor(r, g.series); err != nil {
					s.fail(w, err)
					return
				}
				posters[g.series.ID] = art
			}
			c.SeasonPoster = seasonPosterURL(g.series.ID, *g.season, art[*g.season])
		default:
			c.Type, c.Item = "episode", s.episodeDTO(first, g.series, g.positions)
			if err := s.withSeasonPoster(r, &c.Item, g.series, posters); err != nil {
				s.fail(w, err)
				return
			}
		}
		out = append(out, c)
	}
	items := make([]*itemDTO, len(out))
	for i := range out {
		items[i] = &out[i].Item
	}
	if err := s.fillUserData(r, items); err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

type seasonArtDTO struct {
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	// Chosen by hand (rather than automatic), and whether there's a poster file in the folder.
	Chosen bool `json:"chosen"`
	Local  bool `json:"local"`
}

// seasonArtList: GET /api/series/{id}/season-art lists a show's seasons (in aired order, which
// season posters belong to) with the poster each uses.
func (s *Server) seasonArtList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	series, _, ok := s.seriesFromPath(w, r)
	if !ok {
		return
	}
	orderings, err := s.store.ListOrderings(ctx, series.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	var oid int64
	for _, src := range []string{"tmdb_aired", "tvdb_aired"} {
		for _, o := range orderings {
			if o.Source == src && oid == 0 {
				oid = o.ID
			}
		}
	}
	names := map[int]string{}
	if oid != 0 {
		if names, err = s.store.OrderingSeasons(ctx, oid); err != nil {
			s.fail(w, err)
			return
		}
	}
	art, err := s.store.SeasonPosters(ctx, series.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	for n := range art {
		if _, ok := names[n]; !ok {
			names[n] = ""
		}
	}
	out := []seasonArtDTO{}
	for n, name := range names {
		if name == "" {
			name = "Season " + strconv.Itoa(n)
			if n == 0 {
				name = "Specials"
			}
		}
		a := art[n]
		out = append(out, seasonArtDTO{SeasonNumber: n, Name: name, Image: seasonPosterURL(series.ID, n, a), Chosen: a.Chosen != "", Local: a.Local != ""})
	}
	// Specials last, as on the show's page.
	slices.SortFunc(out, func(a, b seasonArtDTO) int {
		return cmp.Compare(seasonSortKey(a.SeasonNumber), seasonSortKey(b.SeasonNumber))
	})
	writeJSON(w, http.StatusOK, map[string]any{"seasons": out})
}

func seasonSortKey(n int) int {
	if n == 0 {
		return math.MaxInt
	}
	return n
}

// seasonFromPath reads a show and a season number from the request.
func (s *Server) seasonFromPath(w http.ResponseWriter, r *http.Request) (*store.Item, int, bool) {
	series, _, ok := s.seriesFromPath(w, r)
	if !ok {
		return nil, 0, false
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 0 {
		writeError(w, http.StatusBadRequest, "invalid season number")
		return nil, 0, false
	}
	return series, n, true
}

// seasonArtwork: GET /api/series/{id}/seasons/{n}/artwork lists the posters TMDB has for a season.
func (s *Server) seasonArtwork(w http.ResponseWriter, r *http.Request) {
	series, n, ok := s.seasonFromPath(w, r)
	if !ok {
		return
	}
	posters, err := s.scanner.SeasonImages(r.Context(), series, n)
	if errors.Is(err, scanner.ErrNoProvider) {
		posters = []metadata.Image{}
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "couldn't get artwork from TMDB: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"posters": posters})
}

// chooseSeasonArtwork: PUT /api/series/{id}/seasons/{n}/artwork {"path": "/abc.jpg"} uses a TMDB
// poster for a season; a null path goes back to automatic.
func (s *Server) chooseSeasonArtwork(w http.ResponseWriter, r *http.Request) {
	series, n, ok := s.seasonFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Path *string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path != nil && !strings.HasPrefix(*body.Path, "/") {
		writeError(w, http.StatusBadRequest, `send {"path": "/abc.jpg"} or {"path": null}`)
		return
	}
	path := ""
	if body.Path != nil {
		path = *body.Path
	}
	s.setSeasonChosen(w, r, series, n, path)
}

// uploadSeasonArtwork: POST /api/series/{id}/seasons/{n}/artwork with an image as the body uses it
// as the season's poster.
func (s *Server) uploadSeasonArtwork(w http.ResponseWriter, r *http.Request) {
	series, n, ok := s.seasonFromPath(w, r)
	if !ok {
		return
	}
	path, ok := s.saveUpload(w, r, series.ID+"-season"+strconv.Itoa(n))
	if !ok {
		return
	}
	s.setSeasonChosen(w, r, series, n, path)
}

func (s *Server) setSeasonChosen(w http.ResponseWriter, r *http.Request, series *store.Item, n int, path string) {
	old, err := s.store.SetSeasonChosen(r.Context(), series.ID, n, path)
	if err != nil {
		s.removeUpload(path)
		s.fail(w, err)
		return
	}
	if old != path {
		s.removeUpload(old)
	}
	s.seasonArtList(w, r)
}
