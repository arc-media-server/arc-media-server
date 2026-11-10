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
	"path/filepath"
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
	release, ok := m.store.BeginCatalogWork()
	if !ok {
		return nil
	}
	defer release()
	var jobs []store.MarkerJob
	var err error
	if len(localOnly) == 0 || !localOnly[0] {
		jobs, err = m.store.MarkerJobs(ctx, time.Now().Add(-recheckAfter).Unix(), 2000)
	}
	if err != nil {
		return err
	}
	if len(jobs) > 0 {
		m.log.Info("looking for intros and credits", "files", len(jobs))
	}
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
	found := map[string]int{} // files by where their markers came from ("" = none)
	defer func() {
		if len(jobs) > 0 && err == nil {
			m.log.Info("intro and credit lookup finished", "files", len(jobs), "chapters", found["chapters"],
				"theintrodb", found["TheIntroDB"], "introdb", found["IntroDB"], "already_had", found["existing"], "none", found[""])
		}
	}()
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
		title, _ := m.store.FileActivityLabel(ctx, j.FileID)
		op := task.Work("Looking up intros and credits", title, "Chapters and online databases")
		res, err := m.find(ctx, j, key, &online)
		op.Finish()
		if err != nil {
			return err
		}
		found[res.source]++
		m.log.Debug("intro and credit lookup", "title", title, "file", filepath.Base(j.Path), "found", or(res.source, "nothing"), "reason", res.reason)
		if err := m.store.MarkersChecked(ctx, j.FileID); err != nil {
			return err
		}
	}

	ffmpeg := ""
	if m.FFmpeg != nil {
		ffmpeg = m.FFmpeg()
	}
	if !detect || ffmpeg == "" || m.Background == nil && !m.Window.Open() || m.Background != nil && m.Background.Mode("markers") == "off" {
		reason := "turned off in Maintenance"
		if ffmpeg == "" {
			reason = "FFmpeg not found"
		} else if m.Background == nil && !m.Window.Open() {
			reason = "waiting for maintenance hours"
		}
		m.log.Debug("intro and credit detection on this server skipped", "reason", reason)
		m.Background.AbandonTurn("markers")
		return nil
	}
	localJobs, err := m.store.MarkerJobs(ctx, time.Now().Add(time.Second).Unix(), -1)
	if err != nil {
		return err
	}
	waiting, unnumbered := 0, 0
	for _, j := range localJobs {
		if !m.Background.CanStart("markers", j.FileID, j.ProbedAt) {
			waiting++
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
		if !j.Movie && !intro && j.Season < 0 {
			unnumbered++
		}
	}
	if waiting > 0 || unnumbered > 0 || len(missing) > 0 || len(movies) > 0 {
		m.log.Debug("intro and credit detection", "seasons", len(missing), "movies", len(movies),
			"files_waiting_for_maintenance", waiting, "episodes_without_aired_number", unnumbered)
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
		show := fmt.Sprintf("Season %d", s.number)
		if series, _ := m.store.GetItem(ctx, s.series); series != nil {
			show = series.Title + " · " + show
		}
		op := task.Work("Detecting intros", show, "Comparing the season's episodes")
		found, err := detectSeason(work, ffmpeg, files)
		if err == nil {
			for fileID, ms := range found {
				if err = m.store.SaveMarkers(work, fileID, ms); err != nil {
					break
				}
			}
		}
		op.Finish()
		paused := work.Err() != nil
		finish(err)
		if paused {
			return ctx.Err()
		}
		if err != nil {
			m.log.Warn("detecting intros failed", "show", show, "err", err)
			continue
		}
		m.log.Info("compared a season's episodes for intros", "show", show, "episodes", len(files), "intros_found", len(found))

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
		title, _ := m.store.FileActivityTitle(ctx, j.FileID)
		op := task.Work("Detecting credits", title, "Looking for the closing credits")
		found, err := detectCredits(work, ffmpeg, j.Path, j.DurationMS)
		if err == nil && found != nil {
			err = m.store.SaveMarkers(work, j.FileID, []store.Marker{*found})
		}
		op.Finish()
		paused := work.Err() != nil
		finish(err)
		if paused {
			return ctx.Err()
		}
		if err != nil {
			m.log.Warn("detecting credits failed", "title", title, "file", j.Path, "err", err)
			continue
		}
		m.log.Info("looked for a movie's end credits", "title", title, "found", found != nil)

	}
	if m.Background != nil {
		m.Background.CompletePass("markers")
	}
	return nil
}

// lookup is what find did for one file: where its wanted marker came from ("chapters",
// "TheIntroDB", "IntroDB", "existing", or "" for none) and, when none, why nothing was found.
type lookup struct {
	source, reason string
}

// find looks one file's markers up in its chapters and online, saving what fits. online is
// cleared when a service says to slow down.
func (m *Manager) find(ctx context.Context, j store.MarkerJob, key string, online *bool) (lookup, error) {
	chapters, err := m.store.FileChapters(ctx, j.FileID)
	if err != nil {
		return lookup{}, err
	}
	fromFile := fromChapters(chapters)
	if j.Movie {
		fromFile = movieCredits(fromFile, chapters)
	}
	wanted := "intro"
	if j.Movie {
		wanted = "credits"
	}
	if found := fits(fromFile, j.DurationMS, 0); len(found) > 0 {
		if err := m.store.SaveMarkers(ctx, j.FileID, found); err != nil {
			return lookup{}, err
		}
		if slices.ContainsFunc(found, func(mk store.Marker) bool { return mk.Kind == wanted }) {
			return lookup{source: "chapters"}, nil
		}
	}
	have, err := m.store.FileMarkers(ctx, j.FileID)
	if err != nil {
		return lookup{}, err
	}
	if slices.ContainsFunc(have, func(mk store.Marker) bool { return mk.Kind == wanted }) {
		return lookup{source: "existing"}, nil
	}
	if !*online {
		return lookup{reason: "online lookups paused: a service asked Arc to slow down"}, nil
	}
	var found []store.Marker
	source, reason := "", ""
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
			reason = "TheIntroDB asked Arc to slow down"
		} else if err != nil {
			m.log.Debug("TheIntroDB lookup failed", "err", err)
			reason = "TheIntroDB lookup failed"
		}
		if found = fits(ms, j.DurationMS, dur); len(found) > 0 {
			source = "TheIntroDB"
		}
	}
	if len(found) == 0 && *online && !j.Movie {
		ms, err := introDB(ctx, m.http, j.SeriesIMDB, j.Season, j.Episode)
		if errors.Is(err, errLimited) {
			*online = false
			reason = "IntroDB asked Arc to slow down"
		} else if err != nil {
			m.log.Debug("IntroDB lookup failed", "err", err)
			reason = "IntroDB lookup failed"
		}
		if found = fits(ms, j.DurationMS, 0); len(found) > 0 {
			source = "IntroDB"
		}
		time.Sleep(500 * time.Millisecond) // keep well within the free service's limits
	}
	if len(found) > 0 {
		if err := m.store.SaveMarkers(ctx, j.FileID, found); err != nil {
			return lookup{}, err
		}
		if slices.ContainsFunc(found, func(mk store.Marker) bool { return mk.Kind == wanted }) {
			return lookup{source: source}, nil
		}
	}
	if reason == "" {
		switch {
		case j.Movie && key == "":
			reason = "movie credits online need a TheIntroDB key"
		case !j.Movie && (j.Season < 0 || j.Episode <= 0):
			reason = "no aired season and episode number"
		case !j.Movie && key == "" && j.SeriesIMDB == "":
			reason = "show has no IMDb ID, which IntroDB needs"
		default:
			reason = "not in the online databases"
		}
	}
	return lookup{reason: reason}, nil
}

// or is a, or b when a is empty.
func or(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
