// Package config loads the AMS YAML configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen      string    `yaml:"listen"`
	DataDir     string    `yaml:"data_dir"`
	FFprobe     string    `yaml:"ffprobe"`
	TMDBAPIKey  string    `yaml:"tmdb_api_key"`
	TVDBAPIKey  string    `yaml:"tvdb_api_key"`
	ScanOnStart bool      `yaml:"scan_on_start"`
	Libraries   []Library `yaml:"libraries"`
}

type Library struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"` // movies or shows
	// How SxxEyy in filenames is numbered (shows only): tvdb_aired, tvdb_dvd, tvdb_absolute, tmdb_aired.
	SourceOrdering string   `yaml:"source_ordering"`
	Paths          []string `yaml:"paths"`
}

var sourceOrderings = map[string]bool{"tvdb_aired": true, "tvdb_dvd": true, "tvdb_absolute": true, "tmdb_aired": true}

// ValidSourceOrdering reports whether s is a way TV file names can be numbered.
func ValidSourceOrdering(s string) bool { return sourceOrderings[s] }

// Load reads the config file. AMS_TMDB_API_KEY, AMS_TVDB_API_KEY and AMS_LISTEN override the file's values.
func Load(path string) (*Config, error) {
	cfg := &Config{
		Listen:      "127.0.0.1:8099",
		DataDir:     "data",
		FFprobe:     "ffprobe",
		ScanOnStart: true,
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w (copy ams.example.yaml to %s to get started)", err, path)
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if v := os.Getenv("AMS_TMDB_API_KEY"); v != "" {
		cfg.TMDBAPIKey = v
	}
	if v := os.Getenv("AMS_TVDB_API_KEY"); v != "" {
		cfg.TVDBAPIKey = v
	}
	if v := os.Getenv("AMS_LISTEN"); v != "" {
		cfg.Listen = v
	}

	names := map[string]bool{}
	for i := range cfg.Libraries {
		lib := &cfg.Libraries[i]
		lib.Name = strings.TrimSpace(lib.Name)
		if lib.Name == "" {
			return nil, fmt.Errorf("library %d: name is required", i+1)
		}
		if names[strings.ToLower(lib.Name)] {
			return nil, fmt.Errorf("library %q: duplicate name", lib.Name)
		}
		names[strings.ToLower(lib.Name)] = true
		if lib.Kind != "movies" && lib.Kind != "shows" {
			return nil, fmt.Errorf("library %q: kind must be movies or shows", lib.Name)
		}
		if lib.SourceOrdering == "" {
			lib.SourceOrdering = "tvdb_aired"
		}
		if !sourceOrderings[lib.SourceOrdering] {
			return nil, fmt.Errorf("library %q: unknown source_ordering %q", lib.Name, lib.SourceOrdering)
		}
		if len(lib.Paths) == 0 {
			return nil, fmt.Errorf("library %q: at least one path is required", lib.Name)
		}
		for j, p := range lib.Paths {
			abs, err := filepath.Abs(p)
			if err != nil {
				return nil, fmt.Errorf("library %q: %w", lib.Name, err)
			}
			lib.Paths[j] = abs
		}
	}
	return cfg, nil
}
