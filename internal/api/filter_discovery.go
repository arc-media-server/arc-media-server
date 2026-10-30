package api

import (
	"ams/internal/store"
	"fmt"
	"net/http"
	"regexp"
	"slices"
)

var codecFilterPattern = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,31}$`)
var dynamicRanges = []string{"sdr", "hdr10", "hlg", "hdr10_plus", "dolby_vision", "unknown"}

func (f rowFilter) validateExtended() error {
	for _, codec := range []string{f.VideoCodec, f.AudioCodec} {
		if codec != "" && !codecFilterPattern.MatchString(codec) {
			return fmt.Errorf("codec must be a 1–32 character codec identifier")
		}
	}
	if f.AudioChannels < 0 || f.AudioChannels > 32 {
		return fmt.Errorf("audio_channels must be 1–32, or absent")
	}
	if f.DynamicRange != "" && !slices.Contains(dynamicRanges, f.DynamicRange) {
		return fmt.Errorf("unknown dynamic_range")
	}
	if !slices.Contains([]string{"", "unwatched", "played", "unstarted"}, f.WatchState) {
		return fmt.Errorf("unknown watch_state")
	}
	if f.Unwatched && f.WatchState != "" && f.WatchState != "unwatched" {
		return fmt.Errorf("conflicting watch filters")
	}
	if f.PersonID < 0 || !slices.Contains([]string{"", "actor", "director"}, f.PersonRole) || f.PersonRole != "" && f.PersonID == 0 {
		return fmt.Errorf("person_role requires positive person_id and actor or director")
	}
	return nil
}
func (f rowFilter) applyExtended(q *store.ItemQuery) {
	q.VideoCodec, q.AudioCodec, q.AudioChannels, q.DynamicRange = f.VideoCodec, f.AudioCodec, f.AudioChannels, f.DynamicRange
	q.PersonID, q.PersonRole = f.PersonID, f.PersonRole
	q.Played = f.WatchState == "played"
	q.Unstarted = f.WatchState == "unstarted"
	q.Unwatched = q.Unwatched || f.WatchState == "unwatched"
}

type filterDescriptor struct {
	Key      string   `json:"key"`
	Type     string   `json:"type"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
	Minimum  int      `json:"minimum,omitempty"`
	Maximum  int      `json:"maximum,omitempty"`
	// For original facets; no claim that static enum choices are available in this library.
	Facet string `json:"facet,omitempty"`
}
type filterDiscoveryResponse struct {
	Filters   []filterDescriptor `json:"filters"`
	Sorts     []string           `json:"sorts"`
	FacetsURL string             `json:"facets_url"`
}

// filterDiscovery: GET /api/libraries/{id}/filters describes supported browsing and smart-row
// filter keys/types/operators. Enum values are supported choices, not availability counts.
// Fetch facets_url for access-controlled genre/language/label/decade/resolution availability.
// Media filters are source properties; audio/video constraints match one present file and one
// stream of each kind, including accessible child episodes for a series. Dynamic range precedence
// is Dolby Vision, HDR10+, HLG, HDR10, known SDR, unknown; this does not describe output capability.
func (s *Server) filterDiscovery(w http.ResponseWriter, r *http.Request) {
	lib, ok := s.libraryFromPath(w, r)
	if !ok {
		return
	}
	filters := []filterDescriptor{
		{Key: "genre", Type: "string", Operator: "eq", Facet: "genres"}, {Key: "language", Type: "string", Operator: "eq", Facet: "languages"}, {Key: "label", Type: "string", Operator: "eq", Facet: "labels"}, {Key: "decade", Type: "integer", Operator: "eq", Facet: "decades"},
		{Key: "resolution", Type: "string", Operator: "eq", Values: []string{"4k", "1080", "720", "sd"}, Facet: "resolutions"},
		{Key: "video_codec", Type: "codec", Operator: "eq"}, {Key: "audio_codec", Type: "codec", Operator: "eq"}, {Key: "audio_channels", Type: "integer", Operator: "eq", Minimum: 1, Maximum: 32},
		{Key: "dynamic_range", Type: "string", Operator: "eq", Values: dynamicRanges}, {Key: "watch_state", Type: "string", Operator: "eq", Values: []string{"unwatched", "played", "unstarted"}},
		{Key: "person_id", Type: "integer", Operator: "eq", Minimum: 1}, {Key: "person_role", Type: "string", Operator: "eq", Values: []string{"actor", "director"}},
		{Key: "unwatched", Type: "boolean", Operator: "eq"}, {Key: "unmatched", Type: "boolean", Operator: "eq"},
		{Key: "added_within", Type: "integer", Operator: "days", Minimum: 1}, {Key: "min_runtime", Type: "integer", Operator: "gte", Minimum: 1}, {Key: "max_runtime", Type: "integer", Operator: "lte", Minimum: 1}, {Key: "min_rating", Type: "number", Operator: "gte", Maximum: 10},
		{Key: "about", Type: "string", Operator: "contains_any"},
	}
	sorts := make([]string, 0, len(store.ItemSorts))
	for sort := range store.ItemSorts {
		sorts = append(sorts, sort)
	}
	slices.Sort(sorts)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, 200, filterDiscoveryResponse{Filters: filters, Sorts: sorts, FacetsURL: fmt.Sprintf("/api/v1/libraries/%d/facets", lib.ID)})
}
