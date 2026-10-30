# Continue Watching cards: one play button, Remove in the ⋯

**What changed.** On Continue Watching and Up Next cards with the shared hover controls (play,
edit, ⋯):
- the card no longer draws its own play hint as well, so there's one play button, not two
  stacked;
- Remove from Continue Watching moved from its own ⋯ in the top left into the card's ⋯ menu
  (bottom right), after Watched and Watchlist;
- on a Backdrops (4:3) card, the controls now sit on its artwork (they were laid out for a 16:9
  card, so the pencil and ⋯ floated part-way up the picture).

Cards without the shared controls keep their own play hint and the top-left ⋯ for Remove.

**Why.** Owner, 30 October, with a screenshot of a Backdrops Continue Watching card: two play
buttons, controls in the wrong place, and Remove best in the existing menu.

**Checks.** `npx tsc -b`; `e2e/home-actions.spec.ts` (both journeys pass, including a new one:
one play icon, the ⋯ in the 4:3 card's corner, Remove from Continue Watching from the ⋯).

**Logging.** Nothing new: Remove uses the existing request and its error handling.
