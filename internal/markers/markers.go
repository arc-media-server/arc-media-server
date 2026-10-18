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
	FFmpeg   func() string
	Activity *activity.Tracker
	Window   *schedule.Window

	mu     sync.Mutex
	key    string // TheIntroDB
	detect bool
	wake   chan struct{}
}

func New(st *store.Store, log *slog.Logger) *Manager {
	return &Manager{store: st, log: log, http: &http.Client{Timeout: 20 * time.Second}, detect: true, wake: make(chan struct{}, 1)}
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
func (m *Manager) Run(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(3 * time.Minute): // let start-up scans go first
	}
	for {
		if err := m.pass(ctx); err != nil && ctx.Err() == nil {
			m.log.Warn("looking for intros failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-m.wake:
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		case <-time.After(time.Hour):
		}
	}
}

// pass looks up every episode file not yet checked: chapters, then online; then, in the
// maintenance window, detection for seasons still without intros.
func (m *Manager) pass(ctx context.Context) error {
	jobs, err := m.store.MarkerJobs(ctx, time.Now().Add(-recheckAfter).Unix(), 2000)
	if err != nil || len(jobs) == 0 {
		return err
	}
	m.log.Info("looking for intros and credits", "files", len(jobs))
	m.mu.Lock()
	key, detect := m.key, m.detect
	m.mu.Unlock()
	task := m.Activity.Start("markers", "Finding intros and credits")
	defer func() { task.Finish(err) }()

	online := true
	type season struct {
		series string
		number int
	}
	missing := map[season]bool{}
	for i, j := range jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		task.Progress(i+1, len(jobs))
		have, err := m.find(ctx, j, key, &online)
		if err != nil {
			return err
		}
		if !slices.Contains(have, "intro") && j.Season >= 0 {
			missing[season{j.SeriesID, j.Season}] = true
		}
		if err := m.store.MarkersChecked(ctx, j.FileID); err != nil {
			return err
		}
	}

	ffmpeg := ""
	if m.FFmpeg != nil {
		ffmpeg = m.FFmpeg()
	}
	if !detect || ffmpeg == "" || !m.Window.Open() {
		return nil
	}
	n := 0
	for s := range missing {
		if ctx.Err() != nil || !m.Window.Open() {
			return ctx.Err()
		}
		n++
		task.Progress(n, len(missing))
		task.Detail(fmt.Sprintf("Comparing episodes (season %d of %d)", n, len(missing)))
		files, err := m.store.SeasonFiles(ctx, s.series, s.number)
		if err != nil {
			return err
		}
		found, err := detectSeason(ctx, ffmpeg, files)
		if err != nil {
			m.log.Warn("detecting intros failed", "err", err)
			continue
		}
		for fileID, ms := range found {
			if err := m.store.SaveMarkers(ctx, fileID, ms); err != nil {
				return err
			}
		}
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
	if found := fits(fromChapters(chapters), j.DurationMS, 0); len(found) > 0 {
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
	if err != nil || slices.Contains(kinds, "intro") || !*online {
		return kinds, err
	}
	var found []store.Marker
	if key != "" {
		ms, dur, err := theIntroDB(ctx, m.http, key, j.SeriesTVDB, j.SeriesTMDB, j.SeriesIMDB, j.Season, j.Episode)
		if errors.Is(err, errLimited) {
			*online = false
		} else if err != nil {
			m.log.Debug("TheIntroDB lookup failed", "err", err)
		}
		found = fits(ms, j.DurationMS, dur)
	}
	if len(found) == 0 && *online {
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
