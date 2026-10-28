package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/store"
)

const plexProduct = "Arc Media Server"

// plexDiscover is where plex.tv keeps each account's watchlist.
func plexDiscover() string {
	if u := os.Getenv("AMS_PLEX_DISCOVER_URL"); u != "" {
		return strings.TrimRight(u, "/") // a stand-in, for tests
	}
	return "https://discover.provider.plex.tv"
}

func plexTV() string {
	if u := os.Getenv("AMS_PLEX_TV_URL"); u != "" {
		return strings.TrimRight(u, "/") // a stand-in, for tests
	}
	return "https://plex.tv"
}

// PlexLogin is a "Sign in with Plex" in progress: the person opens AuthURL and approves Arc.
type PlexLogin struct {
	ID      int64  `json:"-"`
	Code    string `json:"-"`
	AuthURL string `json:"auth_url"`
}

func plexRequest(ctx context.Context, method, u, clientID, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Product", plexProduct)
	req.Header.Set("X-Plex-Client-Identifier", clientID)
	if token != "" {
		req.Header.Set("X-Plex-Token", token)
	}
	return req, nil
}

func plexDo(client *http.Client, req *http.Request, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("Plex didn't accept the sign-in; try again")
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Plex: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// StartPlexLogin begins signing in with a Plex account (clientID identifies this Arc server).
func StartPlexLogin(ctx context.Context, client *http.Client, clientID string) (*PlexLogin, error) {
	req, err := plexRequest(ctx, http.MethodPost, plexTV()+"/api/v2/pins?strong=true", clientID, "")
	if err != nil {
		return nil, err
	}
	var pin struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}
	if err := plexDo(client, req, &pin); err != nil {
		return nil, err
	}
	q := url.Values{"clientID": {clientID}, "code": {pin.Code}, "context[device][product]": {plexProduct}}
	return &PlexLogin{ID: pin.ID, Code: pin.Code, AuthURL: "https://app.plex.tv/auth#?" + q.Encode()}, nil
}

// CheckPlexLogin returns the account's token once the person has approved Arc ("" until then).
func CheckPlexLogin(ctx context.Context, client *http.Client, clientID string, l *PlexLogin) (string, error) {
	req, err := plexRequest(ctx, http.MethodGet, plexTV()+"/api/v2/pins/"+strconv.FormatInt(l.ID, 10), clientID, "")
	if err != nil {
		return "", err
	}
	var pin struct {
		AuthToken *string `json:"authToken"`
	}
	if err := plexDo(client, req, &pin); err != nil {
		return "", err
	}
	if pin.AuthToken == nil {
		return "", nil
	}
	return *pin.AuthToken, nil
}

// PlexServer is a Plex Media Server the account can reach.
type PlexServer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Owned bool   `json:"owned"`
	token string
	uris  []string
}

// PlexServers lists the servers a Plex account can use, its own first.
func PlexServers(ctx context.Context, client *http.Client, clientID, token string) ([]PlexServer, error) {
	req, err := plexRequest(ctx, http.MethodGet, plexTV()+"/api/v2/resources?includeHttps=1&includeRelay=1", clientID, token)
	if err != nil {
		return nil, err
	}
	var res []struct {
		Name             string `json:"name"`
		ClientIdentifier string `json:"clientIdentifier"`
		Provides         string `json:"provides"`
		Owned            bool   `json:"owned"`
		AccessToken      string `json:"accessToken"`
		Connections      []struct {
			URI   string `json:"uri"`
			Local bool   `json:"local"`
			Relay bool   `json:"relay"`
		} `json:"connections"`
	}
	if err := plexDo(client, req, &res); err != nil {
		return nil, err
	}
	var out []PlexServer
	for _, r := range res {
		if !strings.Contains(r.Provides, "server") {
			continue
		}
		s := PlexServer{ID: r.ClientIdentifier, Name: r.Name, Owned: r.Owned, token: r.AccessToken}
		if s.token == "" {
			s.token = token
		}
		// Local connections first, relays last.
		sort.SliceStable(r.Connections, func(i, j int) bool {
			rank := func(k int) int {
				c := r.Connections[k]
				switch {
				case c.Relay:
					return 2
				case c.Local:
					return 0
				}
				return 1
			}
			return rank(i) < rank(j)
		})
		for _, c := range r.Connections {
			s.uris = append(s.uris, c.URI)
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Owned && !out[j].Owned })
	return out, nil
}

// Plex reads watch state from a Plex Media Server.
type Plex struct {
	http     *http.Client
	clientID string
	base     string
	token    string
	// Owner is the server's account id for the signed-in person (whose resume points are read).
	owner string
	// The signed-in plex.tv account's token: its watchlist is kept on plex.tv, not the server.
	accountToken string

	once  sync.Once
	items map[string]plexItem // by ratingKey, loaded once and shared by every account
	err   error
}

type plexItem struct {
	title string
	ref   store.ImportRef
	watch store.ImportedWatch
}

// ConnectPlex finds a connection to the server that works.
func ConnectPlex(ctx context.Context, client *http.Client, clientID string, s PlexServer) (*Plex, error) {
	var last error
	for _, uri := range s.uris {
		p := &Plex{http: client, clientID: clientID, base: strings.TrimRight(uri, "/"), token: s.token}
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		var id struct{}
		err := p.get(cctx, "/identity", nil, &id)
		cancel()
		if err == nil {
			return p, nil
		}
		last = err
	}
	if last == nil {
		last = errors.New("it has no connections")
	}
	return nil, fmt.Errorf("couldn't reach %s: %w", s.Name, last)
}

// NewPlexDirect connects to a server at a known address with a token (for tests and manual setup).
func NewPlexDirect(client *http.Client, clientID, base, token string) *Plex {
	return &Plex{http: client, clientID: clientID, base: strings.TrimRight(base, "/"), token: token}
}

func (p *Plex) Name() string { return "Plex" }

func (p *Plex) get(ctx context.Context, path string, q url.Values, out any) error {
	u := p.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := plexRequest(ctx, http.MethodGet, u, p.clientID, p.token)
	if err != nil {
		return err
	}
	return plexDo(p.http, req, out)
}

// Accounts lists the people who use the server; the signed-in one is marked as the owner.
func (p *Plex) Accounts(ctx context.Context) ([]Account, error) {
	var res struct {
		MediaContainer struct {
			Account []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"Account"`
		} `json:"MediaContainer"`
	}
	if err := p.get(ctx, "/accounts", nil, &res); err != nil {
		return nil, err
	}
	var out []Account
	for _, a := range res.MediaContainer.Account {
		if a.ID <= 0 || a.Name == "" {
			continue // the server's own system account
		}
		out = append(out, Account{ID: strconv.FormatInt(a.ID, 10), Name: a.Name, Owner: a.ID == 1})
	}
	if len(out) > 0 {
		p.owner = "1"
	}
	return out, nil
}

// plexMeta is a movie, show or episode as Plex lists it.
type plexMeta struct {
	RatingKey            string  `json:"ratingKey"`
	Type                 string  `json:"type"`
	Title                string  `json:"title"`
	GrandparentTitle     string  `json:"grandparentTitle"`
	GrandparentRatingKey string  `json:"grandparentRatingKey"`
	ParentIndex          int     `json:"parentIndex"`
	Index                int     `json:"index"`
	LegacyGUID           string  `json:"guid"`
	ViewCount            int     `json:"viewCount"`
	LastViewedAt         int64   `json:"lastViewedAt"`
	ViewOffset           int64   `json:"viewOffset"`
	UserRating           float64 `json:"userRating"`
	GUIDs                []struct {
		ID string `json:"id"`
	} `json:"Guid"`
	// History entries.
	ViewedAt  int64 `json:"viewedAt"`
	AccountID int64 `json:"accountID"`
}

var legacyGUIDRe = regexp.MustCompile(`^com\.plexapp\.agents\.(imdb|themoviedb|thetvdb)://([^/?]+)(?:/(\d+)/(\d+))?`)

// ids reads a Plex item's online ids from its Guid list, or an old-style agent guid.
func (m plexMeta) ids() (tmdb, tvdb int64, imdb string, season, episode int) {
	season, episode = -1, 0
	for _, g := range m.GUIDs {
		kind, val, ok := strings.Cut(g.ID, "://")
		if !ok {
			continue
		}
		switch kind {
		case "tmdb":
			tmdb = parseID(val)
		case "tvdb":
			tvdb = parseID(val)
		case "imdb":
			imdb = val
		}
	}
	if mm := legacyGUIDRe.FindStringSubmatch(m.LegacyGUID); mm != nil {
		switch mm[1] {
		case "imdb":
			if imdb == "" {
				imdb = mm[2]
			}
		case "themoviedb":
			if tmdb == 0 {
				tmdb = parseID(mm[2])
			}
		case "thetvdb":
			// An episode's old guid is its show's TheTVDB id plus its season and number.
			if mm[3] != "" {
				season, _ = strconv.Atoi(mm[3])
				episode, _ = strconv.Atoi(mm[4])
				return tmdb, tvdb, imdb, season, episode
			}
			if tvdb == 0 {
				tvdb = parseID(mm[2])
			}
		}
	}
	return tmdb, tvdb, imdb, season, episode
}

// list reads every page of a Plex listing.
func (p *Plex) list(ctx context.Context, path string, q url.Values, progress func(int)) ([]plexMeta, error) {
	const page = 500
	var all []plexMeta
	for start := 0; ; start += page {
		q.Set("X-Plex-Container-Start", strconv.Itoa(start))
		q.Set("X-Plex-Container-Size", strconv.Itoa(page))
		var res struct {
			MediaContainer struct {
				TotalSize int        `json:"totalSize"`
				Metadata  []plexMeta `json:"Metadata"`
			} `json:"MediaContainer"`
		}
		if err := p.get(ctx, path, q, &res); err != nil {
			return nil, err
		}
		all = append(all, res.MediaContainer.Metadata...)
		if progress != nil {
			progress(len(all))
		}
		if len(res.MediaContainer.Metadata) < page || (res.MediaContainer.TotalSize > 0 && len(all) >= res.MediaContainer.TotalSize) {
			return all, nil
		}
	}
}

// load reads every movie and episode in the server's movie and TV libraries, with the signed-in
// account's own watch state.
func (p *Plex) load(ctx context.Context, progress func(string)) error {
	p.once.Do(func() {
		p.items = map[string]plexItem{}
		var sections struct {
			MediaContainer struct {
				Directory []struct {
					Key   string `json:"key"`
					Type  string `json:"type"`
					Title string `json:"title"`
				} `json:"Directory"`
			} `json:"MediaContainer"`
		}
		if p.err = p.get(ctx, "/library/sections", nil, &sections); p.err != nil {
			return
		}
		for _, sec := range sections.MediaContainer.Directory {
			path := "/library/sections/" + sec.Key + "/all"
			count := func(n int) { progress(fmt.Sprintf("reading %s (%d)", sec.Title, n)) }
			switch sec.Type {
			case "movie":
				movies, err := p.list(ctx, path, url.Values{"type": {"1"}, "includeGuids": {"1"}}, count)
				if err != nil {
					p.err = err
					return
				}
				for _, m := range movies {
					tmdb, _, imdb, _, _ := m.ids()
					p.items[m.RatingKey] = plexItem{title: m.Title, ref: store.ImportRef{Kind: "movie", TMDB: tmdb, IMDB: imdb},
						watch: m.watch()}
				}
			case "show":
				shows, err := p.list(ctx, path, url.Values{"type": {"2"}, "includeGuids": {"1"}}, count)
				if err != nil {
					p.err = err
					return
				}
				showIDs := map[string]plexMeta{}
				for _, s := range shows {
					showIDs[s.RatingKey] = s
				}
				eps, err := p.list(ctx, path, url.Values{"type": {"4"}, "includeGuids": {"1"}}, count)
				if err != nil {
					p.err = err
					return
				}
				for _, e := range eps {
					tmdb, tvdb, imdb, season, number := e.ids()
					ref := store.ImportRef{Kind: "episode", TMDB: tmdb, TVDB: tvdb, IMDB: imdb, Season: e.ParentIndex, Episode: e.Index}
					if season >= 0 && number > 0 && e.Index == 0 {
						ref.Season, ref.Episode = season, number
					}
					if sh, ok := showIDs[e.GrandparentRatingKey]; ok {
						ref.SeriesTMDB, ref.SeriesTVDB, ref.SeriesIMDB, _, _ = sh.ids()
					}
					// An old-style episode guid names its show's TheTVDB id rather than its own.
					if mm := legacyGUIDRe.FindStringSubmatch(e.LegacyGUID); mm != nil && mm[1] == "thetvdb" && mm[3] != "" {
						ref.SeriesTVDB, ref.TVDB = parseID(mm[2]), 0
					}
					title := e.GrandparentTitle + " " + fmt.Sprintf("S%02dE%02d", e.ParentIndex, e.Index)
					p.items[e.RatingKey] = plexItem{title: title, ref: ref, watch: e.watch()}
				}
			}
		}
	})
	return p.err
}

func (m plexMeta) watch() store.ImportedWatch {
	return store.ImportedWatch{Played: m.ViewCount > 0, PlayCount: m.ViewCount, LastPlayed: m.LastViewedAt,
		PositionMS: m.ViewOffset, Rating: int(m.UserRating + 0.5)}
}

// history reads the server's watch history (every account's), newest first, up to a limit.
func (p *Plex) history(ctx context.Context) ([]plexMeta, error) {
	var all []plexMeta
	const page, limit = 1000, 100000
	for start := 0; start < limit; start += page {
		q := url.Values{"sort": {"viewedAt:desc"}, "X-Plex-Container-Start": {strconv.Itoa(start)},
			"X-Plex-Container-Size": {strconv.Itoa(page)}}
		var res struct {
			MediaContainer struct {
				Metadata []plexMeta `json:"Metadata"`
			} `json:"MediaContainer"`
		}
		if err := p.get(ctx, "/status/sessions/history/all", q, &res); err != nil {
			return nil, err
		}
		all = append(all, res.MediaContainer.Metadata...)
		if len(res.MediaContainer.Metadata) < page {
			break
		}
	}
	return all, nil
}

// Entries lists what an account has watched: for the signed-in account its full watch state
// (watched, resume points, ratings), for others what the server's history shows; history for all.
func (p *Plex) Entries(ctx context.Context, account Account, progress func(string)) ([]Entry, error) {
	if err := p.load(ctx, progress); err != nil {
		return nil, err
	}
	progress("reading watch history")
	hist, err := p.history(ctx)
	if err != nil {
		return nil, err
	}
	byKey := map[string]*Entry{}
	var order []string
	add := func(key string) *Entry {
		if e, ok := byKey[key]; ok {
			return e
		}
		it, ok := p.items[key]
		if !ok {
			return nil
		}
		e := &Entry{Title: it.title, Ref: it.ref}
		byKey[key] = e
		order = append(order, key)
		return e
	}
	if account.Owner {
		for key, it := range p.items {
			w := it.watch
			if w.Played || w.PositionMS > 0 || w.Rating > 0 {
				add(key).Watch = w
			}
		}
	}
	for _, h := range hist {
		if strconv.FormatInt(h.AccountID, 10) != account.ID || h.ViewedAt == 0 {
			continue
		}
		e := add(h.RatingKey)
		if e == nil {
			continue // no longer in the library
		}
		e.History = append(e.History, h.ViewedAt)
		e.Watch.Played = true
		e.Watch.PlayCount = max(e.Watch.PlayCount, len(e.History))
		e.Watch.LastPlayed = max(e.Watch.LastPlayed, h.ViewedAt)
	}
	sort.Strings(order)
	out := make([]Entry, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	return out, nil
}

// SetAccountToken gives the signed-in plex.tv account's token, for its watchlist.
func (p *Plex) SetAccountToken(token string) { p.accountToken = token }

// listItem is a movie, show or episode as a list member (false for anything else).
func (p *Plex) listItem(m plexMeta) (ListItem, bool) {
	tmdb, tvdb, imdb, _, _ := m.ids()
	switch m.Type {
	case "movie":
		return ListItem{Title: m.Title, Ref: store.ImportRef{Kind: "movie", TMDB: tmdb, IMDB: imdb}}, true
	case "show":
		return ListItem{Title: m.Title, Ref: store.ImportRef{Kind: "series", TMDB: tmdb, TVDB: tvdb, IMDB: imdb}}, true
	case "episode":
		// Read with the libraries (load), with its show's ids.
		if it, ok := p.items[m.RatingKey]; ok {
			return ListItem{Title: it.title, Ref: it.ref}, true
		}
	}
	return ListItem{}, false
}

// Watchlist is the signed-in account's plex.tv watchlist (other accounts' can't be read).
func (p *Plex) Watchlist(ctx context.Context, account Account) ([]ListItem, error) {
	if !account.Owner || p.accountToken == "" {
		return nil, nil
	}
	const page = 100
	var out []ListItem
	for start := 0; ; start += page {
		u := plexDiscover() + "/library/sections/watchlist/all?" + url.Values{"includeGuids": {"1"},
			"X-Plex-Container-Start": {strconv.Itoa(start)}, "X-Plex-Container-Size": {strconv.Itoa(page)}}.Encode()
		req, err := plexRequest(ctx, http.MethodGet, u, p.clientID, p.accountToken)
		if err != nil {
			return nil, err
		}
		var res struct {
			MediaContainer struct {
				TotalSize int        `json:"totalSize"`
				Metadata  []plexMeta `json:"Metadata"`
			} `json:"MediaContainer"`
		}
		if err := plexDo(p.http, req, &res); err != nil {
			return nil, err
		}
		for _, m := range res.MediaContainer.Metadata {
			if li, ok := p.listItem(m); ok {
				out = append(out, li)
			}
		}
		n := len(res.MediaContainer.Metadata)
		if n < page || (res.MediaContainer.TotalSize > 0 && start+n >= res.MediaContainer.TotalSize) {
			return out, nil
		}
	}
}

// Playlists are the signed-in account's video playlists (the server lists only the token's own).
func (p *Plex) Playlists(ctx context.Context, account Account) ([]List, error) {
	if !account.Owner {
		return nil, nil
	}
	if err := p.load(ctx, func(string) {}); err != nil {
		return nil, err
	}
	lists, err := p.list(ctx, "/playlists", url.Values{"playlistType": {"video"}}, nil)
	if err != nil {
		return nil, err
	}
	var out []List
	for _, pl := range lists {
		entries, err := p.list(ctx, "/playlists/"+url.PathEscape(pl.RatingKey)+"/items", url.Values{"includeGuids": {"1"}}, nil)
		if err != nil {
			return nil, err
		}
		l := List{Title: pl.Title}
		for _, m := range entries {
			if li, ok := p.listItem(m); ok {
				l.Items = append(l.Items, li)
			}
		}
		if len(l.Items) > 0 {
			out = append(out, l)
		}
	}
	return out, nil
}

// Collections are the collections in the server's movie and TV libraries.
func (p *Plex) Collections(ctx context.Context) ([]List, error) {
	var sections struct {
		MediaContainer struct {
			Directory []struct {
				Key  string `json:"key"`
				Type string `json:"type"`
			} `json:"Directory"`
		} `json:"MediaContainer"`
	}
	if err := p.get(ctx, "/library/sections", nil, &sections); err != nil {
		return nil, err
	}
	var out []List
	for _, sec := range sections.MediaContainer.Directory {
		if sec.Type != "movie" && sec.Type != "show" {
			continue
		}
		cols, err := p.list(ctx, "/library/sections/"+url.PathEscape(sec.Key)+"/collections", url.Values{}, nil)
		if err != nil {
			return nil, err
		}
		for _, c := range cols {
			members, err := p.list(ctx, "/library/collections/"+url.PathEscape(c.RatingKey)+"/children", url.Values{"includeGuids": {"1"}}, nil)
			if err != nil {
				return nil, err
			}
			l := List{Title: c.Title}
			for _, m := range members {
				if li, ok := p.listItem(m); ok {
					l.Items = append(l.Items, li)
				}
			}
			if len(l.Items) > 0 {
				out = append(out, l)
			}
		}
	}
	return out, nil
}
