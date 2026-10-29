package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"ams/internal/activity"
	"ams/internal/metadata"
	"ams/internal/store"
)

// MatchInfo is how an item was last identified: every search and lookup, with what it found.
type MatchInfo struct {
	CheckedAt int64           `json:"checked_at"`
	Steps     []metadata.Step `json:"steps"`
}

// traced returns a context that records lookups, and a function that saves them as the item's
// match explanation. If ctx is already being traced (a caller is identifying the item as part of
// something bigger), that trace is used and the caller saves it.
func (s *Scanner) traced(ctx context.Context, itemID string) (context.Context, func()) {
	if metadata.TraceFrom(ctx) != nil {
		return ctx, func() {}
	}
	tr := &metadata.Trace{}
	return metadata.WithTrace(ctx, tr), func() { s.saveTrace(ctx, itemID, tr) }
}

// saveTrace stores the lookups made while identifying an item. Syncs that only re-fetch known
// ids make no lookups worth explaining, so they keep the earlier explanation.
func (s *Scanner) saveTrace(ctx context.Context, itemID string, tr *metadata.Trace) {
	if len(tr.Steps) == 0 {
		return
	}
	b, err := json.Marshal(MatchInfo{CheckedAt: time.Now().Unix(), Steps: tr.Steps})
	if err == nil {
		err = s.store.SetMatchInfo(context.WithoutCancel(ctx), itemID, string(b))
	}
	if err != nil {
		s.log.Warn("saving match details failed", "item", itemID, "err", err)
	}
}

// MatchInfo returns how an item was last identified, or nil if it hasn't been looked up.
func (s *Scanner) MatchInfo(ctx context.Context, itemID string) (*MatchInfo, error) {
	raw, err := s.store.MatchInfo(ctx, itemID)
	if err != nil || raw == "" {
		return nil, err
	}
	var mi MatchInfo
	if err := json.Unmarshal([]byte(raw), &mi); err != nil {
		return nil, err
	}
	return &mi, nil
}

// Images lists the posters and backgrounds TMDB has for a movie or show, to choose from.
func (s *Scanner) Images(ctx context.Context, it *store.Item) (posters, backdrops, logos []metadata.Image, err error) {
	if !s.tmdb.Configured() {
		return nil, nil, nil, ErrNoProvider
	}
	if it.TMDBID == 0 {
		return []metadata.Image{}, []metadata.Image{}, []metadata.Image{}, nil
	}
	kind := "movie"
	if it.Kind == "series" {
		kind = "tv"
	}
	return s.tmdb.Images(ctx, kind, it.TMDBID)
}

// SeasonImages lists the posters TMDB has for a show's season.
func (s *Scanner) SeasonImages(ctx context.Context, series *store.Item, season int) ([]metadata.Image, error) {
	if !s.tmdb.Configured() {
		return nil, ErrNoProvider
	}
	if series.TMDBID == 0 {
		return []metadata.Image{}, nil
	}
	return s.tmdb.SeasonImages(ctx, series.TMDBID, season)
}

// EpisodeStills lists the stills TMDB has for an episode at its TMDB aired position.
func (s *Scanner) EpisodeStills(ctx context.Context, series *store.Item, season, episode int) ([]metadata.Image, error) {
	if !s.tmdb.Configured() {
		return nil, ErrNoProvider
	}
	if series.TMDBID == 0 {
		return []metadata.Image{}, nil
	}
	return s.tmdb.EpisodeStills(ctx, series.TMDBID, season, episode)
}

// CollectionImages lists the posters and backdrops TMDB has for one of its collections.
func (s *Scanner) CollectionImages(ctx context.Context, tmdbID int64) (posters, backdrops []metadata.Image, err error) {
	if !s.tmdb.Configured() {
		return nil, nil, ErrNoProvider
	}
	return s.tmdb.CollectionImages(ctx, tmdbID)
}

// Providers reports which metadata providers are configured.
func (s *Scanner) Providers() (tmdb, tvdb bool) { return s.tmdb.Configured(), s.tvdb.Configured() }

// Search looks up candidates for someone choosing a match by hand. kind is the item kind
// (movie or series); provider is tmdb or tvdb (TV only).
func (s *Scanner) Search(ctx context.Context, kind, provider, query string, year int) ([]metadata.Candidate, error) {
	switch {
	case kind == "movie" && provider == "tmdb" && s.tmdb.Configured():
		return s.tmdb.SearchMovies(ctx, query, year)
	case kind == "series" && provider == "tmdb" && s.tmdb.Configured():
		return s.tmdb.SearchTVShows(ctx, query, year)
	case kind == "series" && provider == "tvdb" && s.tvdb.Configured():
		return s.tvdb.SearchShows(ctx, query, year)
	case kind != "movie" && kind != "series":
		return nil, fmt.Errorf("only movies and shows can be matched")
	}
	return nil, ErrNoProvider
}

// ErrNotFound means a chosen match doesn't exist at the provider.
var ErrNotFound = errors.New("not found")

// SetMatch identifies an item as a specific provider entry chosen by the user, as Plex's Fix
// Match does: the choice is saved straight away and the slow part happens in the background.
//
// A movie is one lookup, so its details are fetched before SetMatch returns, and taskID is 0.
// For a show only the chosen entry's own record (title, year, overview, artwork and the other
// provider's id) is fetched before it returns, so the page can show the right show at once. Its
// seasons, episodes, episode orders, cast and season artwork are then fetched by a background
// task, listed in Activity as "Matching <title>", which relinks the show's files to the new
// episodes under the library's work slot (waiting for a scan or refresh that holds it). taskID
// is that task's Activity id. A newer match of the same show replaces one still waiting.
//
// If the chosen entry can't be fetched, nothing changes and the error is returned. If the
// background part fails, the new match is kept (the entry exists), the error shows in Activity
// and in the show's match explanation, and Refresh metadata or the next scheduled refresh
// fetches the rest.
func (s *Scanner) SetMatch(ctx context.Context, it *store.Item, provider string, id int64) (taskID int64, err error) {
	tr := &metadata.Trace{Steps: []metadata.Step{{Provider: provider, Kind: "chosen", Picked: id,
		Results: []metadata.Candidate{}, Note: "Chosen by you."}}}
	if it.MetadataState == "locked" {
		return 0, ErrLocalVideo
	}
	ctx = metadata.WithTrace(s.inLanguage(ctx, it.LibraryID), tr)
	switch it.Kind {
	case "movie":
		return 0, s.setMovieMatch(ctx, it, provider, id, tr)
	case "series":
		return s.setSeriesMatch(ctx, it, provider, id, tr)
	}
	return 0, fmt.Errorf("only movies and shows can be matched")
}

// setMovieMatch fetches the chosen movie's details now. If it fails, the item keeps its
// previous match.
func (s *Scanner) setMovieMatch(ctx context.Context, it *store.Item, provider string, id int64, tr *metadata.Trace) (err error) {
	if provider != "tmdb" || !s.tmdb.Configured() {
		return ErrNoProvider
	}
	defer func() {
		if err == nil {
			s.saveTrace(ctx, it.ID, tr)
			return
		}
		if errors.Is(err, metadata.ErrNotFound) {
			err = ErrNotFound
		}
		s.restoreMatch(ctx, it)
	}()
	if err := s.store.SetProviderIDs(ctx, it.ID, id, 0, ""); err != nil {
		return err
	}
	found, err := s.RefreshMovie(ctx, it.ID)
	if err == nil && !found {
		err = ErrNotFound
	}
	return err
}

// restoreMatch puts back the provider ids and state an item had before a failed match.
func (s *Scanner) restoreMatch(ctx context.Context, it *store.Item) {
	ctx = context.WithoutCancel(ctx)
	if err := s.store.SetProviderIDs(ctx, it.ID, it.TMDBID, it.TVDBID, it.IMDBID); err != nil {
		s.log.Warn("restoring previous match failed", "item", it.Title, "err", err)
	}
	s.store.SetMetadataState(ctx, it.ID, it.MetadataState)
}

// setSeriesMatch is the quick part of matching a show: it checks the chosen entry exists, saves
// it with the show's own details, and starts the background task that fetches the rest.
func (s *Scanner) setSeriesMatch(ctx context.Context, it *store.Item, provider string, id int64, tr *metadata.Trace) (int64, error) {
	var head *metadata.Series
	var err error
	switch {
	case provider == "tmdb" && s.tmdb.Configured():
		head, err = s.tmdb.TVSummary(ctx, id)
	case provider == "tvdb" && s.tvdb.Configured():
		head, _, err = s.tvdb.Series(ctx, id)
	default:
		return 0, ErrNoProvider
	}
	if errors.Is(err, metadata.ErrNotFound) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	lib, err := s.store.GetLibrary(ctx, it.LibraryID)
	if err != nil || lib == nil {
		return 0, fmt.Errorf("library %d: %w", it.LibraryID, err)
	}
	// The chosen id, and the other provider's id when the chosen record names it (so the
	// background part follows that link rather than searching by title).
	tmdbID, tvdbID := head.TMDBID, head.TVDBID
	if provider == "tmdb" {
		tmdbID = id
		if !s.tvdb.Configured() {
			tvdbID = 0
		}
	} else {
		tvdbID = id
		if !s.tmdb.Configured() {
			tmdbID = 0
		}
	}

	s.mu.Lock()
	if s.matching == nil {
		s.matching = map[string]int64{}
	}
	s.matchGen++
	gen := s.matchGen
	s.matching[it.ID] = gen
	s.mu.Unlock()

	// The show's own details, so the page shows the new show at once; the background part
	// replaces them with the library's details provider's. Details of the old match (tagline,
	// rating, studio and so on) are cleared rather than shown against the new one.
	if err := s.store.SetProviderIDs(ctx, it.ID, tmdbID, tvdbID, head.IMDBID); err != nil {
		s.endMatch(it.ID, gen)
		return 0, err
	}
	quick := store.Item{ID: it.ID, Title: head.Title, OriginalTitle: head.OriginalTitle, Overview: head.Overview,
		ReleaseDate: head.FirstAired, Year: head.Year, PosterPath: head.PosterPath, BackdropPath: head.BackdropPath,
		TMDBID: tmdbID, TVDBID: tvdbID, IMDBID: head.IMDBID, MetadataState: "matched"}
	if err := s.store.UpdateMetadata(ctx, &quick); err != nil {
		s.endMatch(it.ID, gen)
		s.restoreMatch(ctx, it)
		return 0, err
	}
	s.saveTrace(ctx, it.ID, tr)

	task := s.Activity.Start("refresh", "Matching "+activityTitle(head.Title, head.Year))
	task.Detail("Fetching seasons and episodes")
	s.wg.Add(1)
	go s.finishSeriesMatch(lib, it.ID, gen, provider, tmdbID, tvdbID, head.IMDBID, tr, task)
	return task.ID(), nil
}

// errReplaced ends a background match that a newer match of the same show replaced.
var errReplaced = errors.New("replaced by a newer match")

// finishSeriesMatch is the background part of matching a show: a series refresh (all seasons,
// episodes and orders, then relinking its files), waiting while the library is busy.
func (s *Scanner) finishSeriesMatch(lib *store.Library, itemID string, gen int64, provider string, tmdbID, tvdbID int64,
	imdbID string, tr *metadata.Trace, task *activity.Handle) {
	defer s.wg.Done()
	defer s.endMatch(itemID, gen)
	ctx, cancel := context.WithCancel(s.ctx)
	defer cancel()
	task.Cancellable(cancel)
	ctx = metadata.WithTrace(ctx, tr)
	// A scan that read the show before the match was saved could have written the old ids back;
	// put the chosen ones back first, inside the library's work slot.
	before := func(ctx context.Context) error {
		if !s.latestMatch(itemID, gen) {
			return errReplaced
		}
		cur, err := s.store.GetItem(ctx, itemID)
		if err != nil || cur == nil {
			return fmt.Errorf("show %s: %w", itemID, err)
		}
		if cur.TMDBID == tmdbID && cur.TVDBID == tvdbID {
			return nil
		}
		return s.store.SetProviderIDs(ctx, itemID, tmdbID, tvdbID, imdbID)
	}
	var err error
	waiting := false
	for {
		if !s.latestMatch(itemID, gen) {
			task.Discard()
			return
		}
		err = s.refreshSeries(ctx, lib, itemID, false, before, task)
		if !errors.Is(err, ErrLibraryBusy) {
			break
		}
		if !waiting {
			waiting = true
			task.Detail("Waiting for the library's current work to finish")
		}
		t := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Stop()
			task.Finish(ctx.Err())
			return
		case <-t.C:
		}
	}
	switch {
	case err == nil:
		s.saveTrace(ctx, itemID, tr)
	case errors.Is(err, errReplaced) || errors.Is(err, context.Canceled):
	default:
		s.log.Warn("fetching the new match's details failed", "item", itemID, "provider", provider, "err", err)
		chosen := tvdbID
		if provider == "tmdb" {
			chosen = tmdbID
		}
		tr.Steps = append(tr.Steps, metadata.Step{Provider: provider, Kind: "lookup", Query: strconv.FormatInt(chosen, 10),
			Results: []metadata.Candidate{},
			Error:   "Couldn't fetch its seasons and episodes: " + err.Error() + ". Refresh metadata to try again."})
		s.saveTrace(ctx, itemID, tr)
	}
}

// latestMatch reports whether gen is still the newest match of the show.
func (s *Scanner) latestMatch(itemID string, gen int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.matching[itemID] == gen
}

// endMatch forgets a match once its work is over, unless a newer one has taken its place.
func (s *Scanner) endMatch(itemID string, gen int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.matching[itemID] == gen {
		delete(s.matching, itemID)
	}
}

// Matching reports whether a show's Fix Match is still being fetched in the background.
func (s *Scanner) Matching(itemID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.matching[itemID] != 0
}

// ErrLocalVideo means a title is in an Other videos library, which isn't looked up online.
var ErrLocalVideo = errors.New("videos in an Other videos library aren't identified online")
