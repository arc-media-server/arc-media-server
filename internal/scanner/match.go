package scanner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
func (s *Scanner) Images(ctx context.Context, it *store.Item) (posters, backdrops []metadata.Image, err error) {
	if !s.tmdb.Configured() {
		return nil, nil, ErrNoProvider
	}
	if it.TMDBID == 0 {
		return []metadata.Image{}, []metadata.Image{}, nil
	}
	kind := "movie"
	if it.Kind == "series" {
		kind = "tv"
	}
	return s.tmdb.Images(ctx, kind, it.TMDBID)
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

// SetMatch identifies an item as a specific provider entry chosen by the user, then fetches its
// metadata. For a series the other provider's id is worked out again from the chosen one, and the
// library is rescanned so files are linked to the new episodes. If it fails, the item keeps its
// previous match.
func (s *Scanner) SetMatch(ctx context.Context, it *store.Item, provider string, id int64) (err error) {
	tr := &metadata.Trace{Steps: []metadata.Step{{Provider: provider, Kind: "chosen", Picked: id,
		Results: []metadata.Candidate{}, Note: "Chosen by you."}}}
	ctx = metadata.WithTrace(ctx, tr)
	defer func() {
		if err == nil {
			s.saveTrace(ctx, it.ID, tr)
			return
		}
		if errors.Is(err, metadata.ErrNotFound) {
			err = ErrNotFound
		}
		ctx := context.WithoutCancel(ctx)
		if rerr := s.store.SetProviderIDs(ctx, it.ID, it.TMDBID, it.TVDBID, it.IMDBID); rerr != nil {
			s.log.Warn("restoring previous match failed", "item", it.Title, "err", rerr)
		}
		s.store.SetMetadataState(ctx, it.ID, it.MetadataState)
	}()

	switch it.Kind {
	case "movie":
		if provider != "tmdb" || !s.tmdb.Configured() {
			return ErrNoProvider
		}
		if err := s.store.SetProviderIDs(ctx, it.ID, id, 0, ""); err != nil {
			return err
		}
		found, err := s.RefreshMovie(ctx, it.ID)
		if err == nil && !found {
			err = ErrNotFound
		}
		return err
	case "series":
		var tmdbID, tvdbID int64
		switch {
		case provider == "tmdb" && s.tmdb.Configured():
			tmdbID = id
		case provider == "tvdb" && s.tvdb.Configured():
			tvdbID = id
		default:
			return ErrNoProvider
		}
		if err := s.store.SetProviderIDs(ctx, it.ID, tmdbID, tvdbID, ""); err != nil {
			return err
		}
		lib, err := s.store.GetLibrary(ctx, it.LibraryID)
		if err != nil || lib == nil {
			return fmt.Errorf("library %d: %w", it.LibraryID, err)
		}
		if err := s.SyncSeriesMetadata(ctx, lib, it.ID, false); err != nil {
			return err
		}
		if updated, err := s.store.GetItem(ctx, it.ID); err == nil && updated.MetadataState == "unmatched" {
			return ErrNotFound
		}
		s.Start(lib.ID)
		return nil
	}
	return fmt.Errorf("only movies and shows can be matched")
}
