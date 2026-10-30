# ASS/SSA subtitles as scripts, for libass

**What changed.** `GET /api/items/{id}/subtitles/{track}/ass?file=&shift_ms=` returns an ASS or
SSA track as its own script, for players that draw it with libass and so keep its fonts,
colours, positions, karaoke and other effects. Until now apps only had the track as plain cues
(JSON) or WebVTT, which lose all of that.
- The script is the track as written: copied from the video with FFmpeg (`-c:s copy`; the whole
  file is read the first time, then the script is cached per file in the subtitles cache), or
  read from the file beside the video (as UTF-8, Windows-1252 converted).
- `shift_ms` moves every event as `/vtt` does (a converted stream's start, the viewer's timing
  adjustment); events that end before 0 are left out. Styles and everything else are untouched.
- The fonts the video carries (Matroska attachments: .ttf, .otf, .ttc, .otc) are packed into a
  `[Fonts]` section in the ASS spec's encoding, which libass reads, so one file is all a player
  needs. They're extracted once (`-dump_attachment`) and cached; at most 40 MB of fonts go into
  one script, and fonts past that are left out (libass then uses similar system fonts).
- Tracks that aren't ASS/SSA answer 400; `/vtt` stays the way to get those.

**Why.** The Android TV app draws ASS with libass (arc-android#1), as Plex's and Jellyfin's apps
do. Jellyfin serves fonts from a separate attachments endpoint; the Android libass library takes
fonts for a sideloaded script only from its `[Fonts]` section, so they're packed in (logged on
#80). Picture subtitles and players without libass still have subtitles burned in when
converting, as before.

**Limits.** The first request for a track inside a video reads the whole file (as cues and
`/vtt` already do). Fonts are matched by libass by name as usual; a font the file doesn't carry
falls back to a system font.

**Checks.** `go vet`, `go test` for `internal/subtitles` (shifting, events with commas, the font
encoding, line wrapping, and a real MKV made with FFmpeg carrying an ASS track and an attached
font: script and font extracted, then served from the cache), `internal/api` and
`internal/apidoc`; `go run ./tools/apidoc` added the endpoint to docs/API.md and the v1 contract.

**Logging.** A script that can't be read is logged once at warning with the file id, track and
FFmpeg's last line; fonts that can't be read likewise (the script is still served). Cancelled
requests aren't logged. Nothing is logged per event or per font.
