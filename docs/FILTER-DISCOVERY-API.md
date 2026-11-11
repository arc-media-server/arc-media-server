# Rich filters and discovery

`GET /api/v1/libraries/{id}/filters` returns filter descriptors (`key`, `type`, `operator`,
optional `values`, bounds and facet name), sort names and `facets_url`. Supported enum values
are capabilities, not a claim that those formats exist in this library. Existing facets provide
permission-filtered genre/language/label/decade/resolution values and counts. Codec type accepts
an ffprobe identifier, 1–32 ASCII letters/digits/underscore/dot/hyphen; it is not an enum.

New GET `/items` query parameters, also accepted in saved custom Home row, smart playlist and
smart collection filter objects:

- `video_codec`, `audio_codec` (case-insensitive exact ffprobe name, e.g. hevc, truehd, eac3).
- `audio_channels` (1–32, exact channel count, e.g. 8 = 7.1; not an Atmos claim).
- `dynamic_range`: dolby_vision, hdr10_plus, hlg, hdr10, sdr, unknown.
- `watch_state`: unwatched, played, unstarted (existing show watch-state semantics apply).
- `person_id`, optionally `person_role` actor or director.

All are ANDed with existing filters, access rules, counts, letter navigation and pagination.
Conflicting watch_state and legacy unwatched filters are rejected. New values are validated
in browsing and saved filters. The discovery endpoint also lists existing browsing-only filters
such as about/unmatched; those are not newly added to saved row filters.

Audio codec and channel count must match one stream. Video codec and dynamic range match one
video stream; audio and video must belong to the same present file. A series qualifies through
an accessible child episode. Existing resolution filtering retains its existing movie/file
semantics; its legacy behavior is unchanged.

Dynamic range describes stored source information, prioritizing Dolby Vision, HDR10+, HLG,
HDR10, known SDR, then unknown. A DV source with an HDR10 base layer is classified as DV here,
not counted again under HDR10. Missing probe metadata never silently becomes SDR. No actual
output/HDR passthrough/Atmos capability is implied. No analysis/probe/provider work is triggered.

`GET /libraries/{id}/facets` lists the values a library actually has, with title counts, most
titles first: `video_codecs` and `audio_codecs` (lower-case ffprobe names), `audio_channels` (as
strings, "6" is 5.1) and `dynamic_ranges`. A show counts once through its accessible episodes, and
the profile's access rules apply. Each value works as the matching `/items` filter, so apps can
offer only what exists. The web app's Filters panel uses them (Video and audio) together with a
Watch status choice, as do saved filters for custom Home rows, smart collections and playlists. Logging reuses bounded request diagnostics; no per-title info output. Fixture
tests verify cross-file/track false positives, DV precedence, SDR/unknown, series access and
validation. Real devices and large-owner-library timing are not validated.
