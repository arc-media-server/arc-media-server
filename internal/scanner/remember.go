package scanner

import (
	"strconv"
	"sync"
	"time"

	"ams/internal/metadata"
)

// remembered keeps the titles a library preview identified for a while, so the first scan after
// adding the library goes straight to their details instead of searching for each again.
type remembered struct {
	mu sync.Mutex
	m  map[string]rememberedMatch
}

type rememberedMatch struct {
	c  metadata.Candidate
	at time.Time
}

// rememberFor is how long a preview's matches are kept.
const rememberFor = 2 * time.Hour

func rememberKey(kind string, n MovieName) string {
	return kind + ":" + metadata.Normalize(n.Title) + ":" + strconv.Itoa(n.Year)
}

func (r *remembered) put(kind string, n MovieName, c *metadata.Candidate) {
	if c == nil || c.ID == 0 || n.Title == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil {
		r.m = map[string]rememberedMatch{}
	}
	r.m[rememberKey(kind, n)] = rememberedMatch{c: *c, at: time.Now()}
}

// get returns a title's remembered match, or nil.
func (r *remembered) get(kind string, n MovieName) *metadata.Candidate {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.m[rememberKey(kind, n)]
	if !ok || time.Since(m.at) > rememberFor {
		return nil
	}
	return &m.c
}
