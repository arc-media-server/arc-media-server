# The Watchlist moves in the sidebar like a library

The owner's choice (31 October): the Watchlist moves in the sidebar order like a library, as Plex
pins sources, and its Home row follows it. Before, it was a fixed link above the libraries and a
fixed row near the top of Home, with no way to move it.

## What changed

- **Server:** a new profile preference, `watchlist_position` (additive to `/api/v1`), says how many
  libraries come before the Watchlist. 0 or absent puts it first. It's validated as 0–500 and kept
  per profile, like `library_order`.
- **Web sidebar:**
  - The Watchlist leaves the Home/Playlists group and sits in the Libraries list.
  - It has a ⋯ with Reorder libraries…, and moves in the same in-place list, the phone dialog and
    the phone bar.
  - Reset to server order puts it back first.
  - A profile with a single library can now reorder too, to move the Watchlist around it.
- **Web Home:** the Watchlist's row sits before the rows of the library that follows it in the
  sidebar. Continue Watching, Up Next and the seasonal row stay first. The Home editor shows the
  same order.
- **TV app** (arc-android#3): the Watchlist is in the TV sidebar at the same place and opens a
  Watchlist page, and its Home row follows. The TV's Queue row moves to the end of Home.

## Checks

- Go: the preference round trip, validation of -1 and 501, and isolation per profile
  (`webserver_prefs_test`).
- Web: a unit test of the Home order for positions 0, 1, 2 and out of range. The sidebar reorder
  journey is updated to include the Watchlist and passes. It moves the Watchlist last and resets
  it, and checks the single-library ⋯.
- The `journeys.spec.ts:349` library ⋯ failure also fails on main, from the sidebar's own ⋯
  clashing with its strict locator. It is not caused by this change.
