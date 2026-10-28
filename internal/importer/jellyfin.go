package importer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ams/internal/store"
)

// Jellyfin reads every user's watch state from a Jellyfin (or Emby) server, signed in as an
// administrator.
type Jellyfin struct {
	http     *http.Client
	base     string
	deviceID string
	token    string
	// The administrator signed in as, whose view lists the server's collections.
	userID string
}

func (j *Jellyfin) Name() string { return "Jellyfin" }

func (j *Jellyfin) auth() string {
	a := fmt.Sprintf(`MediaBrowser Client="Arc Media Server", Device="Arc", DeviceId="%s", Version="1.0"`, j.deviceID)
	if j.token != "" {
		a += fmt.Sprintf(`, Token="%s"`, j.token)
	}
	return a
}

func (j *Jellyfin) do(ctx context.Context, method, path string, q url.Values, body, out any) error {
	u := j.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", j.auth())
	req.Header.Set("X-Emby-Authorization", j.auth()) // older servers and Emby
	resp, err := j.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return errors.New("Jellyfin didn't accept that user name and password")
	case resp.StatusCode == http.StatusForbidden:
		return errors.New("that Jellyfin user isn't an administrator, so it can't read other people's watch history")
	case resp.StatusCode/100 != 2:
		return fmt.Errorf("Jellyfin: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ConnectJellyfin signs in to a Jellyfin server as an administrator.
func ConnectJellyfin(ctx context.Context, client *http.Client, base, deviceID, username, password string) (*Jellyfin, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	j := &Jellyfin{http: client, base: base, deviceID: deviceID}
	var res struct {
		AccessToken string `json:"AccessToken"`
		User        struct {
			ID     string `json:"Id"`
			Policy struct {
				IsAdministrator bool `json:"IsAdministrator"`
			} `json:"Policy"`
		} `json:"User"`
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := j.do(cctx, http.MethodPost, "/Users/AuthenticateByName", nil, map[string]string{"Username": username, "Pw": password}, &res); err != nil {
		return nil, err
	}
	if !res.User.Policy.IsAdministrator {
		return nil, errors.New("sign in with a Jellyfin administrator, so Arc can read everyone's watch history")
	}
	j.token, j.userID = res.AccessToken, res.User.ID
	return j, nil
}

// Accounts lists the server's users.
func (j *Jellyfin) Accounts(ctx context.Context) ([]Account, error) {
	var users []struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	}
	if err := j.do(ctx, http.MethodGet, "/Users", nil, nil, &users); err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(users))
	for _, u := range users {
		out = append(out, Account{ID: u.ID, Name: u.Name})
	}
	return out, nil
}

type jellyfinItem struct {
	ID                string            `json:"Id"`
	Name              string            `json:"Name"`
	Type              string            `json:"Type"`
	SeriesID          string            `json:"SeriesId"`
	SeriesName        string            `json:"SeriesName"`
	ParentIndexNumber *int              `json:"ParentIndexNumber"`
	IndexNumber       *int              `json:"IndexNumber"`
	ProviderIDs       map[string]string `json:"ProviderIds"`
	UserData          struct {
		PlaybackPositionTicks int64  `json:"PlaybackPositionTicks"`
		PlayCount             int    `json:"PlayCount"`
		Played                bool   `json:"Played"`
		LastPlayedDate        string `json:"LastPlayedDate"`
	} `json:"UserData"`
}

// provider reads a provider id, whatever its case ("Tmdb", "TMDB").
func (it jellyfinItem) provider(name string) string {
	for k, v := range it.ProviderIDs {
		if strings.EqualFold(k, name) {
			return v
		}
	}
	return ""
}

// items lists a user's items of some types matching a filter, every page.
func (j *Jellyfin) items(ctx context.Context, userID, types, filter string) ([]jellyfinItem, error) {
	q := url.Values{"Recursive": {"true"}, "IncludeItemTypes": {types}}
	if filter != "" {
		q.Set("Filters", filter)
	}
	return j.pages(ctx, "/Users/"+url.PathEscape(userID)+"/Items", q)
}

// pages reads every page of an item listing, with provider ids.
func (j *Jellyfin) pages(ctx context.Context, path string, base url.Values) ([]jellyfinItem, error) {
	const page = 1000
	var all []jellyfinItem
	for start := 0; ; start += page {
		q := url.Values{"Fields": {"ProviderIds"}, "StartIndex": {strconv.Itoa(start)}, "Limit": {strconv.Itoa(page)}, "EnableImages": {"false"}}
		for k, v := range base {
			q[k] = v
		}
		var res struct {
			Items            []jellyfinItem `json:"Items"`
			TotalRecordCount int            `json:"TotalRecordCount"`
		}
		if err := j.do(ctx, http.MethodGet, path, q, nil, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Items...)
		if len(res.Items) < page || len(all) >= res.TotalRecordCount {
			return all, nil
		}
	}
}

// Entries lists what a user has watched or started.
func (j *Jellyfin) Entries(ctx context.Context, account Account, progress func(string)) ([]Entry, error) {
	progress("reading shows")
	series, err := j.items(ctx, account.ID, "Series", "")
	if err != nil {
		return nil, err
	}
	shows := map[string]jellyfinItem{}
	for _, s := range series {
		shows[s.ID] = s
	}
	progress("reading what's been watched")
	played, err := j.items(ctx, account.ID, "Movie,Episode", "IsPlayed")
	if err != nil {
		return nil, err
	}
	progress("reading what's in progress")
	resumable, err := j.items(ctx, account.ID, "Movie,Episode", "IsResumable")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []Entry
	for _, it := range append(played, resumable...) {
		if seen[it.ID] {
			continue
		}
		seen[it.ID] = true
		e := Entry{Title: it.Name, Watch: store.ImportedWatch{Played: it.UserData.Played, PlayCount: it.UserData.PlayCount,
			PositionMS: it.UserData.PlaybackPositionTicks / 10000}}
		if t, err := time.Parse(time.RFC3339Nano, it.UserData.LastPlayedDate); err == nil {
			e.Watch.LastPlayed = t.Unix()
		}
		switch it.Type {
		case "Movie":
			e.Ref = store.ImportRef{Kind: "movie", TMDB: parseID(it.provider("Tmdb")), IMDB: it.provider("Imdb")}
		case "Episode":
			e.Ref = store.ImportRef{Kind: "episode", TMDB: parseID(it.provider("Tmdb")), TVDB: parseID(it.provider("Tvdb")),
				IMDB: it.provider("Imdb"), Season: -1}
			if it.ParentIndexNumber != nil && it.IndexNumber != nil {
				e.Ref.Season, e.Ref.Episode = *it.ParentIndexNumber, *it.IndexNumber
				e.Title = fmt.Sprintf("%s S%02dE%02d", it.SeriesName, *it.ParentIndexNumber, *it.IndexNumber)
			}
			if s, ok := shows[it.SeriesID]; ok {
				e.Ref.SeriesTMDB, e.Ref.SeriesTVDB, e.Ref.SeriesIMDB = parseID(s.provider("Tmdb")), parseID(s.provider("Tvdb")), s.provider("Imdb")
			}
		default:
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

// shows maps a user's shows by id, for their episodes' show ids.
func (j *Jellyfin) shows(ctx context.Context, userID string) (map[string]jellyfinItem, error) {
	series, err := j.items(ctx, userID, "Series", "")
	if err != nil {
		return nil, err
	}
	out := map[string]jellyfinItem{}
	for _, s := range series {
		out[s.ID] = s
	}
	return out, nil
}

// listItem is a movie, show or episode as a list member (false for anything else, like a folder).
func (it jellyfinItem) listItem(shows map[string]jellyfinItem) (ListItem, bool) {
	ids := func(x jellyfinItem) (int64, int64, string) {
		return parseID(x.provider("Tmdb")), parseID(x.provider("Tvdb")), x.provider("Imdb")
	}
	switch it.Type {
	case "Movie":
		tmdb, _, imdb := ids(it)
		return ListItem{Title: it.Name, Ref: store.ImportRef{Kind: "movie", TMDB: tmdb, IMDB: imdb}}, true
	case "Series":
		tmdb, tvdb, imdb := ids(it)
		return ListItem{Title: it.Name, Ref: store.ImportRef{Kind: "series", TMDB: tmdb, TVDB: tvdb, IMDB: imdb}}, true
	case "Episode":
		tmdb, tvdb, imdb := ids(it)
		li := ListItem{Title: it.Name, Ref: store.ImportRef{Kind: "episode", TMDB: tmdb, TVDB: tvdb, IMDB: imdb, Season: -1}}
		if it.ParentIndexNumber != nil && it.IndexNumber != nil {
			li.Ref.Season, li.Ref.Episode = *it.ParentIndexNumber, *it.IndexNumber
			li.Title = fmt.Sprintf("%s S%02dE%02d", it.SeriesName, *it.ParentIndexNumber, *it.IndexNumber)
		}
		if s, ok := shows[it.SeriesID]; ok {
			li.Ref.SeriesTMDB, li.Ref.SeriesTVDB, li.Ref.SeriesIMDB = ids(s)
		}
		return li, true
	}
	return ListItem{}, false
}

func listItems(items []jellyfinItem, shows map[string]jellyfinItem) []ListItem {
	var out []ListItem
	for _, it := range items {
		if li, ok := it.listItem(shows); ok {
			out = append(out, li)
		}
	}
	return out
}

// Watchlist is the user's favourite movies and shows (Jellyfin has no separate watchlist).
func (j *Jellyfin) Watchlist(ctx context.Context, account Account) ([]ListItem, error) {
	favourites, err := j.items(ctx, account.ID, "Movie,Series", "IsFavorite")
	if err != nil {
		return nil, err
	}
	return listItems(favourites, nil), nil
}

// Playlists are the user's video playlists, in their order.
func (j *Jellyfin) Playlists(ctx context.Context, account Account) ([]List, error) {
	lists, err := j.items(ctx, account.ID, "Playlist", "")
	if err != nil {
		return nil, err
	}
	shows, err := j.shows(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	var out []List
	for _, pl := range lists {
		entries, err := j.pages(ctx, "/Playlists/"+url.PathEscape(pl.ID)+"/Items", url.Values{"UserId": {account.ID}})
		if err != nil {
			return nil, err
		}
		if items := listItems(entries, shows); len(items) > 0 {
			out = append(out, List{Title: pl.Name, Items: items})
		}
	}
	return out, nil
}

// Collections are the server's collections (box sets), as the administrator sees them.
func (j *Jellyfin) Collections(ctx context.Context) ([]List, error) {
	if j.userID == "" {
		return nil, nil
	}
	sets, err := j.items(ctx, j.userID, "BoxSet", "")
	if err != nil {
		return nil, err
	}
	var out []List
	for _, set := range sets {
		members, err := j.pages(ctx, "/Users/"+url.PathEscape(j.userID)+"/Items", url.Values{"ParentId": {set.ID}})
		if err != nil {
			return nil, err
		}
		if items := listItems(members, nil); len(items) > 0 {
			out = append(out, List{Title: set.Name, Items: items})
		}
	}
	return out, nil
}
