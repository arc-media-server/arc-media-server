package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"ams/internal/metadata"
	"ams/internal/scanner"
)

// matchInfo: GET /api/items/{id}/match explains how an item was identified: what was read from
// its file and folder names, and every search made with what it found.
func (s *Server) matchInfo(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	lib, err := s.store.GetLibrary(ctx, it.LibraryID)
	if err != nil {
		s.fail(w, err)
		return
	}

	parsed := []scanner.ParsedName{}
	switch it.Kind {
	case "movie", "episode":
		files, err := s.store.ItemFiles(ctx, it.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		for _, f := range files {
			if it.Kind == "movie" {
				parsed = append(parsed, scanner.ExplainMovieFile(f.Path))
			} else {
				parsed = append(parsed, scanner.ExplainEpisodeFile(lib, f.Path))
			}
		}
	case "series":
		// The series is read from its folder; one episode shows how.
		paths, err := s.store.SeriesFilePaths(ctx, it.ID, 1)
		if err != nil {
			s.fail(w, err)
			return
		}
		for _, p := range paths {
			parsed = append(parsed, scanner.ExplainEpisodeFile(lib, p))
		}
	}

	info, err := s.scanner.MatchInfo(ctx, it.ID)
	if err != nil {
		s.fail(w, err)
		return
	}
	if info == nil {
		info = &scanner.MatchInfo{Steps: []metadata.Step{}}
	}
	tmdb, tvdb := s.scanner.Providers()
	writeJSON(w, http.StatusOK, map[string]any{
		"item":            toDTO(*it),
		"parsed":          parsed,
		"checked_at":      info.CheckedAt,
		"steps":           info.Steps,
		"providers":       map[string]bool{"tmdb": tmdb, "tvdb": tvdb},
		"source_ordering": lib.SourceOrdering,
	})
}

// searchMatches: GET /api/items/{id}/match/search?provider=tmdb&q=Alien&year=1979
func (s *Server) searchMatches(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	query := strings.TrimSpace(q.Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	year, _ := strconv.Atoi(q.Get("year"))
	provider := q.Get("provider")
	if provider == "" {
		provider = "tmdb"
	}
	results, err := s.scanner.Search(r.Context(), it.Kind, provider, query, year)
	if errors.Is(err, scanner.ErrNoProvider) {
		writeError(w, http.StatusConflict, "add a "+strings.ToUpper(provider)+" API key to ams.yaml to search it")
		return
	}
	if err != nil {
		s.log.Warn("match search failed", "query", query, "err", err)
		writeError(w, http.StatusBadGateway, "search failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// setMatch: PUT /api/items/{id}/match {"provider": "tmdb", "id": 603} identifies an item as the
// chosen provider entry and fetches its metadata.
func (s *Server) setMatch(w http.ResponseWriter, r *http.Request) {
	it, ok := s.itemFromPath(w, r)
	if !ok {
		return
	}
	var body struct {
		Provider string `json:"provider"`
		ID       int64  `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ID <= 0 ||
		(body.Provider != "tmdb" && body.Provider != "tvdb") {
		writeError(w, http.StatusBadRequest, `send {"provider": "tmdb" or "tvdb", "id": <number>}`)
		return
	}
	if s.scanner.Running() && it.Kind == "series" {
		writeError(w, http.StatusConflict, "a library scan is running; try again when it finishes")
		return
	}
	err := s.scanner.SetMatch(r.Context(), it, body.Provider, body.ID)
	switch {
	case errors.Is(err, scanner.ErrNoProvider):
		writeError(w, http.StatusConflict, "add a "+strings.ToUpper(body.Provider)+" API key in Settings to use it")
		return
	case errors.Is(err, scanner.ErrLocalVideo):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, scanner.ErrNotFound):
		writeError(w, http.StatusNotFound, strings.ToUpper(body.Provider)+" has nothing with that id")
		return
	case err != nil:
		s.log.Warn("setting match failed", "item", it.Title, "err", err)
		writeError(w, http.StatusBadGateway, "couldn't fetch the new match: "+err.Error())
		return
	}
	s.getItem(w, r)
}
