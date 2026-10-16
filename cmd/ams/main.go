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
	"ams/internal/config"
	"ams/internal/metadata"
	"ams/internal/probe"
	"ams/internal/scanner"
	"ams/internal/store"
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
	st, err := store.Open(ctx, filepath.Join(cfg.DataDir, "ams.db"))
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

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
	for _, lib := range cfg.Libraries {
		id, err := st.SyncLibrary(ctx, lib.Name, lib.Kind, lib.SourceOrdering, lib.Paths)
		if err != nil {
			return fmt.Errorf("library %q: %w", lib.Name, err)
		}
		if cfg.ScanOnStart {
			sc.Start(id)
		}
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           api.New(st, sc, art, log, version),
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
