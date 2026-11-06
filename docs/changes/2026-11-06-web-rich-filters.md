# Web: richer filters (video and audio formats, HDR, watch status)

The server could already filter titles by video codec, audio codec, channel count and HDR type,
and by watch status (docs/FILTER-DISCOVERY-API.md). The web app didn't offer any of it. Now it does.

## What changed

- **Library Filters panel:** a new **Video and audio** section with HDR (Dolby Vision, HDR10+,
  HDR10, HLG, SDR), Video format (HEVC, H.264, AV1, MPEG-2…), Audio format (Dolby TrueHD, Dolby
  Digital Plus, DTS…) and Audio channels (7.1, 5.1, Stereo…).
  - Each lists only what the library's files have, with how many titles.
  - A choice appears only when there's more than one value, or a value is already chosen.
  - Choices show as chips beside Filters and live in the address, like every other filter.
- **Watch status:** Any, Watched or Not started.
  - "Only titles I haven't watched" stays as the quick switch. Unwatched includes titles in
    progress; Not started doesn't.
  - Choosing one turns the other off, because the server refuses both together. An old link with
    both keeps Unwatched.
- **Saved filters:** custom Home rows, smart collections and smart playlists offer the same
  choices. A custom row's link opens the library with them.
- **Server:** `GET /libraries/{id}/facets` gains `video_codecs`, `audio_codecs`,
  `audio_channels` and `dynamic_ranges`.
  - Each value comes with how many titles have it, in the library's present files.
  - A show counts once, through its accessible episodes.
  - The profile's access rules apply, so a restricted profile isn't told what hidden titles
    contain.
  - Additive to `/api/v1`.
- Format names (`lib/media-labels.ts`) are brand names and aren't translated. "Mono", "Stereo",
  "N channels" and "Not known yet" are.

## Decisions (logged on #80)

- **Only real values:** a filter lists only values the library has, as Plex does, rather than
  every format Arc knows. A format with one value isn't offered.
- **Unknown HDR:** files not analysed yet show as "Not known yet" rather than being counted as
  SDR.
- **Placement:** the video and audio filters sit in their own section of the panel, not the
  top bar, to keep the bar short.

## Checks

- Go:
  - `TestMediaFacetsCountTitlesAndRespectAccess`: titles are counted once even with two tracks;
    the order is right; each value finds the titles it counted; an age-limited profile doesn't
    see a restricted title's formats.
  - `TestMediaFacetsCountShowsOnceThroughEpisodes`.
  - `go test ./...` and `go vet` pass; the API contract and docs are regenerated.
- Browser, `e2e/filters.spec.ts`:
  - The library offers exactly H.264 and MPEG-2 with counts, and no audio format, since the
    fixture has only one.
  - MPEG-2 narrows the library, shows a chip and survives a reload; the chip removes it.
  - Watched finds the watched title, and the Unwatched switch replaces it.
  - A custom Home row made of a video format shows only that title, and its link carries the
    filter.

## Logging

No new logging. The facet queries are ordinary reads, and failures surface through existing
request logging.

## Limits

- A library has to be analysed for formats to appear; files not analysed yet don't count.
- Channel count is the file's track, not what plays: 7.1 doesn't promise Atmos or that this
  device outputs 7.1.
- The Android TV and Apple TV apps don't offer these filters yet (the facets are ready).
