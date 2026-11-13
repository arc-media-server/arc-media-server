# Recently Added season cards show their new-episode count again

#449 shortened season cards' caption to the season ("Season 2") and dropped the new-episode
count ("Season 2 · 3 new episodes"), and the card also hides the show's own unwatched badge
(it would be the whole show's number), so nothing on the card said how many episodes arrived.
The owner missed it. The count is now the season card's corner badge, in the same place and style
as the unwatched badge (as Plex shows it), with "3 new episodes" as its label and tooltip; the
caption stays "Season 2". Whole-show and single-episode cards are unchanged.

`CardOverrides` gains `count`, drawn by a new `CountBadge` beside `WatchBadge` in place of it.

Checks: `npx tsc -b`, production build, i18n tests (21 passed), and the Recently Added, i18n and
Home actions browser specs (7 passed, with a new check that each season card's badge carries its
count). No logging change.
