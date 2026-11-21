package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ams/internal/store"
)

const maxCollectionLookups = 256

type collectionLookupAttempt struct {
	running bool
	until   time.Time
}

// Bounded recent-result hints, not metadata storage. Network/DB work never holds the lock.
type collectionLookupCache struct {
	mu       sync.Mutex
	attempts map[string]*collectionLookupAttempt
	now      func() time.Time // configured before use by tests
}

func (c *collectionLookupCache) time() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

func (c *collectionLookupCache) begin(key string) *collectionLookupAttempt {
	now := c.time()
	c.mu.Lock()
	defer c.mu.Unlock()
	if previous := c.attempts[key]; previous != nil && (previous.running || now.Before(previous.until)) {
		return nil
	}
	if c.attempts == nil {
		c.attempts = make(map[string]*collectionLookupAttempt)
	}
	if len(c.attempts) >= maxCollectionLookups && c.attempts[key] == nil {
		// Evict the oldest completed hint; never replace an active attempt.
		oldest := ""
		var earliest time.Time
		for k, a := range c.attempts {
			if !a.running && (oldest == "" || a.until.Before(earliest)) {
				oldest, earliest = k, a.until
			}
		}
		if oldest == "" {
			return nil
		}
		delete(c.attempts, oldest)
	}
	attempt := &collectionLookupAttempt{running: true}
	c.attempts[key] = attempt
	return attempt
}

func (c *collectionLookupCache) finish(key string, attempt *collectionLookupAttempt, success, cancelled bool) {
	now := c.time()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attempts[key] != attempt {
		return
	}
	if cancelled {
		delete(c.attempts, key)
		return
	}
	delay := time.Minute
	if success {
		delay = 24 * time.Hour
	}
	attempt.running = false
	attempt.until = now.Add(delay)
}

func (s *Server) fillCollectionInfo(ctx context.Context, c *store.Collection) {
	if s.scanner == nil || ctx.Err() != nil {
		return
	}
	key := fmt.Sprintf("%s:%d", c.ID, c.TMDBID)
	attempt := s.collectionLookups.begin(key)
	if attempt == nil {
		return
	}
	success := false
	defer func() { s.collectionLookups.finish(key, attempt, success, ctx.Err() != nil) }()
	lookupCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	info, err := s.scanner.CollectionInfo(lookupCtx, c.TMDBID)
	if err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "collection metadata lookup failed", "provider", "tmdb", "collection", c.ID)
		}
		return
	}
	if info == nil {
		return
	} // not configured: short retry after settings change
	if err = s.store.SetCollectionInfo(lookupCtx, c.ID, info.Overview, info.PosterPath, info.BackdropPath); err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "collection metadata could not be saved", "collection", c.ID)
		}
		return
	}
	success = true // an empty upstream overview is a successful, useful result too
	c.Overview = info.Overview
	if info.PosterPath != "" {
		c.PosterPath = info.PosterPath
	}
	if info.BackdropPath != "" {
		c.BackdropPath = info.BackdropPath
	}
}

// fillSharedArtwork gives a manual collection (made by hand, or imported from Plex or Jellyfin)
// without a poster or background the artwork of the TMDB collection its titles share, as an
// automatic collection has. Artwork already there or chosen by hand is kept.
func (s *Server) fillSharedArtwork(ctx context.Context, c *store.Collection) {
	if s.scanner == nil || ctx.Err() != nil || c.Kind != "manual" || (c.PosterPath != "" && c.BackdropPath != "") {
		return
	}
	shared, err := s.store.SharedTMDBCollection(ctx, c)
	if err != nil || shared == 0 {
		return
	}
	key := fmt.Sprintf("%s:%d", c.ID, shared)
	attempt := s.collectionLookups.begin(key)
	if attempt == nil {
		return
	}
	success := false
	defer func() { s.collectionLookups.finish(key, attempt, success, ctx.Err() != nil) }()
	lookupCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	info, err := s.scanner.CollectionInfo(lookupCtx, shared)
	if err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "collection artwork lookup failed", "provider", "tmdb", "collection", c.ID)
		}
		return
	}
	if info == nil {
		return
	}
	if err = s.store.FillCollectionArtwork(lookupCtx, c.ID, info.PosterPath, info.BackdropPath); err != nil {
		if ctx.Err() == nil {
			s.log.WarnContext(ctx, "collection artwork could not be saved", "collection", c.ID)
		}
		return
	}
	success = true
	if c.PosterPath == "" {
		c.PosterPath = info.PosterPath
	}
	if c.BackdropPath == "" {
		c.BackdropPath = info.BackdropPath
	}
}

// fillSharedArtworkLater does fillSharedArtwork for a library's listed collections in the
// background, so the listing isn't held up by TMDB; the next listing shows the artwork.
func (s *Server) fillSharedArtworkLater(ctx context.Context, cols []store.Collection) {
	var todo []store.Collection
	for _, c := range cols {
		if c.Kind == "manual" && (c.PosterPath == "" || c.BackdropPath == "") {
			todo = append(todo, c)
		}
	}
	if len(todo) == 0 || s.scanner == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Minute)
		defer cancel()
		started, filled := time.Now(), 0
		for i := range todo {
			before := todo[i].PosterPath + "|" + todo[i].BackdropPath
			s.fillSharedArtwork(ctx, &todo[i])
			if todo[i].PosterPath+"|"+todo[i].BackdropPath != before {
				filled++
			}
		}
		if filled > 0 {
			s.log.InfoContext(ctx, "collection artwork filled from TMDB", "collections", filled, "checked", len(todo), "duration_ms", time.Since(started).Milliseconds())
		}
	}()
}
