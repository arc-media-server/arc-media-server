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
	loadedScanHours, loadedScanMinutes                          int
	ScanAutomatic, ScanPartial, ScanOnStart                     bool
	ScanIntervalMinutes                                         int
	AnalysisEnabled, MetadataRefreshEnabled, AutoBackupsEnabled bool
	EmptyTrashEnabled                                           bool
	DatabaseUpkeepEnabled, CacheCleanupEnabled                  bool
	PreviewMode, DetectionMode                                  string
	BackgroundEpoch                                             int64
	ScanIntervalHours                                           int // 0 disables recurring discovery

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
	// OpenSubtitles.com, for finding subtitles online: an API key, and optionally an account
	// (which allows more downloads a day).
	OpenSubtitlesKey, OpenSubtitlesUser, OpenSubtitlesPassword string
	// The maintenance window, in hours (02:00–05:00 unless changed): metadata refreshes and
	// backups wait for it.
	MaintenanceStart, MaintenanceEnd int
	// Make scrubbing previews only in the maintenance window (otherwise as soon as files arrive).
	PreviewsAtNight bool
	// OMDb, for Rotten Tomatoes' and Metacritic's scores ("" for none).
	OMDbKey string
	// Pre-roll videos before movies: files or folders, one per line; "random" plays one of them,
	// "all" every one in order.
	PrerollPaths, PrerollMode string
	// TheIntroDB, for intro and credits timings ("" to use IntroDB alone), and whether to find them
	// on this computer when there are none online (on unless turned off).
	TheIntroDBKey string
	DetectIntros  bool
	// Streams to people away from home: the most each may use (kbps, 0 for no limit), and the
	// home's internet upload speed (kbps, 0 if not set), shared between them.
	RemoteKbps, UploadKbps int
	// Remote access: reachable from the internet (listening on the network), at an outside port
	// (0 for the same as Port), opened on the router with UPnP (on unless turned off).
	RemoteAccess bool
	RemotePort   int
	UPnP         bool
	// HTTPS on the same port: "" off, "files" (HTTPSCert and HTTPSKey) or "letsencrypt" (HTTPSDomain).
	HTTPSMode, HTTPSCert, HTTPSKey, HTTPSDomain string
	// Networks that count as home besides private addresses, e.g. "100.64.0.0/10" (comma-separated).
	HomeNetworks string
	// Where converted streams are written while they're watched ("" for Arc's data folder), and
	// how many videos may be converted at once (0 for no limit).
	TranscodeDir  string
	MaxTranscodes int
}

// DefaultRegion is used until a region is chosen.
const DefaultRegion = "US"

const DefaultPort = 8099

// Database keys.
const (
	keyScanAutomatic = "scan_automatic"
	keyScanPartial   = "scan_partial"
	keyScanOnStart   = "scan_on_start"
	keyScanInterval  = "scan_interval_hours"
	keyTMDB          = "tmdb_api_key"
	keyTVDB          = "tvdb_api_key"
	keyFFprobe       = "ffprobe_path"
	keyLocalBypass   = "local_bypass"
	keyPort          = "port"
	keyAllowNetwork  = "allow_network"
	keyRegion        = "region"
	keyHWAccel       = "hw_accel"
	keyToneMap       = "tone_mapping"
	keyOSKey         = "opensubtitles_api_key"
	keyOSUser        = "opensubtitles_username"
	keyOSPassword    = "opensubtitles_password"
	keyMaintStart    = "maintenance_start"
	keyMaintEnd      = "maintenance_end"
	keyPreviewsNight = "previews_at_night"
	keyOMDb          = "omdb_api_key"
	keyPrerollPaths  = "preroll_paths"
	keyPrerollMode   = "preroll_mode"
	keyTheIntroDB    = "theintrodb_api_key"
	keyDetectIntros  = "detect_intros"
	keyRemoteKbps    = "remote_kbps"
	keyUploadKbps    = "upload_kbps"
	keyRemoteAccess  = "remote_access"
	keyRemotePort    = "remote_port"
	keyUPnP          = "upnp"
	keyHTTPSMode     = "https_mode"
	keyHTTPSCert     = "https_cert"
	keyHTTPSKey      = "https_key"
	keyHTTPSDomain   = "https_domain"
	keyHomeNetworks  = "home_networks"
	keyTranscodeDir  = "transcode_dir"
	keyMaxTranscodes = "max_transcodes"
)

// Environment variables that override settings.
var envOverrides = map[string]string{keyTMDB: "AMS_TMDB_API_KEY", keyTVDB: "AMS_TVDB_API_KEY", keyFFprobe: "AMS_FFPROBE",
	keyOSKey: "AMS_OPENSUBTITLES_API_KEY", keyOMDb: "AMS_OMDB_API_KEY"}

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
	if s.OpenSubtitlesKey, err = get(keyOSKey); err != nil {
		return s, err
	}
	if s.OpenSubtitlesUser, err = get(keyOSUser); err != nil {
		return s, err
	}
	if s.OpenSubtitlesPassword, err = get(keyOSPassword); err != nil {
		return s, err
	}
	s.MaintenanceStart, s.MaintenanceEnd = 2, 5
	if v, err = get(keyMaintStart); err != nil {
		return s, err
	}
	if h, err := strconv.Atoi(v); err == nil && h >= 0 && h < 24 {
		s.MaintenanceStart = h
	}
	if v, err = get(keyMaintEnd); err != nil {
		return s, err
	}
	if h, err := strconv.Atoi(v); err == nil && h >= 0 && h < 24 {
		s.MaintenanceEnd = h
	}
	if v, err = get(keyPreviewsNight); err != nil {
		return s, err
	}
	s.PreviewsAtNight = v == "1"
	if s.OMDbKey, err = get(keyOMDb); err != nil {
		return s, err
	}
	if s.PrerollPaths, err = get(keyPrerollPaths); err != nil {
		return s, err
	}
	if s.PrerollMode, err = get(keyPrerollMode); err != nil {
		return s, err
	}
	if s.PrerollMode != "all" {
		s.PrerollMode = "random"
	}
	if s.TheIntroDBKey, err = get(keyTheIntroDB); err != nil {
		return s, err
	}
	if v, err = get(keyDetectIntros); err != nil {
		return s, err
	}
	s.DetectIntros = v != "0"
	if v, err = get(keyRemoteKbps); err != nil {
		return s, err
	}
	s.RemoteKbps, _ = strconv.Atoi(v)
	if v, err = get(keyUploadKbps); err != nil {
		return s, err
	}
	s.UploadKbps, _ = strconv.Atoi(v)
	if v, err = get(keyRemoteAccess); err != nil {
		return s, err
	}
	s.RemoteAccess = v == "1"
	if v, err = get(keyRemotePort); err != nil {
		return s, err
	}
	if p, err := strconv.Atoi(v); err == nil && p > 0 && p < 65536 {
		s.RemotePort = p
	}
	if v, err = get(keyUPnP); err != nil {
		return s, err
	}
	s.UPnP = v != "0"
	if s.HTTPSMode, err = get(keyHTTPSMode); err != nil {
		return s, err
	}
	if s.HTTPSCert, err = get(keyHTTPSCert); err != nil {
		return s, err
	}
	if s.HTTPSKey, err = get(keyHTTPSKey); err != nil {
		return s, err
	}
	if s.HTTPSDomain, err = get(keyHTTPSDomain); err != nil {
		return s, err
	}
	if s.HomeNetworks, err = get(keyHomeNetworks); err != nil {
		return s, err
	}
	if s.TranscodeDir, err = get(keyTranscodeDir); err != nil {
		return s, err
	}
	if v, err = get(keyMaxTranscodes); err != nil {
		return s, err
	}
	s.MaxTranscodes, _ = strconv.Atoi(v)
	for key, dest := range map[string]*bool{keyScanAutomatic: &s.ScanAutomatic, keyScanPartial: &s.ScanPartial, keyScanOnStart: &s.ScanOnStart} {
		v, err := get(key)
		if err != nil {
			return s, err
		}
		*dest = v != "0"
	}
	s.ScanIntervalHours = 1
	if v, err := get(keyScanInterval); err != nil {
		return s, err
	} else if h, err := strconv.Atoi(v); err == nil && ValidScanInterval(h) {
		s.ScanIntervalHours = h
	}

	for key, dest := range map[string]*bool{"analysis_enabled": &s.AnalysisEnabled, "metadata_refresh_enabled": &s.MetadataRefreshEnabled, "auto_backups_enabled": &s.AutoBackupsEnabled, "database_upkeep_enabled": &s.DatabaseUpkeepEnabled, "cache_cleanup_enabled": &s.CacheCleanupEnabled} {
		v, err := get(key)
		if err != nil {
			return s, err
		}
		*dest = v != "0"
	}
	if v, err := get("empty_trash_enabled"); err != nil {
		return s, err
	} else {
		s.EmptyTrashEnabled = v == "1"
	}
	s.PreviewMode = "added"
	if s.PreviewsAtNight {
		s.PreviewMode = "scheduled"
	}
	s.DetectionMode = "scheduled"
	if !s.DetectIntros {
		s.DetectionMode = "off"
	}
	for key, dest := range map[string]*string{"preview_mode": &s.PreviewMode, "detection_mode": &s.DetectionMode} {
		v, err := get(key)
		if err != nil {
			return s, err
		}
		if ValidProcessingMode(v) {
			*dest = v
		}
	}
	v, err = get("background_epoch")
	if err != nil {
		return s, err
	}
	s.BackgroundEpoch, _ = strconv.ParseInt(v, 10, 64)
	s.ScanIntervalMinutes = s.ScanIntervalHours * 60
	v, err = get("scan_interval_minutes")
	if err != nil {
		return s, err
	}
	if n, e := strconv.Atoi(v); e == nil && ValidScanMinutes(n) {
		s.ScanIntervalMinutes = n
		s.ScanIntervalHours = n / 60
	}
	s.loadedScanHours, s.loadedScanMinutes = s.ScanIntervalHours, s.ScanIntervalMinutes
	return s, nil
}

func (s Settings) ScanMinutes() int {
	if s.ScanIntervalHours != s.loadedScanHours && s.ScanIntervalMinutes == s.loadedScanMinutes {
		return s.ScanIntervalHours * 60
	}
	return s.ScanIntervalMinutes
}
func ValidProcessingMode(v string) bool { return v == "off" || v == "scheduled" || v == "added" }
func ValidScanMinutes(n int) bool {
	return n == 0 || n == 15 || n == 30 || n == 60 || n == 120 || n == 360 || n == 720 || n == 1440
}

// ValidScanInterval reports the supported full-discovery intervals (hours).
func ValidScanInterval(h int) bool { return h == 0 || h == 1 || h == 2 || h == 6 || h == 12 || h == 24 }

// Save stores the settings.
func Save(ctx context.Context, st *store.Store, s Settings) error {
	for key, v := range map[string]string{
		keyScanAutomatic: boolString(s.ScanAutomatic), keyScanPartial: boolString(s.ScanPartial), keyScanOnStart: boolString(s.ScanOnStart), keyScanInterval: strconv.Itoa(s.ScanMinutes() / 60),
		"scan_interval_minutes": strconv.Itoa(s.ScanMinutes()), "analysis_enabled": boolString(s.AnalysisEnabled), "metadata_refresh_enabled": boolString(s.MetadataRefreshEnabled), "auto_backups_enabled": boolString(s.AutoBackupsEnabled), "empty_trash_enabled": boolString(s.EmptyTrashEnabled), "database_upkeep_enabled": boolString(s.DatabaseUpkeepEnabled), "cache_cleanup_enabled": boolString(s.CacheCleanupEnabled), "preview_mode": s.PreviewMode, "detection_mode": s.DetectionMode,
		keyTMDB: s.TMDBKey, keyTVDB: s.TVDBKey, keyFFprobe: s.FFprobePath,
		keyLocalBypass: boolString(s.LocalBypass), keyAllowNetwork: boolString(s.AllowNetwork),
		keyPort: strconv.Itoa(s.Port), keyRegion: s.Region,
		keyHWAccel: boolString(s.HWAccel), keyToneMap: boolString(s.ToneMap),
		keyOSKey: s.OpenSubtitlesKey, keyOSUser: s.OpenSubtitlesUser, keyOSPassword: s.OpenSubtitlesPassword,
		keyMaintStart: strconv.Itoa(s.MaintenanceStart), keyMaintEnd: strconv.Itoa(s.MaintenanceEnd),
		keyPreviewsNight: boolString(s.PreviewsAtNight), keyOMDb: s.OMDbKey,
		keyPrerollPaths: s.PrerollPaths, keyPrerollMode: s.PrerollMode,
		keyTheIntroDB: s.TheIntroDBKey, keyDetectIntros: boolString(s.DetectIntros),
		keyRemoteKbps: strconv.Itoa(s.RemoteKbps), keyUploadKbps: strconv.Itoa(s.UploadKbps),
		keyRemoteAccess: boolString(s.RemoteAccess), keyRemotePort: strconv.Itoa(s.RemotePort), keyUPnP: boolString(s.UPnP),
		keyHTTPSMode: s.HTTPSMode, keyHTTPSCert: s.HTTPSCert, keyHTTPSKey: s.HTTPSKey, keyHTTPSDomain: s.HTTPSDomain,
		keyHomeNetworks: s.HomeNetworks, keyTranscodeDir: s.TranscodeDir, keyMaxTranscodes: strconv.Itoa(s.MaxTranscodes),
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
	if s.AllowNetwork || s.RemoteAccess {
		host = ""
	}
	return fmt.Sprintf("%s:%d", host, s.Port)
}

// OutsidePort is the port people away from home connect to.
func (s Settings) OutsidePort() int {
	if s.RemotePort > 0 {
		return s.RemotePort
	}
	return s.Port
}

func boolString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
