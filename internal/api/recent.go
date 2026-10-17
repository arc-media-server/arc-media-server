package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"ams/internal/artwork"
	"ams/internal/store"
)

// seasonPosterURL is the link to a season's poster, or "" if it has none.
func seasonPosterURL(seriesID string, season int, art store.SeasonArt) string {
	base := "/api/series/" + seriesID + "/seasons/" + strconv.Itoa(season) + "/image"
	switch {
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
	oid, err := s.store.DisplayOrdering(ctx, series.ID, currentUserID(r), lib.SourceOrdering)
	if err != nil || oid == 0 {
		return nil, err
	}
	o, err := s.store.GetOrdering(ctx, oid)
	if err != nil || o == nil || (o.Source != "tmdb_aired" && o.Source != "tvdb_aired") {
		return nil, err
	}
	return s.store.SeasonPosters(ctx, series.ID)
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
// additions as Plex shows them: a show that's new appears once; several new episodes of a season
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
	eps, err := s.store.RecentEpisodes(ctx, lib, maxAge(r), 400)
	if err != nil {
		s.fail(w, err)
		return
	}

	type group struct {
		series   *store.Item
		season   *int
		newShow  bool // the whole show is new
		episodes []store.Item
	}
	var groups []*group
	byKey := map[string]*group{}
	series := map[string]*store.Item{}
	positions := map[string]map[string][2]int{}
	userID := currentUserID(r)
	for _, ep := range eps {
		sr, ok := series[ep.ParentID]
		if !ok {
			if sr, err = s.store.GetItem(ctx, ep.ParentID); err != nil {
				s.fail(w, err)
				return
			}
			series[ep.ParentID] = sr
			pos := map[string][2]int{}
			if sr != nil {
				if pos, _, err = s.orderPositions(ctx, userID, sr); err != nil {
					s.fail(w, err)
					return
				}
			}
			positions[ep.ParentID] = pos
		}
		if sr == nil {
			continue
		}
		// Episodes are grouped by season in the profile's episode order, or all together when the
		// show is new as a whole; ones that aren't in the order (files the provider doesn't know)
		// are grouped by show.
		key := ep.ParentID + ":-"
		var season *int
		newShow := sr.AddedAt >= ep.AddedAt-24*3600
		if p, ok := positions[ep.ParentID][ep.ID]; ok && !newShow {
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
			g = &group{series: sr, season: season, newShow: newShow}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.episodes = append(g.episodes, ep)
	}

	posters := map[string]map[int]store.SeasonArt{}
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
			c.Type, c.Item = "episode", s.episodeDTO(first, g.series, positions[g.series.ID])
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
