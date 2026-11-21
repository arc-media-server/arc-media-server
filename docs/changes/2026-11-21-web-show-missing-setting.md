# Show missing episodes moves into Settings (web)

## What changed

- Settings, Display has a new **Show missing episodes** switch. It's saved on the profile
  (`show_missing_episodes` in the profile's prefs, added in #495), so it applies to every show and
  follows the profile to every app.
- A show's page no longer has its own Show Missing / Hide Missing button; it lists missing episodes
  when the setting is on.

## Why

The owner chose one setting for all shows instead of a toggle on each show (the Android, Kodi and
LG/Samsung apps follow the same setting).

## Decisions

- Off by default, as before. Plex doesn't list episodes you don't have, so this is Arc's own; a
  per-show toggle was the alternative, dropped because it didn't stay set and had to be found on
  every show.

## Checks

- `npx tsc -b`, the production build.
- Browser journeys: the season bar test (button gone), the Settings preference tests (new switch
  saves and reloads), the episode-details and storage-unavailable journeys: all passed. Run with the
  container's own Chromium and no video recording (the pinned Playwright browser and ffmpeg aren't
  installed here); the config wasn't changed in the commit.
- No screenshot taken; no Go code changed.

## Logging

Nothing new: a preference save already reports failures in the page and the server logs nothing
for ordinary preference changes.
