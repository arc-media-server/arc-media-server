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
		year        int
		candidates  []Candidate
		want        int64
	}{
		{"exact", "Alien", 0, []Candidate{{ID: 1, Title: "Alien"}, {ID: 2, Title: "Aliens"}}, 1},
		{"article", "Matrix", 0, []Candidate{{ID: 1, Title: "The Matrix"}}, 1},
		{"original title", "Sen to Chihiro", 0, []Candidate{{ID: 1, Title: "Spirited Away", OriginalTitle: "Sen to Chihiro"}}, 1},
		{"unrelated top hit", "Unknown Mystery", 0, []Candidate{{ID: 1, Title: "A Different Movie"}}, 0},
		{"ambiguous remake", "The Thing", 0, []Candidate{{ID: 1, Title: "The Thing", Year: 1982}, {ID: 2, Title: "The Thing", Year: 2011}}, 0},
		{"close alternatives", "Example Feature", 0, []Candidate{{ID: 1, Title: "Example Features"}, {ID: 2, Title: "Example Featur"}}, 0},
		{"long typo", "Interstelllar", 0, []Candidate{{ID: 1, Title: "Interstellar"}}, 1},
		{"sequel number", "A Very Long Movie 2", 0, []Candidate{{ID: 1, Title: "A Very Long Movie 3"}}, 0},
		{"duplicate provider row", "Alien", 0, []Candidate{{ID: 1, Title: "Alien"}, {ID: 1, Title: "Alien"}}, 1},
		{"punctuation", "Spider-Man", 0, []Candidate{{ID: 1, Title: "Spider Man"}}, 1},
		// TMDB lists an obscure "The Alien" (1979) beside Alien (1979): the title word for word wins.
		{"article tie", "Alien", 0, []Candidate{{ID: 1, Title: "Alien"}, {ID: 2, Title: "Alien Portrait"}, {ID: 3, Title: "The Alien"}}, 1},
		{"article tie, the other way", "The Alien", 0, []Candidate{{ID: 1, Title: "Alien"}, {ID: 3, Title: "The Alien"}}, 3},
		{"article tie, neither literal", "Matrix", 0, []Candidate{{ID: 1, Title: "The Matrix"}, {ID: 2, Title: "A Matrix"}}, 0},
		// From a real run: TMDB's year search also returns same-named films from other years,
		// and same-named obscure films from the same year.
		{"remake from another year", "Dune", 2021, []Candidate{{ID: 1, Title: "Dune", Year: 2021, Votes: 13000}, {ID: 2, Title: "Dune", Year: 1984, Votes: 3000}, {ID: 3, Title: "Dune", Year: 2020, Votes: 1}}, 1},
		{"original from decades before", "Halloween", 2018, []Candidate{{ID: 1, Title: "Halloween", Year: 1978, Votes: 6000}, {ID: 2, Title: "Halloween", Year: 2018, Votes: 4800}}, 2},
		{"obscure film of the same name and year", "Rocky", 1976, []Candidate{{ID: 1, Title: "Rocky", Year: 1976, Votes: 8000}, {ID: 2, Title: "Rocky", Year: 1976, Votes: 2}}, 1},
		{"festival year differs by one", "Crash", 2004, []Candidate{{ID: 1, Title: "Crash", Year: 2005, Votes: 4000}, {ID: 2, Title: "Crash", Year: 2004, Votes: 3}, {ID: 3, Title: "Crash", Year: 2004}}, 1},
		{"two well-known films of one name and year", "Twins", 2000, []Candidate{{ID: 1, Title: "Twins", Year: 2000, Votes: 900}, {ID: 2, Title: "Twins", Year: 2000, Votes: 400}}, 0},
		{"remake without a year stays for Fix Match", "The Thing", 0, []Candidate{{ID: 1, Title: "The Thing", Year: 1982, Votes: 7000}, {ID: 2, Title: "The Thing", Year: 2011, Votes: 3000}}, 0},
		// TheTVDB tells same-named shows apart in the name.
		{"provider year tag", "Doctor Who", 2005, []Candidate{{ID: 1, Title: "Doctor Who (2005)", Year: 2005}, {ID: 2, Title: "Doctor Who Confidential", Year: 2005}}, 1},
		{"provider country tag", "The Office", 2005, []Candidate{{ID: 1, Title: "The Office (US)", Year: 2005}}, 1},
		{"country tags decided by year", "The Office", 2001, []Candidate{{ID: 1, Title: "The Office (US)", Year: 2005}, {ID: 2, Title: "The Office (UK)", Year: 2001}}, 2},
		{"country tag in the folder name", "The Office (UK)", 0, []Candidate{{ID: 1, Title: "The Office (US)", Year: 2005}, {ID: 2, Title: "The Office (UK)", Year: 2001}}, 2},
		{"subtitle the provider leaves out", "Cosmos - A Spacetime Odyssey", 2014, []Candidate{{ID: 1, Title: "Cosmos", Year: 2014}}, 1},
		{"subtitle needs the exact year", "Cosmos - A Spacetime Odyssey", 2014, []Candidate{{ID: 1, Title: "Cosmos", Year: 1980}}, 0},
		{"subtitle, two of that name and year", "Cosmos: A Spacetime Odyssey", 2014, []Candidate{{ID: 1, Title: "Cosmos", Year: 2014}, {ID: 2, Title: "Cosmos", Year: 2014}}, 0},
		{"votes never pick a weaker title", "Example Feature", 0, []Candidate{{ID: 1, Title: "Example Feature"}, {ID: 2, Title: "Example Features", Votes: 9000}}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := confidentMatch(tc.title, tc.year, tc.candidates); got != tc.want {
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
