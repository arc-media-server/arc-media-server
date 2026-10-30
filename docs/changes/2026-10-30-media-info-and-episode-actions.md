# Media Info and quieter episode rows (30 October)

Follows #291 (one shared media information dialog), after the owner's review.

## What changed

- **Episode rows** show only Watched (the circle) and ⋯, always visible (no hide-on-hover).
  Everything else moved into the episode's ⋯, which is the title page's `MoreMenu` (now with a
  `compact` trigger and `extra` items), not a second menu:
  Play next, Add to queue, Add to Playlist… · Playback options…, Episode details…, Media Info… ·
  then, for admins, Edit… and Optimize…. Add to Collection and Fix Match are left out for episodes.
  Every profile gets the ⋯; unavailable episodes still have it for Episode details (and Edit).
- **Episode details** (the old book icon: overview, crew, guest cast) is now "Episode details…"
  in the ⋯. Clicking a row still plays, as before.
- **Movie page:** the "Media information" text button is gone; "Media Info…" is in the movie's ⋯
  for admins and other profiles. The other hero buttons are unchanged.
- **Media Info dialog** (Plex's Media Info as the reference), per file/version/part:
  - Video: Codec, Resolution ("3840×2160 · 4K"), Aspect ratio, Frame rate ("23.976 fps",
    "24 fps"), Bit depth, HDR (Dolby Vision profile N / HDR10+ / HDR10 / HLG / SDR), Bitrate.
  - Audio and Subtitles: a table with one row per track (Language, Codec, Channels, Bitrate;
    Language, Format, Forced). A column no track has a value for is dropped.
  - File: Container, Size, Overall bitrate, Duration. Then the file's location with Copy.
  - Every value is labelled; a missing value is left out rather than shown as "—".
  - Codec profile, pixel format, colour primaries/transfer and how HDR was checked ("Checked
    from a sample of frames", etc.) sit under a closed "Advanced" disclosure.
- `SourceFilesButton`, `EpisodeDetailsButton`, the compact `PlaybackOptionsButton`, the
  `QueueActions` button and `AddToPlaylistButton` were only used by the busy rows and are
  replaced by controlled dialogs (`MediaInfoDialog`, `EpisodeDetailsDialog`,
  `PlaybackOptionsDialog`) opened from the menus. Unused locale keys were removed.
- The Dashboard's Direct Play / Direct Stream / Transcoding labels from #291 are unchanged.

## Decisions

- Menu labels reuse the existing strings ("Play next", "Add to queue") rather than new
  title-case copies.
- When HDR couldn't be checked and the file reports no transfer, the HDR row is left out rather
  than claiming SDR.

## Logging

UI only: no server change, so no new logging. Failures loading Media Info or episode details
show inline with Retry, as before.

## Checks

`npx tsc -b`, `npx tsc -p e2e/tsconfig.json` and `npm run build` (with the i18n audit) pass.
Affected browser journeys pass: episode-details (imported credits/keyboard, Retry/spoilers,
inline specials), ux (movie source labels/Media Info, episode Media Info), preplay (episode
keyboard choices), recovery (episode Space/Enter), maintenance (title More menus for movie and
series, non-admin More, and the new episode-row test). The full suite was not run (owner's
scope for this change).
