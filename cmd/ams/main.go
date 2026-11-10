// Command ams runs Arc Media Server. Everything is set up in the web app; the only thing chosen
// outside it is where Arc keeps its data (-data).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"ams/internal/activity"
	"ams/internal/api"
	"ams/internal/artwork"
	"ams/internal/auth"
	"ams/internal/background"
	"ams/internal/backup"
	"ams/internal/config"
	"ams/internal/events"
	"ams/internal/lifecycle"
	"ams/internal/logbuf"
	"ams/internal/markers"
	"ams/internal/metadata"
	"ams/internal/notify"
	"ams/internal/opensubtitles"
	"ams/internal/optimize"
	"ams/internal/probe"
	"ams/internal/ratings"
	"ams/internal/remote"
	"ams/internal/scanner"
	"ams/internal/schedule"
	"ams/internal/settings"
	"ams/internal/store"
	"ams/internal/subtitles"
	"ams/internal/sysstats"
	"ams/internal/transcode"
	"ams/internal/trickplay"
	"ams/internal/upkeep"
	"ams/internal/watcher"
)

// The most recent log lines, which the admin can read on the Dashboard.
var logs = logbuf.New(2000)
var revision = "unknown" // injected for container builds

const version = "0.1.0-dev"

func main() {
	dataDir := flag.String("data", "", "folder for Arc's database, artwork cache and backups (default: data, or AMS_DATA)")
	legacyConfig := flag.String("config", "ams.yaml", "an old config file to import settings from, once")
	debug := flag.Bool("debug", false, "enable debug logging")
	resetPassword := flag.Bool("reset-password", false, "forgot the password? set a new random one, print it and exit")
	flag.Parse()

	logs.SetDetailed(*debug)
	log := slog.New(logs.Wrap(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logs.Level()})))
	defer logs.Close()

	if *resetPassword {
		if err := resetAdminPassword(*dataDir); err != nil {
			log.Error("resetting the password failed", "err", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*dataDir, *legacyConfig, log); err != nil {
		log.Error("fatal", "err", err)
		logs.Close()
		os.Exit(1)
	}
}

func run(dataDir, legacyConfig string, log *slog.Logger) error {
	// An old config file is only read to import its settings (and, the first time, its data folder).
	var legacy *config.Config
	if _, err := os.Stat(legacyConfig); err == nil {
		if legacy, err = config.Load(legacyConfig); err != nil {
			return err
		}
	}
	if dataDir == "" {
		dataDir = os.Getenv("AMS_DATA")
	}
	if dataDir == "" && legacy != nil {
		dataDir = legacy.DataDir
	}
	if dataDir == "" {
		dataDir = "data"
	}
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workers := lifecycle.New(ctx, log)
	ctx = workers.Context()

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	if err := logs.EnableArchive(filepath.Join(dataDir, "logs")); err != nil {
		log.Warn("persistent logging unavailable", "err", err)
	}
	backupDir := filepath.Join(dataDir, "backups")
	st, err := store.Open(ctx, filepath.Join(dataDir, "ams.db"), store.Options{
		// A new AMS version is about to change the database: keep a copy of it as it was.
		BeforeMigrate: func(s *store.Store) error {
			_, err := backup.New(backupDir, s, log).Create(ctx, backup.ReasonBeforeUpgrade)
			return err
		},
	})
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()
	defer workers.Close() // Always cancel/join writers before closing the store.
	var apiWorkers sync.WaitGroup
	workers.OnStop("asynchronous API work", apiWorkers.Wait)
	changes := events.New()
	tasks := activity.New()
	tasks.SetOnChange(func() { changes.Publish(events.Event{Topic: "activity", Admin: true}) })
	notifier := notify.NewWithContext(ctx, st, log)
	workers.OnStop("webhook deliveries", notifier.Close)
	// The maintenance window; its hours come from the settings below.
	window := schedule.New(2, 5)
	backups := backup.New(backupDir, st, log)
	backups.Activity = tasks
	backups.Window = window
	backups.OnFail = func(err error) { notifier.Problem("The automatic backup failed: " + err.Error()) }

	if legacy != nil {
		if err := importLegacyConfig(ctx, legacy, legacyConfig, st, log); err != nil {
			return err
		}
	}
	cfg, err := settings.Load(ctx, st)
	if err != nil {
		return err
	}
	redactSettings := func(s settings.Settings) {
		logs.SetSecrets(s.TMDBKey, s.TVDBKey, s.OMDbKey, s.OpenSubtitlesKey, s.OpenSubtitlesPassword, s.TheIntroDBKey)
	}
	redactSettings(cfg)
	window.Set(cfg.MaintenanceStart, cfg.MaintenanceEnd)
	bg, err := background.New(ctx, st, window, tasks)
	if err != nil {
		return err
	}
	bg.Events = changes
	bg.Log = log
	bg.Configure(cfg)
	workers.Go("background monitor", func() { bg.Monitor(ctx) })
	workers.Go("database maintenance", func() {
		bg.RunUpkeep(ctx, "database", func(ctx context.Context, allowed func() bool) (string, error) {
			result, err := st.OptimizeDatabase(ctx, allowed)
			detail := fmt.Sprintf("Database size %s → %s; integrity checked; query statistics updated", upkeep.FormatBytes(result.BeforeBytes), upkeep.FormatBytes(result.AfterBytes))
			if result.Vacuumed {
				detail += "; compacted"
			} else if result.VacuumSkipped != "" {
				detail += "; " + result.VacuumSkipped
			}
			if result.CheckpointBusy {
				detail += "; active readers delayed final disk reclamation"
			}
			if err != nil {
				detail = "Database upkeep did not complete"
			}
			return detail, err
		})
	})
	workers.Go("cache maintenance", func() {
		bg.RunUpkeep(ctx, "cache", func(ctx context.Context, allowed func() bool) (string, error) {
			result, err := upkeep.CleanCaches(ctx, st, dataDir, allowed)
			return result.String(), err
		})
	})
	backups.Allowed = func() bool { return bg.Ready("backup") && !bg.Forced("backup") }
	bg.Register("backup", func() {
		workers.Go("requested backup", func() {
			defer bg.CompletePass("backup")
			for !bg.Ready("backup") {
				if ctx.Err() != nil || bg.Mode("backup") == "off" {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				}
			}
			if _, err := backups.Create(ctx, backup.ReasonAuto); err != nil {
				log.Warn("requested backup failed", "err", err)
			}
		})
	})
	workers.Go("backup scheduler", func() { backups.Run(ctx) })
	// Critics' ratings (Rotten Tomatoes and Metacritic with an OMDb key). IMDb is paused.
	extRatings := ratings.New(filepath.Join(dataDir, "ratings"), st, log)
	extRatings.Events = changes
	extRatings.Activity, extRatings.Window = tasks, window
	extRatings.SetOMDbKey(cfg.OMDbKey)
	workers.Go("ratings maintenance", func() { extRatings.Run(ctx) })

	prober := probe.New("ffprobe")
	var ffMu sync.Mutex
	ffStatus := map[string]any{}
	// FFmpeg converts files the player can't play as they are; it's found beside ffprobe.
	var ffmpegBin string
	findFFprobe := func(preferred string) error {
		bin, v, err := probe.Find(ctx, preferred)
		ffMu.Lock()
		defer ffMu.Unlock()
		if err != nil {
			ffStatus = map[string]any{"found": false, "error": err.Error()}
			return err
		}
		prober.SetBin(bin)
		ffmpegBin = probe.FFmpegNextTo(bin)
		ffStatus = map[string]any{"found": true, "path": bin, "version": v, "ffmpeg": ffmpegBin}
		return nil
	}
	if err := findFFprobe(cfg.FFprobePath); err != nil {
		log.Warn("ffprobe not found; files can't be read until it's installed or set in Settings", "err", err)
	} else {
		log.Info("found ffprobe", "path", prober.Bin())
		if ffmpegBin == "" {
			log.Warn("FFmpeg not found next to ffprobe or on the PATH; files browsers can't play won't be converted")
		}
	}
	transcoder := transcode.New(filepath.Join(dataDir, "transcode"), func() string {
		ffMu.Lock()
		defer ffMu.Unlock()
		return ffmpegBin
	}, prober.Bin, log)
	workers.OnStop("conversion cleanup", transcoder.Close)
	workers.Go("conversion monitor", func() { transcoder.Run(ctx) })
	workers.Go("hardware detection", func() { transcoder.Detect(ctx) })

	tmdb, tvdb := metadata.NewTMDB(cfg.TMDBKey), metadata.NewTVDB(cfg.TVDBKey)
	subsOnline := opensubtitles.New("Arc Media Server v" + version)
	subsOnline.SetCredentials(cfg.OpenSubtitlesKey, cfg.OpenSubtitlesUser, cfg.OpenSubtitlesPassword)
	tmdb.SetRegion(cfg.Region)
	if cfg.TMDBKey == "" {
		log.Warn("no TMDB API key yet; add one in Settings so Arc can identify movies and shows")
	}

	art := artwork.New(filepath.Join(dataDir, "artwork"), log)
	art.Events = changes
	artKeeper := art.Keep(ctx, func(ctx context.Context) ([]string, error) {
		images, err := st.ItemImages(ctx)
		if err != nil {
			return nil, err
		}
		sources := make([]string, 0, len(images))
		var full []string // full-resolution backdrops for banners, larger, so fetched last
		for _, im := range images {
			// Uploaded artwork is already on disk.
			if src := artwork.Source(im.Kind, im.Type, im.Path); strings.HasPrefix(src, "http") {
				sources = append(sources, src)
			}
			if im.Type == "backdrop" {
				if src := artwork.FullSource(im.Kind, im.Path); src != "" {
					full = append(full, src)
				}
			}
		}
		return append(sources, full...), nil
	})

	workers.OnStop("artwork keeper", artKeeper.Close)

	sc := scanner.New(ctx, st, prober, tmdb, tvdb, log)
	workers.OnStop("scanner jobs", sc.Wait)
	sc.Events = changes
	sc.Activity = tasks
	sc.Window = window
	// Automatic trash never uses Run now's outside-window override. It is independent of
	// metadata refresh and ordinary automatic backup settings, but always takes a safety backup.
	workers.Go("automatic trash", func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		allowed := func() bool { return window.OpenAt(time.Now()) && bg.Ready("empty_trash") }
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if !allowed() {
					continue
				}
				for _, id := range sc.TrashCandidates() {
					_, err := sc.EmptyTrash(ctx, id, true, func(ctx context.Context) error {
						_, err := backups.Create(ctx, backup.ReasonBeforeDelete)
						return err
					}, allowed)
					if err != nil && !errors.Is(err, scanner.ErrTrashUnsafe) && !errors.Is(err, scanner.ErrLibraryBusy) && ctx.Err() == nil {
						log.Warn("automatic empty trash failed", "library", id, "err", err)
					}
				}
			}
		}
	})

	sc.ThumbDir = filepath.Join(dataDir, "videothumbs")
	sc.FFmpeg = func() string {
		ffMu.Lock()
		defer ffMu.Unlock()
		return ffmpegBin
	}
	// After each scan, download new artwork so it's there offline.
	var skips *markers.Manager // set below; scans finishing wake it
	sc.OnFinish = func(libraryID int64) {
		changes.Publish(events.Event{Topic: "library", LibraryID: libraryID})
		bg.Wake("previews")
		bg.Wake("analysis")
		bg.Wake("markers")
		artKeeper.Trigger()
		extRatings.Changed()
		if skips != nil {
			skips.Wake()
		}
		// Tell webhooks what's new, or that the scan failed.
		status := sc.Status(libraryID)
		lib, err := st.GetLibrary(ctx, libraryID)
		if err != nil || lib == nil {
			return
		}
		if status.Error != "" {
			notifier.Problem(fmt.Sprintf("Scanning %s failed: %s", lib.Name, status.Error))
			return
		}
		notifier.LibraryScanned(ctx, *lib, status.StartedAt.Unix())
	}
	artKeeper.Trigger()

	// Scan libraries when their folders change.
	watch := watcher.New(sc.Request, log)
	scanConfig := func(c settings.Settings) watcher.Config {
		return watcher.Config{Automatic: c.ScanAutomatic, Partial: c.ScanPartial, Interval: time.Duration(c.ScanMinutes()) * time.Minute}
	}
	watch.Configure(scanConfig(cfg))
	watch.ScanPaths = sc.RequestPaths
	workers.Go("library watcher", func() { watch.Run(ctx) })
	// librariesChanged brings the watcher in step with the libraries, optionally scanning them all.
	// Timeline thumbnails, made in the background for libraries that have them turned on.
	thumbs := trickplay.New(filepath.Join(dataDir, "trickplay"), st, func() string {
		ffMu.Lock()
		defer ffMu.Unlock()
		return ffmpegBin
	}, log)
	thumbs.Activity = tasks
	thumbs.Background = bg
	bg.Register("previews", thumbs.Wake)
	workers.Go("media analysis", func() { bg.RunAnalysis(ctx, sc.EnrichFormatFile) })
	workers.Go("timeline previews", func() { thumbs.Run(ctx) })
	// Intro and credits markers, found in the background.
	skips = markers.New(st, log)
	skips.Activity, skips.Window = tasks, window
	skips.FFmpeg = func() string {
		ffMu.Lock()
		defer ffMu.Unlock()
		return ffmpegBin
	}
	sc.MaintenanceAllowed = func() bool { return bg.MaintenanceReady() && !sc.DiscoveryPending() }
	skips.Background = bg
	bg.Register("markers", skips.WakeLocal)
	skips.Configure(cfg.TheIntroDBKey, cfg.DetectionMode != "off")
	workers.Go("intro detection", func() { skips.Run(ctx) })
	// Optimized versions, made one at a time in the background.
	optimizer := optimize.New(filepath.Join(dataDir, "optimized"), st, log)
	optimizer.Events = changes
	optimizer.Activity = tasks
	optimizer.FFmpeg = skips.FFmpeg
	optimizer.Probe = prober.Probe
	optimizer.ToneMap = func() bool { return transcoder.Caps().ToneMapping }
	workers.Go("optimized copies", func() { optimizer.Run(ctx) })
	librariesChanged := func(scan bool) {
		thumbs.Wake()
		libs, err := st.ListLibraries(ctx)
		if err != nil {
			log.Error("listing libraries failed", "err", err)
			return
		}
		folders := map[int64][]string{}
		for _, l := range libs {
			folders[l.ID] = l.Paths
			if scan {
				sc.Start(l.ID)
			}
		}
		watch.SetLibraries(folders)
	}
	librariesChanged(cfg.ScanOnStart)

	var refreshing atomic.Bool
	bg.Register("metadata", func() {
		if !refreshing.CompareAndSwap(false, true) {
			return
		}
		workers.Go("requested metadata", func() {
			defer refreshing.Store(false)
			defer bg.CompletePass("metadata")
			libs, err := st.ListLibraries(ctx)
			if err != nil {
				return
			}
			for _, lib := range libs {
				for ctx.Err() == nil {
					if bg.Mode("metadata") == "off" {
						return
					}
					if !bg.MaintenanceReady() {
						select {
						case <-ctx.Done():
							return
						case <-time.After(time.Second):
						}
						continue
					}
					if !sc.Maintain(lib.ID) {
						select {
						case <-ctx.Done():
							return
						case <-time.After(time.Second):
						}
						continue
					}
					for sc.Status(lib.ID).Running {
						select {
						case <-ctx.Done():
							return
						case <-time.After(250 * time.Millisecond):
						}
					}
					if sc.Status(lib.ID).Phase != "Metadata refresh paused" {
						break
					}
				}
			}
		})
	})
	// Once a night, refresh known titles without discovering media folders,
	// and catch up on scrubbing previews. Backups check the window themselves.
	workers.Go("maintenance scheduler", func() {
		last := ""
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
			night := window.Night(time.Now())
			if !bg.Ready("metadata") || night == last {
				continue
			}
			last = night
			task := tasks.Start("maintenance", "Nightly maintenance")
			libs, err := st.ListLibraries(ctx)
			bg.Wake("metadata")
			thumbs.Wake()
			task.Detail(fmt.Sprintf("Requested known-title maintenance for %d libraries", len(libs)))
			task.Finish(err)
		}
	})

	// Sessions unused for 90 days are forgotten.
	workers.Go("session retention", func() {
		for {
			started := time.Now()
			removed, err := st.DeleteStaleSessions(ctx, started.Add(-90*24*time.Hour).Unix())
			if ctx.Err() == nil {
				if err != nil {
					log.Warn("stale session cleanup failed", "err", err, "took", time.Since(started))
				} else {
					log.Info("stale session cleanup finished", "removed", removed, "took", time.Since(started))
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(24 * time.Hour):
			}
		}
	})

	// The web server can move to a new address when the port or network setting changes.
	var mu sync.Mutex
	current := cfg
	// Links in webhook messages point at this server: its network address when other devices can
	// reach it.
	notifier.BaseURL = func() string {
		mu.Lock()
		c := current
		mu.Unlock()
		return publicURL(c)
	}
	listeners := make(chan net.Listener, 1)
	workers.OnStop("pending listeners", func() {
		for {
			select {
			case pending := <-listeners:
				_ = pending.Close()
			default:
				return
			}
		}
	})
	// Where converted streams are written: the folder chosen in Settings (in an "Arc Transcode"
	// folder of its own), or the data folder's.
	transcodeDir := func(c settings.Settings) string {
		if c.TranscodeDir != "" {
			return filepath.Join(c.TranscodeDir, "Arc Transcode")
		}
		return filepath.Join(dataDir, "transcode")
	}
	// Remote access: HTTPS on the same port, the router's port forwarding, the outside address.
	remoteAccess := remote.New(remote.NewCerts(filepath.Join(dataDir, "certs")), remote.NewMapper(log))
	remoteAccess.Certs.Configure(cfg.HTTPSMode, cfg.HTTPSCert, cfg.HTTPSKey, cfg.HTTPSDomain)
	applyHomeNetworks(cfg.HomeNetworks, cfg.TrustedProxies)
	transcoder.Configure(transcodeDir(cfg), cfg.MaxTranscodes)
	remoteAccess.Configure(cfg.RemoteAccess, cfg.UPnP, listenPort(cfg), cfg.OutsidePort(), cfg.HTTPSDomain)
	workers.Go("remote mapping", func() { remoteAccess.Mapper.Run(ctx) })
	// CPU, memory and network use for the Dashboard's graphs.
	stats := sysstats.New()
	workers.Go("system statistics", func() { stats.Run(ctx) })
	// Caller holds mu. Restores use this without validating provider keys online or
	// rewriting restored settings; recovery must also work while providers are down.
	useSettings := func(next settings.Settings) {
		redactSettings(next)
		tmdb.SetKey(next.TMDBKey)
		tmdb.SetRegion(next.Region)
		tvdb.SetKey(next.TVDBKey)
		subsOnline.SetCredentials(next.OpenSubtitlesKey, next.OpenSubtitlesUser, next.OpenSubtitlesPassword)
		extRatings.SetOMDbKey(next.OMDbKey)
		skips.Configure(next.TheIntroDBKey, next.DetectionMode != "off")
		window.Set(next.MaintenanceStart, next.MaintenanceEnd)
		bg.Configure(next)
		current = next
		watch.Configure(scanConfig(next))
		applyHomeNetworks(next.HomeNetworks, next.TrustedProxies)
		transcoder.Configure(transcodeDir(next), next.MaxTranscodes)
		remoteAccess.Certs.Configure(next.HTTPSMode, next.HTTPSCert, next.HTTPSKey, next.HTTPSDomain)
		remoteAccess.Configure(next.RemoteAccess, next.UPnP, listenPort(next), next.OutsidePort(), next.HTTPSDomain)
	}
	applySettings := func(ctx context.Context, before, next settings.Settings) error {
		mu.Lock()
		stale := current != before
		mu.Unlock()
		if stale {
			return settings.ErrConcurrentChange
		}
		// Provider validation shares one bounded budget and never holds the settings reader lock.
		validation, stopValidation := context.WithTimeout(ctx, 30*time.Second)
		defer stopValidation()
		if next.TMDBKey != before.TMDBKey && next.TMDBKey != "" {
			if err := metadata.CheckTMDBKey(validation, next.TMDBKey); err != nil {
				return err
			}
		}
		if next.OpenSubtitlesKey != "" && (next.OpenSubtitlesKey != before.OpenSubtitlesKey ||
			next.OpenSubtitlesUser != before.OpenSubtitlesUser || next.OpenSubtitlesPassword != before.OpenSubtitlesPassword) {
			check := opensubtitles.New("Arc Media Server v" + version)
			check.SetCredentials(next.OpenSubtitlesKey, next.OpenSubtitlesUser, next.OpenSubtitlesPassword)
			if err := check.Check(validation); err != nil {
				return err
			}
		}
		if next.OMDbKey != before.OMDbKey && next.OMDbKey != "" {
			if err := ratings.CheckOMDbKey(validation, next.OMDbKey); err != nil {
				return err
			}
		}
		if next.TVDBKey != before.TVDBKey && next.TVDBKey != "" {
			if err := metadata.CheckTVDBKey(validation, next.TVDBKey); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		mu.Lock()
		defer mu.Unlock()
		if current != before {
			return settings.ErrConcurrentChange
		}
		if next.FFprobePath != current.FFprobePath {
			if err := findFFprobe(next.FFprobePath); err != nil && next.FFprobePath != "" {
				findFFprobe(current.FFprobePath)
				return fmt.Errorf("no working ffprobe at %s", next.FFprobePath)
			}
			// A different FFmpeg may support different hardware.
			workers.Go("hardware detection", func() { transcoder.Detect(workers.Context()) })
		}
		var ln net.Listener
		if next.ListenAddress() != current.ListenAddress() {
			var err error
			if ln, err = net.Listen("tcp", next.ListenAddress()); err != nil {
				return fmt.Errorf("can't use port %d: %w", next.Port, err)
			}
		}
		if err := settings.Save(ctx, st, next); err != nil {
			if ln != nil {
				ln.Close()
			}
			return err
		}
		useSettings(next)
		if ln != nil {
			select {
			case listeners <- ln:
			case <-workers.Context().Done():
				_ = ln.Close()
			}
		}
		return nil
	}

	subtitleCache := subtitles.NewCacheWithContext(ctx, filepath.Join(dataDir, "subtitles"), func() string {
		ffMu.Lock()
		defer ffMu.Unlock()
		return ffmpegBin
	}, log)
	workers.OnStop("subtitle extraction", subtitleCache.Close)
	handler := api.New(api.Deps{
		WorkContext: ctx, Workers: &apiWorkers,
		Store: st, Scanner: sc, Background: bg, Events: changes, WatchProblem: watch.Problem, Artwork: art, Backups: backups, Activity: tasks, Notify: notifier, Markers: skips, Optimize: optimizer, Remote: remoteAccess, Stats: stats, Log: log, Version: version,
		LibrariesChanged: func() { librariesChanged(false) },
		AfterRestore: func(ctx context.Context) error {
			bg.Reset(ctx)
			next, err := settings.Load(ctx, st)
			if err != nil {
				return err
			}
			mu.Lock()
			var ln net.Listener
			var listenErr error
			if next.ListenAddress() != current.ListenAddress() {
				ln, listenErr = net.Listen("tcp", next.ListenAddress())
			}
			// Apply restored access controls even if its old port cannot be opened.
			useSettings(next)
			mu.Unlock()
			if err := findFFprobe(next.FFprobePath); err != nil {
				log.Warn("restored ffprobe unavailable; trying automatic discovery", "err", err)
				findFFprobe("")
			}
			workers.Go("hardware detection", func() { transcoder.Detect(workers.Context()) })
			if ln != nil {
				select {
				case listeners <- ln:
				case <-workers.Context().Done():
					_ = ln.Close()
				}
			}
			// Files may have changed since the backup was made: rescan everything.
			librariesChanged(true)
			if listenErr != nil {
				return fmt.Errorf("couldn't open restored port %d; restart Arc after freeing the port: %w", next.Port, listenErr)
			}
			return nil
		},
		Settings: func() settings.Settings {
			mu.Lock()
			defer mu.Unlock()
			return current
		},
		ApplySettings: applySettings,
		UploadsDir:    filepath.Join(dataDir, "uploads"),
		Logs:          logs,
		Revision:      revision,
		Transcoder:    transcoder,
		Trickplay:     thumbs,
		OpenSubtitles: subsOnline,
		Subtitles:     subtitleCache,
		FFprobeStatus: func() any {
			ffMu.Lock()
			defer ffMu.Unlock()
			return ffStatus
		},
	})

	handler = api.TrackRequests(handler, func() (func(), bool) { return workers.Begin("HTTP requests") })

	ln, err := net.Listen("tcp", cfg.ListenAddress())
	if err != nil {
		return fmt.Errorf("can't listen on %s (is Arc already running?): %w", cfg.ListenAddress(), err)
	}
	for {
		// Event and room streams end as soon as shutdown starts, so an open tab doesn't hold the
		// restart (or a Docker stop) for the whole timeout.
		closing := make(chan struct{})
		srv := &http.Server{Handler: secureRemote(handler, remoteAccess.Certs), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: 2 * time.Minute,
			BaseContext: func(net.Listener) context.Context { return api.ShutdownContextWithParent(ctx, closing) }}
		srv.RegisterOnShutdown(func() { close(closing) })
		served := make(chan error, 1)
		servingListener := ln
		if !workers.Go("HTTP listener", func() { served <- srv.Serve(remote.Listener(servingListener, remoteAccess.Certs.Config)) }) {
			_ = servingListener.Close()
			return nil
		}
		log.Info("Arc Media Server started", "version", version, "revision", revision, "address", displayAddress(ln.Addr()), "data", dataDir)

		var next net.Listener
		var serveErr error
		select {
		case <-ctx.Done():
		case next = <-listeners:
		case err := <-served:
			if !errors.Is(err, http.ErrServerClosed) {
				serveErr = err
			}
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Warn("HTTP shutdown timed out; closing active connections")
			_ = srv.Close()
		}
		cancel()
		if serveErr != nil {
			return serveErr
		}
		if next == nil {
			break
		}
		ln = next
	}
	workers.Close()
	log.Info("stopped")
	return nil
}

// resetAdminPassword gives the admin profile a new random password and prints it, signing out
// everywhere that used the old one. It's run from the command line on the server itself.
func resetAdminPassword(dataDir string) error {
	if dataDir == "" {
		dataDir = os.Getenv("AMS_DATA")
	}
	if dataDir == "" {
		dataDir = "data"
	}
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(dataDir, "ams.db"), store.Options{})
	if err != nil {
		return err
	}
	defer st.Close()
	admin, err := st.AdminUser(ctx)
	if err != nil {
		return err
	}
	if admin == nil {
		return errors.New("Arc hasn't been set up yet; open it in a browser to create the admin profile")
	}
	token, _ := auth.NewToken()
	password := token[:16]
	hash, err := auth.Hash(password)
	if err != nil {
		return err
	}
	if err := st.UpdateUser(ctx, admin.ID, store.UserUpdate{PasswordHash: &hash}); err != nil {
		return err
	}
	if err := st.DeletePasswordSessions(ctx, ""); err != nil {
		return err
	}
	fmt.Printf("The password for %s is now: %s\nSign in with it, then change it in Settings.\n", admin.Name, password)
	return nil
}

// displayAddress is a URL to open Arc at.
func displayAddress(a net.Addr) string {
	host, port, _ := net.SplitHostPort(a.String())
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "localhost"
	}
	return "http://" + net.JoinHostPort(host, port)
}

// importLegacyConfig copies what an old ams.yaml held (API keys, ffprobe, address, libraries) into
// the database, once. Since then everything is set in the web app and the file is ignored.
func importLegacyConfig(ctx context.Context, cfg *config.Config, path string, st *store.Store, log *slog.Logger) error {
	const key = "config_settings_imported"
	if done, err := st.Setting(ctx, key); err != nil || done == "1" {
		return err
	}
	s, err := settings.Load(ctx, st)
	if err != nil {
		return err
	}
	if cfg.TMDBAPIKey != "" {
		s.TMDBKey = cfg.TMDBAPIKey
	}
	if cfg.TVDBAPIKey != "" {
		s.TVDBKey = cfg.TVDBAPIKey
	}
	if cfg.FFprobe != "" && cfg.FFprobe != "ffprobe" {
		s.FFprobePath = cfg.FFprobe
	}
	if host, port, err := net.SplitHostPort(cfg.Listen); err == nil {
		if p, err := strconv.Atoi(port); err == nil {
			s.Port = p
		}
		ip := net.ParseIP(host)
		s.AllowNetwork = host != "localhost" && (ip == nil || !ip.IsLoopback())
	}
	if err := settings.Save(ctx, st, s); err != nil {
		return err
	}
	// Libraries were imported separately by an earlier version; don't add them twice.
	if done, err := st.Setting(ctx, "config_libraries_imported"); err != nil {
		return err
	} else if done != "1" {
		for _, lib := range cfg.Libraries {
			if _, err := st.SyncLibrary(ctx, lib.Name, lib.Kind, lib.SourceOrdering, lib.Paths); err != nil {
				return fmt.Errorf("library %q: %w", lib.Name, err)
			}
		}
		if err := st.SetSetting(ctx, "config_libraries_imported", "1"); err != nil {
			return err
		}
	}
	log.Info("imported settings from the old config file; everything is set in the web app from now on, and the file can be deleted", "file", path)
	return st.SetSetting(ctx, key, "1")
}

// secureRemote sends people away from home to HTTPS while it's on (as Plex's "Secure connections:
// Preferred" does); the home network can still use plain HTTP.
func secureRemote(h http.Handler, certs *remote.Certs) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsHTTPS(r) && !auth.IsLocal(r) && certs.Enabled() {
			http.Redirect(w, r, "https://"+r.Host+r.URL.RequestURI(), http.StatusTemporaryRedirect)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// publicURL is the address other devices can open this server at ("" if it can't be worked out).
func publicURL(c settings.Settings) string {
	host, port, err := net.SplitHostPort(c.ListenAddress())
	if err != nil {
		return ""
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if ip, ok := a.(*net.IPNet); ok && ip.IP.To4() != nil && !ip.IP.IsLoopback() && ip.IP.IsPrivate() {
					host = ip.IP.String()
					break
				}
			}
		}
	}
	return "http://" + net.JoinHostPort(host, port)
}

// applyHomeNetworks sets the extra networks that count as home and the trusted reverse proxies
// (saved settings were checked).
func applyHomeNetworks(home, proxies string) {
	nets, _, _ := auth.ParseNetworks(home)
	auth.SetHomeNetworks(nets)
	trusted, _, _ := auth.ParseNetworks(proxies)
	auth.SetTrustedProxies(trusted)
}

// listenPort is the port Arc listens on (AMS_LISTEN can differ from the setting).
func listenPort(c settings.Settings) int {
	if _, port, err := net.SplitHostPort(c.ListenAddress()); err == nil {
		if p, err := strconv.Atoi(port); err == nil {
			return p
		}
	}
	return c.Port
}
