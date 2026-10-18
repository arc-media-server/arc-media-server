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
	"ams/internal/backup"
	"ams/internal/config"
	"ams/internal/logbuf"
	"ams/internal/metadata"
	"ams/internal/opensubtitles"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/schedule"
	"ams/internal/settings"
	"ams/internal/store"
	"ams/internal/subtitles"
	"ams/internal/transcode"
	"ams/internal/trickplay"
	"ams/internal/watcher"
)

// The most recent log lines, which the admin can read on the Dashboard.
var logs = logbuf.New(2000)

const version = "0.1.0-dev"

func main() {
	dataDir := flag.String("data", "", "folder for Arc's database, artwork cache and backups (default: data, or AMS_DATA)")
	legacyConfig := flag.String("config", "ams.yaml", "an old config file to import settings from, once")
	debug := flag.Bool("debug", false, "enable debug logging")
	resetPassword := flag.Bool("reset-password", false, "forgot the password? set a new random one, print it and exit")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	log := slog.New(logs.Wrap(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

	if *resetPassword {
		if err := resetAdminPassword(*dataDir); err != nil {
			log.Error("resetting the password failed", "err", err)
			os.Exit(1)
		}
		return
	}
	if err := run(*dataDir, *legacyConfig, log); err != nil {
		log.Error("fatal", "err", err)
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

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
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
	tasks := activity.New()
	// The maintenance window; its hours come from the settings below.
	window := schedule.New(2, 5)
	var previewsAtNight atomic.Bool
	backups := backup.New(backupDir, st, log)
	backups.Activity = tasks
	backups.Window = window
	go backups.Run(ctx)

	if legacy != nil {
		if err := importLegacyConfig(ctx, legacy, legacyConfig, st, log); err != nil {
			return err
		}
	}
	cfg, err := settings.Load(ctx, st)
	if err != nil {
		return err
	}
	window.Set(cfg.MaintenanceStart, cfg.MaintenanceEnd)
	previewsAtNight.Store(cfg.PreviewsAtNight)

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
	go transcoder.Run(ctx)
	go transcoder.Detect(ctx)

	tmdb, tvdb := metadata.NewTMDB(cfg.TMDBKey), metadata.NewTVDB(cfg.TVDBKey)
	subsOnline := opensubtitles.New("Arc Media Server v" + version)
	subsOnline.SetCredentials(cfg.OpenSubtitlesKey, cfg.OpenSubtitlesUser, cfg.OpenSubtitlesPassword)
	tmdb.SetRegion(cfg.Region)
	if cfg.TMDBKey == "" {
		log.Warn("no TMDB API key yet; add one in Settings so Arc can identify movies and shows")
	}

	art := artwork.New(filepath.Join(dataDir, "artwork"), log)
	artKeeper := art.Keep(ctx, func(ctx context.Context) ([]string, error) {
		images, err := st.ItemImages(ctx)
		if err != nil {
			return nil, err
		}
		sources := make([]string, 0, len(images))
		for _, im := range images {
			// Uploaded artwork is already on disk.
			if src := artwork.Source(im.Kind, im.Type, im.Path); strings.HasPrefix(src, "http") {
				sources = append(sources, src)
			}
		}
		return sources, nil
	})

	sc := scanner.New(ctx, st, prober, tmdb, tvdb, log)
	sc.Activity = tasks
	sc.Window = window
	sc.ThumbDir = filepath.Join(dataDir, "videothumbs")
	sc.FFmpeg = func() string {
		ffMu.Lock()
		defer ffMu.Unlock()
		return ffmpegBin
	}
	// After each scan, download new artwork so it's there offline.
	sc.OnFinish = func(int64) { artKeeper.Trigger() }
	artKeeper.Trigger()

	// Scan libraries when their folders change.
	watch := watcher.New(sc.Request, log)
	go watch.Run(ctx)
	// librariesChanged brings the watcher in step with the libraries, optionally scanning them all.
	// Timeline thumbnails, made in the background for libraries that have them turned on.
	thumbs := trickplay.New(filepath.Join(dataDir, "trickplay"), st, func() string {
		ffMu.Lock()
		defer ffMu.Unlock()
		return ffmpegBin
	}, log)
	thumbs.Activity = tasks
	thumbs.Allowed = func() bool { return !previewsAtNight.Load() || window.Open() }
	go thumbs.Run(ctx)
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
	librariesChanged(true)

	// Once a night, in the maintenance window: scan every library (which refreshes old details)
	// and catch up on scrubbing previews. Backups check the window themselves.
	go func() {
		last := ""
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
			night := window.Night(time.Now())
			if !window.Open() || night == last {
				continue
			}
			last = night
			task := tasks.Start("maintenance", "Nightly maintenance")
			libs, err := st.ListLibraries(ctx)
			for _, l := range libs {
				sc.Request(l.ID)
			}
			thumbs.Wake()
			task.Detail(fmt.Sprintf("Checked %d libraries for changes and old details", len(libs)))
			task.Finish(err)
		}
	}()

	// Sessions unused for 90 days are forgotten.
	go func() {
		for {
			st.DeleteStaleSessions(ctx, time.Now().Add(-90*24*time.Hour).Unix())
			select {
			case <-ctx.Done():
				return
			case <-time.After(24 * time.Hour):
			}
		}
	}()

	// The web server can move to a new address when the port or network setting changes.
	var mu sync.Mutex
	current := cfg
	listeners := make(chan net.Listener, 1)
	applySettings := func(ctx context.Context, next settings.Settings) error {
		mu.Lock()
		defer mu.Unlock()
		if next.TMDBKey != current.TMDBKey && next.TMDBKey != "" {
			if err := metadata.CheckTMDBKey(ctx, next.TMDBKey); err != nil {
				return err
			}
		}
		if next.OpenSubtitlesKey != "" && (next.OpenSubtitlesKey != current.OpenSubtitlesKey ||
			next.OpenSubtitlesUser != current.OpenSubtitlesUser || next.OpenSubtitlesPassword != current.OpenSubtitlesPassword) {
			check := opensubtitles.New("Arc Media Server v" + version)
			check.SetCredentials(next.OpenSubtitlesKey, next.OpenSubtitlesUser, next.OpenSubtitlesPassword)
			if err := check.Check(ctx); err != nil {
				return err
			}
		}
		if next.TVDBKey != current.TVDBKey && next.TVDBKey != "" {
			if err := metadata.CheckTVDBKey(ctx, next.TVDBKey); err != nil {
				return err
			}
		}
		if next.FFprobePath != current.FFprobePath {
			if err := findFFprobe(next.FFprobePath); err != nil && next.FFprobePath != "" {
				findFFprobe(current.FFprobePath)
				return fmt.Errorf("no working ffprobe at %s", next.FFprobePath)
			}
			// A different FFmpeg may support different hardware.
			go transcoder.Detect(context.WithoutCancel(ctx))
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
		tmdb.SetKey(next.TMDBKey)
		tvdb.SetKey(next.TVDBKey)
		subsOnline.SetCredentials(next.OpenSubtitlesKey, next.OpenSubtitlesUser, next.OpenSubtitlesPassword)
		window.Set(next.MaintenanceStart, next.MaintenanceEnd)
		if next.PreviewsAtNight != current.PreviewsAtNight {
			previewsAtNight.Store(next.PreviewsAtNight)
			thumbs.Wake()
		}
		if next.Region != current.Region {
			tmdb.SetRegion(next.Region)
			// Age ratings are per region: fetch everything again, a batch per scan.
			if err := st.MarkAllUnsynced(ctx); err != nil {
				return err
			}
		}
		current = next
		if ln != nil {
			listeners <- ln
		}
		return nil
	}

	handler := api.New(api.Deps{
		Store: st, Scanner: sc, Artwork: art, Backups: backups, Activity: tasks, Log: log, Version: version,
		LibrariesChanged: func() { librariesChanged(false) },
		// Files may have been added or removed since the backup was made: rescan everything.
		AfterRestore: func() { librariesChanged(true) },
		Settings: func() settings.Settings {
			mu.Lock()
			defer mu.Unlock()
			return current
		},
		ApplySettings: applySettings,
		UploadsDir:    filepath.Join(dataDir, "uploads"),
		Logs:          logs,
		Transcoder:    transcoder,
		Trickplay:     thumbs,
		OpenSubtitles: subsOnline,
		Subtitles: subtitles.NewCache(filepath.Join(dataDir, "subtitles"), func() string {
			ffMu.Lock()
			defer ffMu.Unlock()
			return ffmpegBin
		}),
		FFprobeStatus: func() any {
			ffMu.Lock()
			defer ffMu.Unlock()
			return ffStatus
		},
	})

	ln, err := net.Listen("tcp", cfg.ListenAddress())
	if err != nil {
		return fmt.Errorf("can't listen on %s (is Arc already running?): %w", cfg.ListenAddress(), err)
	}
	for {
		srv := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
		served := make(chan error, 1)
		go func(ln net.Listener) { served <- srv.Serve(ln) }(ln)
		log.Info("Arc Media Server started", "version", version, "address", displayAddress(ln.Addr()), "data", dataDir)

		var next net.Listener
		select {
		case <-ctx.Done():
		case next = <-listeners:
		case err := <-served:
			if !errors.Is(err, http.ErrServerClosed) {
				return err
			}
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		srv.Shutdown(shutdownCtx)
		cancel()
		if next == nil {
			break
		}
		ln = next
	}
	sc.Wait()
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
