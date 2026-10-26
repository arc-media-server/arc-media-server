// Package markers finds where episodes' intros, recaps and credits are, for Skip Intro and Skip
// Credits: from the file's chapters, online databases (TheIntroDB with a key, then IntroDB), and
// failing those by comparing a season's episodes on this computer. Timings are checked against
// each file's own length before they're trusted.
package markers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"ams/internal/activity"
	"ams/internal/background"
	"ams/internal/schedule"
	"ams/internal/store"
)

// recheckAfter is how long until a file with nothing found is looked up again (online databases
// keep growing).
const recheckAfter = 30 * 24 * time.Hour

// Manager looks for markers in the background.
type Manager struct {
	store *store.Store
	log   *slog.Logger
	http  *http.Client

	// FFmpeg finds the program for detection; Activity and Window as elsewhere (detection, which is
	// heavy, waits for the maintenance window).
	FFmpeg     func() string
	Activity   *activity.Tracker
	Window     *schedule.Window
	Background *background.Manager

	mu     sync.Mutex
	key    string // TheIntroDB
	detect bool
	wake   chan struct{}
	local  chan struct{}
}

func New(st *store.Store, log *slog.Logger) *Manager {
	return &Manager{store: st, log: log, http: &http.Client{Timeout: 20 * time.Second}, detect: true, wake: make(chan struct{}, 1), local: make(chan struct{}, 1)}
}

// Configure sets TheIntroDB's key ("" to use IntroDB alone) and whether to detect markers on this
// computer when none are found online.
func (m *Manager) Configure(theIntroDBKey string, detect bool) {
	m.mu.Lock()
	m.key, m.detect = theIntroDBKey, detect
	m.mu.Unlock()
	m.Wake()
}

// Wake looks for markers soon (after a scan adds episodes).
func (m *Manager) Wake() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Run looks for markers until ctx ends.
func (m *Manager) WakeLocal() {
	select {
	case m.local <- struct{}{}:
	default:
	}
}

func (m *Manager) Run(ctx context.Context) {
	onlineAfter := time.Now().Add(3 * time.Minute)
	select {
	case <-ctx.Done():
		return
	case <-m.local:
	case <-time.After(time.Until(onlineAfter)):
	}
	for {
		if err := m.pass(ctx, time.Now().Before(onlineAfter)); err != nil && ctx.Err() == nil {
			m.Background.AbandonTurn("markers")
			m.log.Warn("looking for intros failed", "err", err)
		}
		delay := time.Hour
		if left := time.Until(onlineAfter); left > 0 {
			delay = min(delay, left)
		}
		select {
		case <-ctx.Done():
			return
		case <-m.local:
		case <-m.wake:
			select {
			case <-ctx.Done():
				return
			case <-m.local:
			case <-time.After(time.Minute):
			}
		case <-time.After(delay):
		}
	}
}

// pass looks up every episode file not yet checked: chapters, then online; then, in the
// maintenance window, detection for seasons still without intros.
func (m *Manager) pass(ctx context.Context, localOnly ...bool) error {
	var jobs []store.MarkerJob
	var err error
	if len(localOnly) == 0 || !localOnly[0] {
		jobs, err = m.store.MarkerJobs(ctx, time.Now().Add(-recheckAfter).Unix(), 2000)
	}
	if err != nil {
		return err
	}
	m.log.Info("looking for intros and credits", "files", len(jobs))
	m.mu.Lock()
	key, detect := m.key, m.detect
	m.mu.Unlock()
	task := m.Activity.Start("markers", "Finding intros and credits")
	// Most passes find nothing to look up; only passes that did something are listed.
	worked := len(jobs) > 0
	defer func() {
		if err == nil && !worked {
			task.Discard()
			return
		}
		task.Finish(err)
	}()

	online := true
	type season struct {
		series string
		number int
	}
	missing := map[season]bool{}
	var movies []store.MarkerJob // movies still without credits
	for i, j := range jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		task.Progress(i+1, len(jobs))
		_, err := m.find(ctx, j, key, &online)
		if err != nil {
			return err
		}
		if err := m.store.MarkersChecked(ctx, j.FileID); err != nil {
			return err
		}
	}

	ffmpeg := ""
	if m.FFmpeg != nil {
		ffmpeg = m.FFmpeg()
	}
	if !detect || ffmpeg == "" || m.Background == nil && !m.Window.Open() || m.Background != nil && m.Background.Mode("markers") == "off" {
		m.Background.AbandonTurn("markers")
		return nil
	}
	localJobs, err := m.store.MarkerJobs(ctx, time.Now().Add(time.Second).Unix(), -1)
	if err != nil {
		return err
	}
	for _, j := range localJobs {
		if !m.Background.CanStart("markers", j.FileID, j.ProbedAt) {
			continue
		}
		have, err := m.store.FileMarkers(ctx, j.FileID)
		if err != nil {
			return err
		}
		intro, credits := false, false
		for _, marker := range have {
			intro = intro || marker.Kind == "intro"
			credits = credits || marker.Kind == "credits"
		}
		if j.Movie && !credits {
			movies = append(movies, j)
		}
		if !j.Movie && !intro && j.Season >= 0 {
			missing[season{j.SeriesID, j.Season}] = true
		}
	}
	n := 0
	for s := range missing {
		if ctx.Err() != nil || m.Background == nil && !m.Window.Open() {
			return ctx.Err()
		}
		n++
		task.Progress(n, len(missing))
		task.Detail(fmt.Sprintf("Comparing episodes (season %d of %d)", n, len(missing)))
		files, err := m.store.SeasonFiles(ctx, s.series, s.number)
		if err != nil {
			return err
		}
		ids := []int64{}
		for _, file := range files {
			ids = append(ids, file.FileID)
		}
		work, finish, allowed := m.Background.Begin(ctx, "markers", fmt.Sprintf("season:%s:%d", s.series, s.number), ids...)
		if !allowed {
			if m.Background.Deferred("markers") {
				return ctx.Err()
			}
			continue
		}
		worked = true
		found, err := detectSeason(work, ffmpeg, files)
		if err == nil {
			for fileID, ms := range found {
				if err = m.store.SaveMarkers(work, fileID, ms); err != nil {
					break
				}
			}
		}
		paused := work.Err() != nil
		finish(err)
		if paused {
			return ctx.Err()
		}
		if err != nil {
			m.log.Warn("detecting intros failed", "err", err)
			continue
		}

	}
	for i, j := range movies {
		if ctx.Err() != nil || m.Background == nil && !m.Window.Open() {
			return ctx.Err()
		}
		task.Progress(i+1, len(movies))
		task.Detail(fmt.Sprintf("Finding movie credits (%d of %d)", i+1, len(movies)))
		work, finish, allowed := m.Background.Begin(ctx, "markers", fmt.Sprintf("movie:%d", j.FileID), j.FileID)
		if !allowed {
			if m.Background.Deferred("markers") {
				return ctx.Err()
			}
			continue
		}
		worked = true
		found, err := detectCredits(work, ffmpeg, j.Path, j.DurationMS)
		if err == nil && found != nil {
			err = m.store.SaveMarkers(work, j.FileID, []store.Marker{*found})
		}
		paused := work.Err() != nil
		finish(err)
		if paused {
			return ctx.Err()
		}
		if err != nil {
			m.log.Warn("detecting credits failed", "file", j.Path, "err", err)
			continue
		}

	}
	if m.Background != nil {
		m.Background.CompletePass("markers")
	}
	return nil
}

// find looks one file's markers up in its chapters and online, saving what fits, and returns the
// kinds it now has. online is cleared when a service says to slow down.
func (m *Manager) find(ctx context.Context, j store.MarkerJob, key string, online *bool) ([]string, error) {
	chapters, err := m.store.FileChapters(ctx, j.FileID)
	if err != nil {
		return nil, err
	}
	fromFile := fromChapters(chapters)
	if j.Movie {
		fromFile = movieCredits(fromFile, chapters)
	}
	if found := fits(fromFile, j.DurationMS, 0); len(found) > 0 {
		if err := m.store.SaveMarkers(ctx, j.FileID, found); err != nil {
			return nil, err
		}
	}
	have := func() ([]string, error) {
		ms, err := m.store.FileMarkers(ctx, j.FileID)
		var kinds []string
		for _, mk := range ms {
			kinds = append(kinds, mk.Kind)
		}
		return kinds, err
	}
	kinds, err := have()
	wanted := "intro"
	if j.Movie {
		wanted = "credits"
	}
	if err != nil || slices.Contains(kinds, wanted) || !*online {
		return kinds, err
	}
	var found []store.Marker
	if key != "" {
		var ms []store.Marker
		var dur int64
		var err error
		if j.Movie {
			ms, dur, err = theIntroDBMovie(ctx, m.http, key, j.SeriesTMDB, j.SeriesIMDB)
		} else {
			ms, dur, err = theIntroDB(ctx, m.http, key, j.SeriesTVDB, j.SeriesTMDB, j.SeriesIMDB, j.Season, j.Episode)
		}
		if errors.Is(err, errLimited) {
			*online = false
		} else if err != nil {
			m.log.Debug("TheIntroDB lookup failed", "err", err)
		}
		found = fits(ms, j.DurationMS, dur)
	}
	if len(found) == 0 && *online && !j.Movie {
		ms, err := introDB(ctx, m.http, j.SeriesIMDB, j.Season, j.Episode)
		if errors.Is(err, errLimited) {
			*online = false
		} else if err != nil {
			m.log.Debug("IntroDB lookup failed", "err", err)
		}
		found = fits(ms, j.DurationMS, 0)
		time.Sleep(500 * time.Millisecond) // keep well within the free service's limits
	}
	if len(found) > 0 {
		if err := m.store.SaveMarkers(ctx, j.FileID, found); err != nil {
			return nil, err
		}
	}
	return have()
}
