// Package settings holds the server's settings. They live in the database and are changed in the
// web app; there is no config file. Environment variables can override some of them (useful in
// containers), in which case the app shows them as fixed.
package settings

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"ams/internal/store"
)

// Settings are the server-wide settings.
type Settings struct {
	TMDBKey string
	TVDBKey string
	// Full path to ffprobe; "" means find it automatically.
	FFprobePath string
	// Profiles can be picked without the password from this computer and the home network.
	LocalBypass bool
	Port        int
	// Accept connections from other devices (otherwise only this computer).
	AllowNetwork bool
	// Country (ISO 3166-1, e.g. "GB") whose age ratings are shown and used by parental controls.
	Region string
	// Convert video with the graphics card when there's one that can (on unless turned off).
	HWAccel bool
	// Map HDR to SDR when converting HDR video (on unless turned off).
	ToneMap bool
}

// DefaultRegion is used until a region is chosen.
const DefaultRegion = "US"

const DefaultPort = 8099

// Database keys.
const (
	keyTMDB         = "tmdb_api_key"
	keyTVDB         = "tvdb_api_key"
	keyFFprobe      = "ffprobe_path"
	keyLocalBypass  = "local_bypass"
	keyPort         = "port"
	keyAllowNetwork = "allow_network"
	keyRegion       = "region"
	keyHWAccel      = "hw_accel"
	keyToneMap      = "tone_mapping"
)

// Environment variables that override settings.
var envOverrides = map[string]string{keyTMDB: "AMS_TMDB_API_KEY", keyTVDB: "AMS_TVDB_API_KEY", keyFFprobe: "AMS_FFPROBE"}

// FromEnv reports which settings (by JSON name) are fixed by environment variables.
func FromEnv() map[string]bool {
	out := map[string]bool{}
	for key, env := range envOverrides {
		if os.Getenv(env) != "" {
			out[key] = true
		}
	}
	return out
}

// Load reads the settings, applying environment overrides and defaults.
func Load(ctx context.Context, st *store.Store) (Settings, error) {
	get := func(key string) (string, error) {
		if env := envOverrides[key]; env != "" {
			if v := os.Getenv(env); v != "" {
				return v, nil
			}
		}
		return st.Setting(ctx, key)
	}
	var s Settings
	var err error
	if s.TMDBKey, err = get(keyTMDB); err != nil {
		return s, err
	}
	if s.TVDBKey, err = get(keyTVDB); err != nil {
		return s, err
	}
	if s.FFprobePath, err = get(keyFFprobe); err != nil {
		return s, err
	}
	v, err := get(keyLocalBypass)
	if err != nil {
		return s, err
	}
	s.LocalBypass = v == "1"
	if v, err = get(keyAllowNetwork); err != nil {
		return s, err
	}
	s.AllowNetwork = v == "1"
	if v, err = get(keyPort); err != nil {
		return s, err
	}
	s.Port = DefaultPort
	if p, err := strconv.Atoi(v); err == nil && p > 0 && p < 65536 {
		s.Port = p
	}
	if s.Region, err = get(keyRegion); err != nil {
		return s, err
	}
	if s.Region == "" {
		s.Region = DefaultRegion
	}
	if v, err = get(keyHWAccel); err != nil {
		return s, err
	}
	s.HWAccel = v != "0"
	if v, err = get(keyToneMap); err != nil {
		return s, err
	}
	s.ToneMap = v != "0"
	return s, nil
}

// Save stores the settings.
func Save(ctx context.Context, st *store.Store, s Settings) error {
	for key, v := range map[string]string{
		keyTMDB: s.TMDBKey, keyTVDB: s.TVDBKey, keyFFprobe: s.FFprobePath,
		keyLocalBypass: boolString(s.LocalBypass), keyAllowNetwork: boolString(s.AllowNetwork),
		keyPort: strconv.Itoa(s.Port), keyRegion: s.Region,
		keyHWAccel: boolString(s.HWAccel), keyToneMap: boolString(s.ToneMap),
	} {
		if os.Getenv(envOverrides[key]) != "" && envOverrides[key] != "" {
			continue // fixed by the environment
		}
		if err := st.SetSetting(ctx, key, v); err != nil {
			return err
		}
	}
	return nil
}

// ListenAddress is where the server listens. AMS_LISTEN overrides it.
func (s Settings) ListenAddress() string {
	if v := os.Getenv("AMS_LISTEN"); v != "" {
		return v
	}
	host := "127.0.0.1"
	if s.AllowNetwork {
		host = ""
	}
	return fmt.Sprintf("%s:%d", host, s.Port)
}

func boolString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
