// Package ratings fetches Rotten Tomatoes' and Metacritic's scores from OMDb when
// configured. The retained IMDb dataset implementation is currently disabled.
package ratings

import (
	"ams/internal/events"
	"ams/internal/safeerr"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ams/internal/activity"
	"ams/internal/schedule"
	"ams/internal/store"
)

const (
	// IMDb's ratings for every title, updated daily (free for personal use).
	imdbURL  = "https://datasets.imdbws.com/title.ratings.tsv.gz"
	omdbBase = "https://www.omdbapi.com/"
	// IMDb's file is fetched again after a week, in the maintenance window.
	imdbEvery = 7 * 24 * time.Hour
	// Critics' scores are asked for again after a month.
	criticsEvery = 30 * 24 * time.Hour
	// OMDb's free keys allow 1,000 requests a day; this stays under it.
	omdbPerHour = 40
)

// Manager keeps Rotten Tomatoes and Metacritic ratings up to date in the background.
type Manager struct {
	// Events is configured before workers start; invalidations contain no item metadata.
	Events *events.Hub
	store  *store.Store
	dir    string
	log    *slog.Logger
	http   *http.Client

	// Activity, if set, is told about downloads. Window, if set, is when the weekly download runs.
	Activity *activity.Tracker
	Window   *schedule.Window

	mu      sync.Mutex
	omdbKey string
	// OMDb said the day's requests are used up; try again after this.
	omdbPause time.Time
	wake      chan struct{}
}

func New(dir string, st *store.Store, log *slog.Logger) *Manager {
	return &Manager{store: st, dir: dir, log: log, http: &http.Client{Timeout: 2 * time.Minute}, wake: make(chan struct{}, 1)}
}

// SetOMDbKey sets the OMDb key ("" turns Rotten Tomatoes and Metacritic off).
func (m *Manager) SetOMDbKey(key string) {
	m.mu.Lock()
	changed := key != m.omdbKey
	m.omdbKey, m.omdbPause = key, time.Time{}
	m.mu.Unlock()
	if changed {
		m.Changed()
	}
}

// Changed asks for ratings for titles that don't have them yet (after a scan).
func (m *Manager) Changed() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) file() string { return filepath.Join(m.dir, "title.ratings.tsv.gz") }

// Run keeps ratings up to date until ctx ends.
func (m *Manager) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(2 * time.Minute): // let start-up scans go first
	}
	for {
		m.pass(ctx)
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
			// A scan often ends several in a row; wait for things to settle.
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
		case <-time.After(time.Hour):
		}
	}
}

func (m *Manager) pass(ctx context.Context) {
	release, ok := m.store.BeginCatalogWork()
	if !ok {
		return
	}
	defer release()
	// IMDb ingestion is paused: neither download nor apply a cached dataset. See
	// docs/RATING-SOURCES.md before restoring this path or removing it permanently.
	if err := m.fetchCritics(ctx); err != nil && ctx.Err() == nil {
		m.log.Warn("fetching critics' ratings failed", "err", err)
	}
}

func ratingsURL() string {
	if u := os.Getenv("AMS_IMDB_RATINGS_URL"); u != "" {
		return u // a stand-in, for tests
	}
	return imdbURL
}

// download fetches IMDb's ratings file (about 8 MB) into the data folder.
func (m *Manager) download(ctx context.Context) error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ratingsURL(), nil)
	if err != nil {
		return err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IMDb ratings file: %s", resp.Status)
	}
	tmp := m.file() + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, m.file())
}

// applyIMDb gives every title with an IMDb id its rating and vote count from the downloaded file.
func (m *Manager) applyIMDb(ctx context.Context, task *activity.Handle) error {
	ids, err := m.store.IMDbIDs(ctx)
	if err != nil || len(ids) == 0 {
		return err
	}
	f, err := os.Open(m.file())
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer zr.Close()
	task.Detail("Matching ratings to your titles")
	found := map[string]store.IMDbRating{}
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		line := sc.Text()
		tab := strings.IndexByte(line, '\t')
		if tab < 0 || !ids[line[:tab]] {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		rating, err1 := strconv.ParseFloat(parts[1], 64)
		votes, err2 := strconv.Atoi(parts[2])
		if err1 == nil && err2 == nil {
			found[parts[0]] = store.IMDbRating{Rating: rating, Votes: votes}
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	if err := m.store.SetIMDbRatings(ctx, found); err != nil {
		return err
	}
	task.Detail(fmt.Sprintf("%d titles rated", len(found)))
	return nil
}

// fetchCritics asks OMDb for Rotten Tomatoes' and Metacritic's scores of titles that don't have
// them or haven't been checked for a month, a few an hour.
func (m *Manager) fetchCritics(ctx context.Context) error {
	m.mu.Lock()
	key, pause := m.omdbKey, m.omdbPause
	m.mu.Unlock()
	if key == "" || time.Now().Before(pause) {
		return nil
	}
	items, err := m.store.ItemsNeedingCritics(ctx, time.Now().Add(-criticsEvery).Unix(), omdbPerHour)
	if err != nil {
		return err
	}
	for _, it := range items {
		rt, mc, err := omdbScores(ctx, m.http, key, it.IMDBID)
		if errors.Is(err, errOMDbLimit) {
			m.mu.Lock()
			m.omdbPause = time.Now().Add(12 * time.Hour)
			m.mu.Unlock()
			return nil
		}
		if err != nil {
			return err
		}
		if err := m.store.SetCriticRatings(ctx, it.ID, rt, mc); err != nil {
			return err
		}
		m.Events.Publish(events.Event{Topic: "catalog", LibraryID: it.LibraryID})
	}
	return nil
}

var (
	errOMDbLimit = errors.New("OMDb's daily limit is used up")
	errOMDbKey   = errors.New("OMDb didn't accept the key")
)

// omdbScores returns a title's Rotten Tomatoes critics' score and Metacritic score (nil when OMDb
// has none).
func omdbScores(ctx context.Context, client *http.Client, key, imdbID string) (rt, mc *int, err error) {
	u := omdbBase + "?" + url.Values{"i": {imdbID}, "apikey": {key}}.Encode()
	if base := os.Getenv("AMS_OMDB_URL"); base != "" {
		u = base + "?" + url.Values{"i": {imdbID}, "apikey": {key}}.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, nil, safeerr.Transport(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, safeerr.Transport(err)
	}
	defer resp.Body.Close()
	var body struct {
		Response string `json:"Response"`
		Error    string `json:"Error"`
		Ratings  []struct {
			Source string `json:"Source"`
			Value  string `json:"Value"`
		} `json:"Ratings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, nil, fmt.Errorf("OMDb: %s", resp.Status)
	}
	if body.Response == "False" {
		switch {
		case strings.Contains(body.Error, "limit"):
			return nil, nil, errOMDbLimit
		case strings.Contains(strings.ToLower(body.Error), "api key"):
			return nil, nil, errOMDbKey
		}
		return nil, nil, nil // e.g. "Incorrect IMDb ID": nothing to show
	}
	for _, r := range body.Ratings {
		switch r.Source {
		case "Rotten Tomatoes":
			if n, err := strconv.Atoi(strings.TrimSuffix(r.Value, "%")); err == nil {
				rt = &n
			}
		case "Metacritic":
			if n, err := strconv.Atoi(strings.SplitN(r.Value, "/", 2)[0]); err == nil {
				mc = &n
			}
		}
	}
	return rt, mc, nil
}

// CheckOMDbKey makes a request with key to see whether OMDb accepts it.
func CheckOMDbKey(ctx context.Context, key string) error {
	_, _, err := omdbScores(ctx, &http.Client{Timeout: 20 * time.Second}, key, "tt0111161")
	if errors.Is(err, errOMDbKey) {
		return errors.New("OMDb didn't accept that key (new keys work once you've clicked the link in OMDb's email)")
	}
	return err
}
