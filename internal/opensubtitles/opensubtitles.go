// Package opensubtitles searches for and downloads subtitles from OpenSubtitles.com (REST API v1).
// It needs an API key (free from opensubtitles.com); signing in with an account raises the
// number of downloads a day.
package opensubtitles

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBase = "https://api.opensubtitles.com/api/v1"

// Client talks to OpenSubtitles. The zero value isn't usable; use New.
type Client struct {
	base      string
	userAgent string
	http      *http.Client

	mu       sync.Mutex
	key      string
	username string
	password string
	token    string
}

func New(userAgent string) *Client {
	base := defaultBase
	// For development and testing against a stand-in server.
	if u := os.Getenv("AMS_OPENSUBTITLES_URL"); u != "" {
		base = strings.TrimSuffix(u, "/")
	}
	return &Client{base: base, userAgent: userAgent, http: &http.Client{Timeout: 30 * time.Second}}
}

// SetCredentials sets the API key and optional account.
func (c *Client) SetCredentials(key, username, password string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if key != c.key || username != c.username || password != c.password {
		c.token = ""
	}
	c.key, c.username, c.password = key, username, password
}

// Configured reports whether there's an API key.
func (c *Client) Configured() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.key != ""
}

// Query is what to search for. TMDB or IMDb ids are best; for an episode, the show's ids with the
// season and episode numbers.
type Query struct {
	TMDBID, ParentTMDBID int64
	IMDBID, ParentIMDBID string
	Season, Episode      int
	Title                string
	Year                 int
	// The file's OpenSubtitles hash, for exact matches.
	Hash string
	// 2-letter codes, e.g. "en", "pt-br".
	Languages []string
}

// Result is one subtitle file found.
type Result struct {
	FileID          int64   `json:"file_id"`
	Language        string  `json:"language"`
	Release         string  `json:"release"`
	FileName        string  `json:"file_name"`
	Downloads       int     `json:"downloads"`
	Rating          float64 `json:"rating"`
	HearingImpaired bool    `json:"hearing_impaired"`
	Forced          bool    `json:"forced"`
	// Made for exactly this file (the hash matched).
	HashMatch bool `json:"hash_match"`
	// Machine or AI translated, usually poorer.
	MachineTranslated bool `json:"machine_translated"`
}

// ErrNoKey means no API key has been set.
var ErrNoKey = errors.New("add an OpenSubtitles API key in Settings to search for subtitles")

// Search finds subtitles, best first.
func (c *Client) Search(ctx context.Context, q Query) ([]Result, error) {
	v := url.Values{}
	switch {
	case q.ParentTMDBID != 0:
		v.Set("parent_tmdb_id", strconv.FormatInt(q.ParentTMDBID, 10))
	case q.ParentIMDBID != "":
		v.Set("parent_imdb_id", strings.TrimPrefix(q.ParentIMDBID, "tt"))
	case q.TMDBID != 0:
		v.Set("tmdb_id", strconv.FormatInt(q.TMDBID, 10))
	case q.IMDBID != "":
		v.Set("imdb_id", strings.TrimPrefix(q.IMDBID, "tt"))
	default:
		v.Set("query", strings.ToLower(q.Title))
		if q.Year > 0 {
			v.Set("year", strconv.Itoa(q.Year))
		}
	}
	if q.Season > 0 || q.Episode > 0 {
		v.Set("season_number", strconv.Itoa(q.Season))
		v.Set("episode_number", strconv.Itoa(q.Episode))
	}
	if q.Hash != "" {
		v.Set("moviehash", q.Hash)
	}
	if len(q.Languages) > 0 {
		v.Set("languages", strings.ToLower(strings.Join(q.Languages, ",")))
	}
	var resp struct {
		Data []struct {
			Attributes struct {
				Language          string  `json:"language"`
				DownloadCount     int     `json:"download_count"`
				Ratings           float64 `json:"ratings"`
				HearingImpaired   bool    `json:"hearing_impaired"`
				ForeignPartsOnly  bool    `json:"foreign_parts_only"`
				MachineTranslated bool    `json:"machine_translated"`
				AITranslated      bool    `json:"ai_translated"`
				MovieHashMatch    bool    `json:"moviehash_match"`
				Release           string  `json:"release"`
				Files             []struct {
					FileID   int64  `json:"file_id"`
					FileName string `json:"file_name"`
				} `json:"files"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/subtitles?"+v.Encode(), nil, false, &resp); err != nil {
		return nil, err
	}
	out := []Result{}
	for _, d := range resp.Data {
		a := d.Attributes
		for _, f := range a.Files {
			out = append(out, Result{FileID: f.FileID, Language: a.Language, Release: a.Release, FileName: f.FileName,
				Downloads: a.DownloadCount, Rating: a.Ratings, HearingImpaired: a.HearingImpaired, Forced: a.ForeignPartsOnly,
				HashMatch: a.MovieHashMatch, MachineTranslated: a.MachineTranslated || a.AITranslated})
		}
	}
	// Exact matches first, then human translations, then the most downloaded.
	sortResults(out)
	return out, nil
}

func sortResults(rs []Result) {
	better := func(a, b Result) bool {
		if a.HashMatch != b.HashMatch {
			return a.HashMatch
		}
		if a.MachineTranslated != b.MachineTranslated {
			return !a.MachineTranslated
		}
		return a.Downloads > b.Downloads
	}
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && better(rs[j], rs[j-1]); j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

// Download fetches a subtitle file's contents. remaining is how many more downloads today allows.
func (c *Client) Download(ctx context.Context, fileID int64) (data []byte, name string, remaining int, err error) {
	var resp struct {
		Link      string `json:"link"`
		FileName  string `json:"file_name"`
		Remaining int    `json:"remaining"`
		Message   string `json:"message"`
	}
	body, _ := json.Marshal(map[string]any{"file_id": fileID, "sub_format": "srt"})
	if err := c.do(ctx, http.MethodPost, "/download", body, true, &resp); err != nil {
		return nil, "", 0, err
	}
	if resp.Link == "" {
		msg := resp.Message
		if msg == "" {
			msg = "no download link"
		}
		return nil, "", resp.Remaining, errors.New(msg)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resp.Link, nil)
	if err != nil {
		return nil, "", 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, "", 0, fmt.Errorf("downloading the subtitle file: %s", res.Status)
	}
	data, err = io.ReadAll(io.LimitReader(res.Body, 20<<20))
	return data, resp.FileName, resp.Remaining, err
}

// Check makes sure the API key (and account, if given) work.
func (c *Client) Check(ctx context.Context) error {
	var out map[string]any
	if err := c.do(ctx, http.MethodGet, "/infos/formats", nil, false, &out); err != nil {
		return err
	}
	c.mu.Lock()
	hasAccount := c.username != ""
	c.mu.Unlock()
	if hasAccount {
		_, err := c.login(ctx)
		return err
	}
	return nil
}

// do sends a request. auth adds the account's token when there's an account.
func (c *Client) do(ctx context.Context, method, path string, body []byte, auth bool, out any) error {
	c.mu.Lock()
	key, hasAccount := c.key, c.username != ""
	c.mu.Unlock()
	if key == "" {
		return ErrNoKey
	}
	var token string
	if auth && hasAccount {
		var err error
		if token, err = c.login(ctx); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Api-Key", key)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("OpenSubtitles couldn't be reached: %w", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 10<<20))
	switch {
	case res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden:
		return fmt.Errorf("OpenSubtitles didn't accept the API key or account (%s)", res.Status)
	case res.StatusCode == http.StatusNotAcceptable:
		// The day's download limit.
		return fmt.Errorf("OpenSubtitles' daily download limit is reached%s", apiMessage(b))
	case res.StatusCode >= 300:
		return fmt.Errorf("OpenSubtitles: %s%s", res.Status, apiMessage(b))
	}
	return json.Unmarshal(b, out)
}

func apiMessage(b []byte) string {
	var m struct {
		Message string `json:"message"`
		Errors  []string
	}
	if json.Unmarshal(b, &m) == nil {
		if m.Message != "" {
			return ": " + m.Message
		}
		if len(m.Errors) > 0 {
			return ": " + strings.Join(m.Errors, "; ")
		}
	}
	return ""
}

func (c *Client) login(ctx context.Context) (string, error) {
	c.mu.Lock()
	token, key, user, pass := c.token, c.key, c.username, c.password
	c.mu.Unlock()
	if token != "" {
		return token, nil
	}
	body, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/login", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Api-Key", key)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenSubtitles couldn't be reached: %w", err)
	}
	defer res.Body.Close()
	var out struct {
		Token string `json:"token"`
	}
	b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK || json.Unmarshal(b, &out) != nil || out.Token == "" {
		return "", fmt.Errorf("couldn't sign in to OpenSubtitles (%s)%s", res.Status, apiMessage(b))
	}
	c.mu.Lock()
	c.token = out.Token
	c.mu.Unlock()
	return out.Token, nil
}

// Hash computes OpenSubtitles' file hash: the size plus the 64-bit sums of the first and last
// 64 KB.
func Hash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	const chunk = 64 * 1024
	size := info.Size()
	if size < chunk*2 {
		return "", errors.New("file too small to hash")
	}
	sum := uint64(size)
	buf := make([]byte, chunk)
	for _, off := range []int64{0, size - chunk} {
		if _, err := f.ReadAt(buf, off); err != nil {
			return "", err
		}
		for i := 0; i < chunk; i += 8 {
			sum += binary.LittleEndian.Uint64(buf[i:])
		}
	}
	return fmt.Sprintf("%016x", sum), nil
}
