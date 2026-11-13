# Card size: one size everywhere, a proper slider, Play that fits the card

Follow-up to #450 (owner: it works but doesn't look right, and Home and libraries scale
separately).

- **One size.** Home, libraries, collections, and the other pages with media cards (search,
  watchlist, playlists) now share one card size, set from either slider. The server still keeps
  the two fields #450 added (`home_poster_size`, `library_poster_size`); the web saves both
  together and reads them as one (Home's first), so the API and its contract are unchanged. The
  size is applied once around every page (`--card-scale` on the app's main area) rather than per
  page.
- **The slider.** It was the browser's default range control (a heavy bar and knob) between two
  hard-to-read poster outlines. It is now a fine 3px track filled up to a 12px round knob in the page's
  own foreground colour, with a grid icon (smaller cards) and a square (larger), in light and dark.
- **Hover Play.** The Play circle was a fixed 48px with a thick ring, covering much of a small
  poster's title art. It is now about a fifth of the card's width (32–44px, a CSS container size)
  with a thinner ring and a lighter fill, and its icon scales with it.
- A collection page no longer shows its own "preferences unavailable" notice: without the
  profile's size it simply shows standard-size cards, like every other page.

Checks: `npx tsc -b`, production build and i18n tests passed; the eight card-size browser
journeys pass, updated for one shared size (both fields saved together, the size carried between
Home and a library, each profile its own). Screenshots of the slider (light and dark) and the
hover state at 75/100/150% checked; the test library has placeholder posters, not real artwork.

No logging change: a web-only presentation change; saving uses the existing preferences request.
