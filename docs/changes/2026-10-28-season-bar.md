# Season bar: editing moves into the Edit dialog

The bar above a season's episodes kept Shuffle Season, Mark Watched, Edit Season, Show Missing and
an "Episode ordering…" button. The owner asked for the editing buttons to go, since editing lives
in the Edit dialog. Every feature stays reachable.

What changed:
- **Episode ordering…** is now in the show's ⋯ More menu, for every profile. It opens the same
  Edit dialog on its ordering tab; non-admin profiles still get only that tab, for their own
  profile's order (it is their only way to choose one, so the ⋯ menu now shows on a show even
  when the profile has nothing to play).
- **Edit → Seasons** now edits a season's name and summary as well as its poster. The tab lists
  the seasons of the order being viewed (including seasons with no files) and says which order
  that is. Picking a season shows Name and Summary (locked once edited, with lock toggles, as
  before) and, in aired orders, the poster picker. Season names and summaries are per order,
  exactly as the old Edit Season dialog saved them (`PATCH /api/series/{id}/seasons/{n}?ordering=`).
- In non-aired orders (DVD, absolute, groups), season posters still belong to the aired seasons,
  so the tab lists those posters underneath ("Season posters (aired seasons)").
- The Edit Season button and `EditSeasonDialog` are gone; Shuffle Season, Mark Watched and Show
  Missing stay on the bar.

Decisions: Plex keeps episode ordering in the show's Edit → Advanced, owner only, and edits a
season from the season's own page. Arc lets every profile choose its own order, so the entry point
has to be outside the admin-only tools; the ⋯ menu is the closest equivalent that everyone sees.
Logged on #80.

Limits: the poster picker needs TMDB season images; with none it says so and offers an upload.
