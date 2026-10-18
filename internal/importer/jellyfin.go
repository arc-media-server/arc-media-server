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
	j.token = res.AccessToken
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
	const page = 1000
	var all []jellyfinItem
	for start := 0; ; start += page {
		q := url.Values{"Recursive": {"true"}, "IncludeItemTypes": {types}, "Fields": {"ProviderIds"},
			"StartIndex": {strconv.Itoa(start)}, "Limit": {strconv.Itoa(page)}, "EnableImages": {"false"}}
		if filter != "" {
			q.Set("Filters", filter)
		}
		var res struct {
			Items            []jellyfinItem `json:"Items"`
			TotalRecordCount int            `json:"TotalRecordCount"`
		}
		if err := j.do(ctx, http.MethodGet, "/Users/"+url.PathEscape(userID)+"/Items", q, nil, &res); err != nil {
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
