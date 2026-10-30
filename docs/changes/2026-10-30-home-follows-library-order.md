# Home follows the library order

**What changed.** Home's rows are no longer moved one by one. Their order follows the profile's
library order (the sidebar's), as Plex's Home follows its pinned libraries:

1. Continue Watching, Up Next (when separate), Watchlist and the seasonal row, in that order;
2. then, for each library in the profile's order, its Recently Added followed by its own filter
   rows (in the order they were added);
3. then collections and playlists.

Reordering libraries (sidebar ⋯ → Reorder libraries…, or Edit Home → Reorder libraries…)
reorders Home. Edit Home keeps hiding rows, adding filter/collection/playlist rows, the Up Next
split, the featured banner switch and, for a TV library's Recently Added, grouping new episodes
by season. Its drag handles, move buttons and Reset Order are gone.

Only the rows carrying on watching (Continue Watching and Up Next) have a card style choice
(posters, season posters, thumbnails, backdrops). Every other row shows its usual look; a style
saved earlier for one is ignored and dropped on the next save.
A filter row from a home videos library now defaults to thumbnails, as that library's Recently
Added does (it used to default to posters on the web and thumbnails on the TV).

**Why.** Owner, 30 October: "the row ordering should be based on the library ordering rather
than each row moved independently; posters style should be changeable only for the continue
watching row", for both the web and the TV app.

**Decisions** (logged on #80):
- The saved `home` layout keeps its shape (`/api/v1` is unchanged); its order is now ignored.
  An old layout keeps its hidden rows, custom rows and carrying-on looks.
- Filter rows sit under their library because their filter names one. Collections and playlists
  don't belong to one library in the saved layout, so they come last.
- Another profile's Home (Profiles → Edit Home) follows that profile's library order; the
  default home follows the server's.

**Limits.** No server change. The Android app follows the same rule in its own PR.

**Checks.** `npx tsc -b`, `npm run test:i18n`, the updated Edit Home browser journey
(`custom Home rows open complete filtered results and follow the library order`) passes.
`home hero: Edit Home switch…` fails before and after this change: it looks for an "Edit Home…"
link in Settings that main no longer has.

**Logging.** Nothing new: ordering is computed in the browser, and saves use the existing
preferences endpoint and its error handling.
