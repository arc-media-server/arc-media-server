// Command ams runs Arc Media Server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"ams/internal/api"
	"ams/internal/artwork"
	"ams/internal/backup"
	"ams/internal/config"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
	"ams/internal/watcher"
)

const version = "0.1.0-dev"

func main() {
	configPath := flag.String("config", "ams.yaml", "path to the config file")
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	if err := run(*configPath, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(configPath string, log *slog.Logger) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	backupDir, err := filepath.Abs(filepath.Join(cfg.DataDir, "backups"))
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, filepath.Join(cfg.DataDir, "ams.db"), store.Options{
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
	backups := backup.New(backupDir, st, log)
	go backups.Run(ctx)

	prober := probe.New(cfg.FFprobe)
	if v, err := prober.Version(ctx); err != nil {
		return fmt.Errorf("ffprobe not usable (set 'ffprobe' in the config to its full path): %w", err)
	} else {
		log.Info("found ffprobe", "version", v)
	}

	var tmdb *metadata.TMDB
	if cfg.TMDBAPIKey != "" {
		tmdb = metadata.NewTMDB(cfg.TMDBAPIKey)
	} else {
		log.Warn("no TMDB API key configured; items will be added without metadata")
	}

	var tvdb *metadata.TVDB
	if cfg.TVDBAPIKey != "" {
		tvdb = metadata.NewTVDB(cfg.TVDBAPIKey)
	} else {
		log.Warn("no TVDB API key configured; TV shows will use TMDB only")
	}

	art := artwork.New(filepath.Join(cfg.DataDir, "artwork"), log)
	artKeeper := art.Keep(ctx, func(ctx context.Context) ([]string, error) {
		images, err := st.ItemImages(ctx)
		if err != nil {
			return nil, err
		}
		sources := make([]string, len(images))
		for i, im := range images {
			sources[i] = artwork.Source(im.Kind, im.Type, im.Path)
		}
		return sources, nil
	})

	sc := scanner.New(ctx, st, prober, tmdb, tvdb, log)
	// After each scan, download new artwork so it's there offline.
	sc.OnFinish = func(int64) { artKeeper.Trigger() }
	artKeeper.Trigger()

	if err := importConfigLibraries(ctx, cfg, st, log); err != nil {
		return err
	}

	// Scan libraries when their folders change.
	watch := watcher.New(sc.Request, log)
	go watch.Run(ctx)
	// librariesChanged brings the watcher in step with the libraries, optionally scanning them all.
	librariesChanged := func(scan bool) {
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

	srv := &http.Server{
		Addr: cfg.Listen,
		Handler: api.New(api.Deps{
			Store: st, Scanner: sc, Artwork: art, Backups: backups, Log: log, Version: version,
			LibrariesChanged: func() { librariesChanged(false) },
			// Files may have been added or removed since the backup was made: rescan everything.
			AfterRestore: func() { librariesChanged(true) },
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	log.Info("Arc Media Server started", "version", version, "listen", "http://"+cfg.Listen)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	sc.Wait()
	log.Info("stopped")
	return nil
}

// importConfigLibraries adds the libraries listed in ams.yaml to the database, once. Since then
// libraries are managed in the app, and the config's list is ignored.
func importConfigLibraries(ctx context.Context, cfg *config.Config, st *store.Store, log *slog.Logger) error {
	const key = "config_libraries_imported"
	done, err := st.Setting(ctx, key)
	if err != nil {
		return err
	}
	if done == "1" {
		if len(cfg.Libraries) > 0 {
			log.Info("libraries are managed in the app now; the libraries section of the config file is ignored")
		}
		return nil
	}
	for _, lib := range cfg.Libraries {
		if _, err := st.SyncLibrary(ctx, lib.Name, lib.Kind, lib.SourceOrdering, lib.Paths); err != nil {
			return fmt.Errorf("library %q: %w", lib.Name, err)
		}
		log.Info("imported library from the config file", "library", lib.Name)
	}
	return st.SetSetting(ctx, key, "1")
}
