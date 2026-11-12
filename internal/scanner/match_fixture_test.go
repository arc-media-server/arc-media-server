package scanner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// matchFixture is a stand-in TheTVDB and TMDB for one show (TVDB 404, TMDB 303) with a realistic
// shape: 8 seasons of 12 episodes, TVDB aired, DVD and absolute orders, and three TMDB episode
// groups. Every response waits delay first, like a real round trip. Connections made by provider
// clients created after it are redirected here, so no real provider is ever contacted.
type matchFixture struct {
	delay               time.Duration
	requests            atomic.Int64
	episodeOffset       atomic.Int64
	replaceFirstAndMove atomic.Bool
	episodeMode         atomic.Int64
	omitDVD             atomic.Bool
	// gate, when set, holds every request after the first series lookup until it is closed.
	mu   sync.Mutex
	gate chan struct{}
	// fail, when set, makes episode requests fail.
	fail atomic.Bool
}

const fixtureSeasons, fixtureEpisodes = 8, 12

func newMatchFixture(t testing.TB, delay time.Duration) *matchFixture {
	f := &matchFixture{delay: delay}
	srv := httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.TLSClientConfig.ServerName = "example.com"
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", srv.Listener.Addr().String())
	}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous; transport.CloseIdleConnections() })
	return f
}

// hold makes requests for episodes wait until the returned function is called.
func (f *matchFixture) hold() (release func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gate = make(chan struct{})
	gate := f.gate
	var once sync.Once
	return func() { once.Do(func() { close(gate) }) }
}

func (f *matchFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.requests.Add(1)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	path := r.URL.Path
	heavy := strings.Contains(path, "/episodes/") || strings.Contains(path, "episode_group") ||
		strings.HasPrefix(r.URL.Query().Get("append_to_response"), "season/")
	if heavy {
		f.mu.Lock()
		gate := f.gate
		f.mu.Unlock()
		if gate != nil {
			select {
			case <-gate:
			case <-r.Context().Done():
				return
			}
		}
		if f.fail.Load() {
			http.Error(w, "provider unavailable", http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case path == "/v4/login":
		fmt.Fprint(w, `{"data":{"token":"fixture-token"}}`)
	case path == "/v4/search", path == "/3/search/tv":
		fmt.Fprint(w, `{"data":[],"results":[]}`)
	case path == "/v4/series/404/extended":
		fmt.Fprint(w, `{"data":{"id":404,"name":"Right Show","year":"2020","firstAired":"2020-01-01","overview":"TVDB overview","image":"/banners/right.jpg","seasonTypes":[{"type":"official"},{"type":"dvd"},{"type":"absolute"}],"remoteIds":[{"type":12,"id":"303"}]}}`)
	case path == "/v4/series/404/translations/eng":
		fmt.Fprint(w, `{"data":{"name":"Right Show","overview":"TVDB overview"}}`)
	case strings.HasPrefix(path, "/v4/series/404/episodes/"):
		typ := strings.Split(path, "/")[5]
		var eps []string
		n := 0
		for s := 1; s <= fixtureSeasons; s++ {
			for e := 1; e <= fixtureEpisodes; e++ {
				n++
				if !f.includeEpisode(typ, e) || typ == "dvd" && f.omitDVD.Load() {
					continue
				}
				season, number := s, e
				if typ == "absolute" {
					season, number = 1, n
				}
				eps = append(eps, fmt.Sprintf(`{"id":%d,"name":"Episode %d-%d","seasonNumber":%d,"number":%d,"aired":"2020-%02d-%02d","overview":"Overview"}`,
					f.episodeID(200000, s, e), s, e, season, number, s, e))
			}
		}
		fmt.Fprintf(w, `{"data":{"episodes":[%s]},"links":{"next":null}}`, strings.Join(eps, ","))
	case path == "/3/tv/303" && strings.HasPrefix(r.URL.Query().Get("append_to_response"), "season/"):
		var parts []string
		for _, p := range strings.Split(r.URL.Query().Get("append_to_response"), ",") {
			var s int
			fmt.Sscanf(p, "season/%d", &s)
			var eps []string
			for e := 1; e <= fixtureEpisodes; e++ {
				if !f.includeEpisode("tmdb_aired", e) {
					continue
				}
				eps = append(eps, fmt.Sprintf(`{"id":%d,"name":"Episode %d-%d","season_number":%d,"episode_number":%d,"air_date":"2020-%02d-%02d","overview":"TMDB overview","runtime":43}`,
					f.episodeID(100000, s, e), s, e, s, e, s, e))
			}
			parts = append(parts, fmt.Sprintf(`"season/%d":{"overview":"Season %d","episodes":[%s]}`, s, s, strings.Join(eps, ",")))
		}
		fmt.Fprintf(w, `{%s}`, strings.Join(parts, ","))
	case path == "/3/tv/303":
		var seasons []string
		for s := 1; s <= fixtureSeasons; s++ {
			seasons = append(seasons, fmt.Sprintf(`{"season_number":%d,"name":"Season %d","poster_path":"/s%d.jpg"}`, s, s, s))
		}
		fmt.Fprintf(w, `{"id":303,"name":"Right Show","original_name":"Right Show","first_air_date":"2020-01-01","overview":"TMDB overview","poster_path":"/right.jpg","backdrop_path":"/right-bg.jpg","external_ids":{"tvdb_id":404,"imdb_id":"tt0000404"},"seasons":[%s],"episode_groups":{"results":[{"id":"g1","name":"DVD","type":3},{"id":"g2","name":"Story","type":4},{"id":"g3","name":"Volumes","type":5}]}}`,
			strings.Join(seasons, ","))
	case strings.HasPrefix(path, "/3/tv/episode_group/"):
		var groups []string
		for s := 1; s <= fixtureSeasons; s++ {
			var eps []string
			for e := 1; e <= fixtureEpisodes; e++ {
				if !f.includeEpisode("tmdb_group", e) {
					continue
				}
				eps = append(eps, fmt.Sprintf(`{"id":%d,"name":"Episode %d-%d","season_number":%d,"episode_number":%d,"order":%d}`, f.episodeID(100000, s, e), s, e, s, e, e))
			}
			groups = append(groups, fmt.Sprintf(`{"name":"Part %d","order":%d,"episodes":[%s]}`, s, s, strings.Join(eps, ",")))
		}
		fmt.Fprintf(w, `{"groups":[%s]}`, strings.Join(groups, ","))
	case path == "/3/movie/603":
		fmt.Fprint(w, `{"id":603,"imdb_id":"tt0133093","title":"The Matrix","original_title":"The Matrix","release_date":"1999-03-31","runtime":136,"overview":"A movie","poster_path":"/matrix.jpg"}`)
	default:
		http.Error(w, "not found", http.StatusNotFound)
	}
}

// showFiles is the fixture show's episode files, named with a wrong title so it scans unmatched.
func showFiles() map[string]string {
	files := map[string]string{}
	for s := 1; s <= fixtureSeasons; s++ {
		for e := 1; e <= fixtureEpisodes; e++ {
			files[fmt.Sprintf("Wrong Name/Season %02d/Wrong Name S%02dE%02d.mkv", s, s, e)] = fmt.Sprintf("s%de%d", s, e)
		}
	}
	return files
}

func (f *matchFixture) episodeID(base, season, episode int) int {
	if f.replaceFirstAndMove.Load() && season == 1 {
		if episode == 1 {
			return base + 900101
		}
		if episode == 2 {
			return base + 101
		}
	}
	return base + season*100 + episode + int(f.episodeOffset.Load())
}

func (f *matchFixture) includeEpisode(source string, episode int) bool {
	switch f.episodeMode.Load() {
	case 1:
		return false
	case 2:
		return episode <= 3
	case 3:
		return source != "official" && source != "tmdb_aired"
	}
	return true
}
