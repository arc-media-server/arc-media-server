package metadata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

type scanTransport func(*http.Request) (*http.Response, error)

func (f scanTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fixtureClient(t *testing.T, handler http.HandlerFunc) *http.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	base, _ := url.Parse(srv.URL)
	return &http.Client{Transport: scanTransport(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		u := *r.URL
		u.Scheme = base.Scheme
		u.Host = base.Host
		r.URL = &u
		return srv.Client().Transport.RoundTrip(r)
	})}
}

func TestMatchingConfidenceCorpus(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		candidates  []Candidate
		want        int64
	}{
		{"exact", "Alien", []Candidate{{ID: 1, Title: "Alien"}, {ID: 2, Title: "Aliens"}}, 1},
		{"article", "Matrix", []Candidate{{ID: 1, Title: "The Matrix"}}, 1},
		{"original title", "Sen to Chihiro", []Candidate{{ID: 1, Title: "Spirited Away", OriginalTitle: "Sen to Chihiro"}}, 1},
		{"unrelated top hit", "Unknown Mystery", []Candidate{{ID: 1, Title: "A Different Movie"}}, 0},
		{"ambiguous remake", "The Thing", []Candidate{{ID: 1, Title: "The Thing", Year: 1982}, {ID: 2, Title: "The Thing", Year: 2011}}, 0},
		{"close alternatives", "Example Feature", []Candidate{{ID: 1, Title: "Example Features"}, {ID: 2, Title: "Example Featur"}}, 0},
		{"long typo", "Interstelllar", []Candidate{{ID: 1, Title: "Interstellar"}}, 1},
		{"sequel number", "A Very Long Movie 2", []Candidate{{ID: 1, Title: "A Very Long Movie 3"}}, 0},
		{"duplicate provider row", "Alien", []Candidate{{ID: 1, Title: "Alien"}, {ID: 1, Title: "Alien"}}, 1},
		{"punctuation", "Spider-Man", []Candidate{{ID: 1, Title: "Spider Man"}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := confidentMatch(tc.title, tc.candidates); got != tc.want {
				t.Fatalf("match=%d want %d", got, tc.want)
			}
		})
	}
}

func TestPreviewResponseReuseAndRefresh(t *testing.T) {
	var calls atomic.Int32
	c := NewTMDB("fixture-key")
	c.http = fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"value": calls.Add(1)})
	})
	read := func(ctx context.Context) int32 {
		t.Helper()
		var out struct {
			Value int32 `json:"value"`
		}
		if err := c.get(ctx, "/movie/1", nil, &out); err != nil {
			t.Fatal(err)
		}
		return out.Value
	}
	fr := WithLanguage(context.Background(), "fr-FR")
	if read(WithPreviewCache(fr)) != 1 || read(WithPreviewReuse(fr)) != 1 {
		t.Fatal("preview details downloaded again")
	}
	if read(WithPreviewReuse(WithLanguage(context.Background(), "en-GB"))) != 2 {
		t.Fatal("language cache collision")
	}
	if read(fr) != 3 || read(WithPreviewReuse(fr)) != 4 {
		t.Fatal("explicit refresh reused or retained stale preview data")
	}
	read(WithPreviewCache(fr))
	c.SetKey("different-fixture-key")
	if read(WithPreviewReuse(fr)) != 6 {
		t.Fatal("API-key cache collision")
	}
}

func TestProviderRetriesAndCancellation(t *testing.T) {
	for _, provider := range []string{"tmdb", "tvdb"} {
		t.Run(provider, func(t *testing.T) {
			var calls atomic.Int32
			client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", "0")
					http.Error(w, "temporary", 503)
					return
				}
				w.Write([]byte(`{"value":1}`))
			})
			var get func(context.Context, string, url.Values, any) error
			if provider == "tmdb" {
				c := NewTMDB("fixture")
				c.http = client
				get = c.get
			} else {
				c := NewTVDB("fixture")
				c.http = client
				c.token = "fixture"
				c.tokenAt = time.Now()
				get = c.get
			}
			var out map[string]any
			if err := get(context.Background(), "/resource", nil, &out); err != nil || calls.Load() != 2 {
				t.Fatalf("retry: calls=%d err=%v", calls.Load(), err)
			}
			calls.Store(0)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := get(ctx, "/cancel", nil, &out); err == nil {
				t.Fatal("cancellation ignored")
			}
		})
	}
}

func TestCacheNeverRetainsErrors(t *testing.T) {
	c := NewTMDB("fixture")
	var calls atomic.Int32
	c.http = fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); http.Error(w, "missing", 404) })
	ctx := WithPreviewCache(context.Background())
	for range 2 {
		var out any
		if err := c.get(ctx, "/missing", nil, &out); err != ErrNotFound {
			t.Fatalf("err=%v", err)
		}
	}
	if calls.Load() != 2 {
		t.Fatal("failed response cached")
	}
}
