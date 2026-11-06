package apidoc

import "fmt"

// Metadata describes logical values, not a change to Go's existing query parsing. Flags stay
// strings where handlers require "1"; SDKs must not silently serialize those as true.
func queryParameter(handler, name string) (map[string]any, error) {
	schema := map[string]any{"type": "string"}
	description := ""
	required := false
	integer := func(desc string) { schema = map[string]any{"type": "integer", "format": "int64"}; description = desc }
	enumeration := func(desc string, values ...string) { description = desc; schema["enum"] = values }
	flag := func(desc string, words bool) {
		enumeration(desc, "0", "1")
		if words {
			schema["enum"] = []string{"0", "1", "false", "true"}
		}
		schema["default"] = "0"
		schema["x-arc-logical-type"] = "boolean"
	}
	switch name {
	case "limit":
		bounds := map[string][2]int{"listItems": {100, 500}, "lookupItems": {50, 100}, "recommendedPage": {50, 100}, "recentlyAddedShows": {24, 100}, "statsHistory": {50, 200}}
		b, ok := bounds[handler]
		if !ok {
			return nil, fmt.Errorf("document %s limit bounds", handler)
		}
		integer("Page size. Invalid values reset to the default on legacy browse/history/recent endpoints; lookup/recommendation endpoints reject them with 400.")
		schema["minimum"] = 1
		schema["maximum"] = b[1]
		schema["default"] = b[0]
	case "client_sent_ms":
		integer("The client's own send time in epoch milliseconds, echoed back for clock-offset and round-trip estimates (Watch Together). Optional; must be nonnegative.")
		schema["minimum"] = 0
	case "offset":
		integer("Zero-based page offset; continue with returned offset + items.length. Browse clamps negative values to zero; lookup/recommendation reject invalid/out-of-range values.")
		schema["minimum"] = 0
		schema["default"] = 0
		if handler != "listItems" {
			schema["maximum"] = 10000000
		}
	case "file":
		integer("Source file ID; omit or use zero for automatic file selection where supported.")
		schema["minimum"] = 0
	case "library", "library_id":
		integer("Library ID; omit for all permitted libraries, or to preview a new library. Does not grant access to a library.")
		schema["minimum"] = 0
		if handler == "lookupItems" {
			schema["minimum"] = 1
		}
	case "ordering":
		integer("Ordering ID; zero/omitted uses the current display ordering. Must belong to the requested series.")
		schema["minimum"] = 0
	case "person_id":
		integer("Positive Arc person ID. person_role optionally narrows actor/director credits.")
		schema["minimum"] = 1
	case "audio_channels":
		integer("Source audio channel count, matched on the same present file/stream as other audio filters.")
		schema["minimum"] = 1
		schema["maximum"] = 32
	case "year":
		integer("Year hint, or the requested year in review. Omit for no search hint; year-in-review defaults to the latest watched year or current server-local year.")
	case "decade":
		integer("Decade start year (e.g. 1990); omit/zero disables the filter.")
	case "added_within":
		integer("Only titles added within this many days; nonpositive/omitted disables it.")
	case "min_runtime", "max_runtime":
		integer("Runtime bound in minutes; nonpositive/omitted disables this bound.")
	case "min_rating":
		schema = map[string]any{"type": "number"}
		description = "Minimum metadata score out of ten; nonpositive/omitted disables the bound."
	case "shift_ms":
		integer("Signed subtitle cue shift in milliseconds; negative shifts earlier.")
		schema["default"] = 0
	case "after":
		integer("Return log entries after this server log sequence number; omit/zero starts from retained entries.")
		schema["default"] = 0
	case "before":
		integer("Return history entries before this play-history ID; omit starts with newest entries.")
		schema["minimum"] = 0
	case "revision":
		integer("Required current play-queue revision. A stale revision conflicts; do not silently retry destructive changes.")
		schema["minimum"] = 1
		required = true
	case "days":
		integer("Statistics window in days; omitted or zero means all time.")
		schema["default"] = 0
	case "optimized":
		integer("Select an optimized copy by output height; pair with optimized_rev to identify its generation.")
		schema["enum"] = []int{720, 1080}
	case "optimized_rev":
		description = "Opaque optimized-copy generation returned by playback; never invent or reuse it for another file."
	case "reverse":
		flag("Reverse the selected non-shuffle sort; only the string 1 enables this flag.", false)
	case "letters":
		flag("Include the filtered jump-to-letter counts; only 1 enables it.", false)
	case "deep":
		flag("Allow expensive cross-provider episode mapping on explicit series refresh; only 1 enables it.", false)
	case "safe":
		flag("Use safe trash removal; only 1 enables it. Busy/unsafe libraries return a skipped reason rather than forcing removal.", false)
	case "with_up_next":
		flag("Include Up Next in Continue Watching; only 1 enables it.", false)
	case "unmatched":
		flag("Include only unmatched/unplaced titles. Both 1 and true enable this flag.", true)
	case "unwatched":
		flag("Filter unwatched titles. Both 1 and true enable it; cannot conflict with watch_state.", true)
	case "missing":
		flag("Include metadata-only episodes without present files. Both 1 and true enable it.", true)
	case "sort":
		enumeration("Library sort key. Reverse preserves stable ID tie-breaking; shuffle stays stable within a day.", "title", "added", "year", "release", "rating", "imdb", "critics", "runtime", "watched", "shuffle")
		schema["default"] = "title"
	case "kind":
		enumeration("Media identity kind; omit to include all kinds permitted by this endpoint.", "movie", "series", "episode")
	case "resolution":
		enumeration("Source resolution band; choices do not guarantee titles exist in the current library.", "4k", "1080", "720", "sd")
	case "dynamic_range":
		enumeration("Stored source dynamic range, not proof of client/output HDR capability.", "sdr", "hdr10", "hlg", "hdr10_plus", "dolby_vision", "unknown")
	case "watch_state":
		enumeration("Profile watch-state filter; do not combine conflicting unwatched/state filters.", "unwatched", "played", "unstarted")
	case "person_role":
		enumeration("Requires positive person_id.", "actor", "director")
	case "provider":
		if handler == "lookupItems" {
			enumeration("Provider namespace for local lookup; this does not query the provider.", "tmdb", "tvdb", "imdb")
			required = true
		} else {
			enumeration("Metadata search provider; defaults to tmdb.", "tmdb", "tvdb")
			schema["default"] = "tmdb"
		}
	case "provider_id":
		description = "Required provider ID: positive decimal TMDB/TVDB ID or IMDb tt identifier. Kept as text to support all namespaces."
		required = true
	case "type":
		enumeration("Selected artwork for regional palette extraction.", "backdrop", "primary")
		schema["default"] = "backdrop"
	case "range":
		enumeration("day requests minute samples for the last day; omit for two-second samples over the last five minutes.", "day")
	case "level":
		enumeration("Minimum log level (canonical values; parser also accepts slog case/offset aliases).", "debug", "info", "warn", "error")
		schema["default"] = "info"
	case "size":
		enumeration("full requests the original backdrop when available; omit for the existing normal artwork size. Posters are not resized by this parameter.", "full")
	case "jump":
		description = "Title-sort boundary: a–z (case-insensitive) or # for other titles. Continue paging with the returned offset and omit jump."
		schema["pattern"] = "^[A-Za-z#]$"
	case "q":
		description = "Search text; interpreted as title/provider text rather than SQL or a regular expression."
		if handler == "searchMatches" {
			required = true
			schema["minLength"] = 1
		}
	case "about":
		description = "Comma-separated terms matched in title, tagline, description or metadata keywords."
	case "genre":
		description = "Genre from the library's access-controlled facets."
	case "label":
		description = "User label from the library's access-controlled facets."
	case "language":
		description = "Original language code (normally two letters), not audio-track language."
	case "languages":
		description = "Comma-separated subtitle language codes; defaults come from the active profile/provider policy."
	case "video_codec", "audio_codec":
		description = "Source codec identifier (1–32 characters). Discover available codecs using the library filter/facet endpoints."
		schema["pattern"] = "^[a-zA-Z0-9_][a-zA-Z0-9_.-]{0,31}$"
	case "path":
		description = "Server-side folder-picker path; omit to list allowed roots. Administrative endpoint; never infer mounts from the client filesystem."
	case "profile":
		description = "Profile ID to filter admin statistics; omit for all profiles."
	case "v":
		description = "Opaque artwork cache version included in returned image URLs. Matching versions permit immutable caching; do not synthesize it."
	default:
		return nil, fmt.Errorf("document query parameter %s on %s", name, handler)
	}
	return map[string]any{"name": name, "in": "query", "required": required, "description": description, "schema": schema}, nil
}
