package metadata

import (
	"context"
	"sync"
)

// A Trace records the lookups made while identifying something, so AMS can explain why it
// matched what it did (or why nothing matched). Attach one to a context with WithTrace; the
// provider clients add a Step for every search or id lookup made with that context.
type Trace struct {
	mu    sync.Mutex
	Steps []Step `json:"steps"`
}

// Step is one lookup.
type Step struct {
	Provider string `json:"provider"` // tmdb or tvdb
	// search (by title), find (by another provider's id) or lookup (by the provider's own id)
	Kind    string      `json:"kind"`
	Query   string      `json:"query"`
	Year    int         `json:"year,omitempty"`
	Results []Candidate `json:"results"`
	// The result that was chosen (0 if none), and why nothing was if so.
	Picked int64  `json:"picked,omitempty"`
	Note   string `json:"note,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Candidate is one search result.
type Candidate struct {
	Provider string `json:"provider"`
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	// Original-language title when it differs.
	OriginalTitle string `json:"original_title,omitempty"`
	Year          int    `json:"year,omitempty"`
	Overview      string `json:"overview,omitempty"`
	// Poster image URL straight from the provider (for showing search results).
	Poster string `json:"poster,omitempty"`
}

// How many results each step keeps: enough to see what the provider offered.
const traceResults = 5

type traceKey struct{}

func WithTrace(ctx context.Context, t *Trace) context.Context {
	return context.WithValue(ctx, traceKey{}, t)
}

// TraceFrom returns the context's trace, or nil.
func TraceFrom(ctx context.Context) *Trace {
	t, _ := ctx.Value(traceKey{}).(*Trace)
	return t
}

// record adds a step to the context's trace, if it has one.
func record(ctx context.Context, s Step) {
	t := TraceFrom(ctx)
	if t == nil {
		return
	}
	if len(s.Results) > traceResults {
		s.Results = s.Results[:traceResults]
	}
	for i := range s.Results {
		s.Results[i].Overview = "" // keep stored explanations small
	}
	if s.Results == nil {
		s.Results = []Candidate{}
	}
	t.mu.Lock()
	t.Steps = append(t.Steps, s)
	t.mu.Unlock()
}
