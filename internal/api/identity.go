package api

import (
	"net/http"
	"strings"
)

func serverDisplayName(name string) string {
	if name = strings.TrimSpace(name); name != "" {
		return name
	}
	return "Arc Media Server"
}

type serverIdentity struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	APIVersion int    `json:"api_version"`
}

// identity: GET /api/identity returns this server's persistent identity and friendly name.
// Requires a selected profile. Rename through PATCH /settings {"server_name":"Living room"}.
// A database restore preserves its identity; independently running cloned servers must not be
// treated as a single instance. IDs do not establish trust or provide remote discovery.
func (s *Server) identity(w http.ResponseWriter, r *http.Request) {
	id, err := s.store.Setting(r.Context(), "server_id")
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, 200, serverIdentity{ID: id, Name: serverDisplayName(s.settings().ServerName), Version: s.version, APIVersion: APIVersion})
}

type featureCapability struct {
	Supported bool `json:"supported"`
	Available bool `json:"available"`
	Allowed   bool `json:"allowed"`
}

type serverCapabilities struct {
	APIVersion int                          `json:"api_version"`
	Features   map[string]featureCapability `json:"features"`
}

// capabilities: GET /api/capabilities describes implemented API features, runtime availability
// and the selected profile's role permission. Item/library restrictions are still checked on
// every operation. Missing feature keys mean unsupported. Availability is a point-in-time hint,
// not a promise of provider/network/device compatibility; no credentials or paths are exposed.
func (s *Server) capabilities(w http.ResponseWriter, r *http.Request) {
	features := map[string]featureCapability{}
	add := func(key string, available, admin bool) {
		features[key] = featureCapability{Supported: true, Available: available, Allowed: !admin || currentUser(r).IsAdmin}
	}
	add("identity", true, false)
	add("browse", true, false)
	add("item_lookup", true, false)
	add("item_batch", true, false)
	add("playback_options", true, false)
	add("playback", true, false) // direct streams do not require a converter
	add("transcoding", s.transcoder != nil, false)
	add("playlists", true, false)
	add("play_queues", true, false)
	add("collections", true, false)
	add("artwork_palette", s.artwork != nil, false)
	add("subtitle_delivery", s.subtitles != nil, false)
	add("trickplay", s.trickplay != nil, false)
	add("activity", s.activity != nil, true)
	add("maintenance", s.background != nil, true)
	add("metadata_edit", true, true)
	add("unmatch_show", s.scanner != nil, true)
	add("artwork_upload", s.uploadsDir != "", true)
	add("library_scan", s.scanner != nil, true)
	writeJSON(w, 200, serverCapabilities{APIVersion: APIVersion, Features: features})
}
