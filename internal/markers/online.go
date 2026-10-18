package markers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"ams/internal/store"
)

// Online sources of intro and credits timings, kept by their communities: IntroDB (free, by IMDb
// id) and TheIntroDB (more titles, by TheTVDB id; its free use is limited, so it's used with a key).

var errLimited = errors.New("rate limited")

func base(env, def string) string {
	if u := os.Getenv(env); u != "" {
		return u // a stand-in, for tests
	}
	return def
}

// introDB looks an episode up on IntroDB (introdb.app). Its "outro" is the credits.
func introDB(ctx context.Context, client *http.Client, imdb string, season, episode int) ([]store.Marker, error) {
	if imdb == "" || season < 0 || episode <= 0 {
		return nil, nil
	}
	q := url.Values{"imdb_id": {imdb}, "season": {strconv.Itoa(season)}, "episode": {strconv.Itoa(episode)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base("AMS_INTRODB_URL", "https://api.introdb.app")+"/segments?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ArcMediaServer")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, errLimited
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("IntroDB: %s", resp.Status)
	}
	type segment struct {
		StartMS int64 `json:"start_ms"`
		EndMS   int64 `json:"end_ms"`
	}
	var body struct {
		Intro, Recap, Outro *segment
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	var out []store.Marker
	for kind, s := range map[string]*segment{"intro": body.Intro, "recap": body.Recap, "credits": body.Outro} {
		if s != nil && s.EndMS > s.StartMS {
			out = append(out, store.Marker{Kind: kind, StartMS: s.StartMS, EndMS: s.EndMS, Source: "introdb"})
		}
	}
	return out, nil
}

// theIntroDB looks an episode up on TheIntroDB (theintrodb.org) by TheTVDB id (else TMDB's or
// IMDb's), returning the timings and the length of the version they were timed on (0 if unknown).
func theIntroDB(ctx context.Context, client *http.Client, key string, tvdb, tmdb int64, imdb string, season, episode int) ([]store.Marker, int64, error) {
	q := url.Values{}
	switch {
	case tvdb != 0:
		q.Set("tvdb_id", strconv.FormatInt(tvdb, 10))
	case tmdb != 0:
		q.Set("tmdb_id", strconv.FormatInt(tmdb, 10))
	case imdb != "":
		q.Set("imdb_id", imdb)
	default:
		return nil, 0, nil
	}
	if season < 0 || episode <= 0 {
		return nil, 0, nil
	}
	q.Set("season", strconv.Itoa(season))
	q.Set("episode", strconv.Itoa(episode))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base("AMS_THEINTRODB_URL", "https://api.theintrodb.org/v4")+"/media?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "ArcMediaServer")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, 0, nil
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, 0, errLimited
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, 0, errors.New("TheIntroDB didn't accept the key")
	case resp.StatusCode != http.StatusOK:
		return nil, 0, fmt.Errorf("TheIntroDB: %s", resp.Status)
	}
	type segment struct {
		StartMS *int64 `json:"start_ms"`
		EndMS   *int64 `json:"end_ms"`
	}
	var body struct {
		Duration *int64    `json:"group_duration_ms"`
		Intro    []segment `json:"intro"`
		Recap    []segment `json:"recap"`
		Credits  []segment `json:"credits"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, 0, err
	}
	var duration int64
	if body.Duration != nil {
		duration = *body.Duration
	}
	var out []store.Marker
	for kind, segs := range map[string][]segment{"intro": body.Intro, "recap": body.Recap, "credits": body.Credits} {
		if len(segs) == 0 {
			continue
		}
		s := segs[0] // the best-supported timing comes first
		m := store.Marker{Kind: kind, Source: "theintrodb"}
		if s.StartMS != nil {
			m.StartMS = *s.StartMS
		}
		switch {
		case s.EndMS != nil:
			m.EndMS = *s.EndMS
		case duration > 0:
			m.EndMS = duration // runs to the end
		default:
			continue
		}
		out = append(out, m)
	}
	return out, duration, nil
}
