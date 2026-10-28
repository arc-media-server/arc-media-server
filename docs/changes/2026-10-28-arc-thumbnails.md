# Arc thumbnails: a third card style for home rows (owner, 28 October)

Edit Home's card style menu gets a third choice next to Posters and Thumbnails: **Arc
thumbnails**. These are squarer (4:3) cards with the title's background art instead of the
episode's still, and its logo centred over the bottom of the art, as in the Apple TV app's Up Next.
The owner asked for it for Continue Watching; it is offered on every row because the cards work
anywhere. No row uses it by default (Continue Watching and Up Next keep landscape thumbnails, as in
Plex).

- Episodes show the show's backdrop and logo; movies their own. Without a backdrop the card falls
  back to the episode still (or the poster), and without a logo (or if it fails to load) the title is
  written on the art instead.
- The card keeps everything else: the progress bar, title and episode line underneath, play on click
  in Continue Watching and Up Next, and "Remove from Continue Watching".
- Spoiler protection has nothing to blur, since background art never shows an unwatched episode.
- Server: home rows accept `"style": "arc"`, and episodes on their own (Continue Watching, Up Next)
  now carry `images.series_logo` alongside `series_backdrop`.

Decision (logged here; Plex has no equivalent): 4:3 was chosen as "more square" while still fitting
wide background art; it is one class in `ArcArtwork.tsx` if the owner wants it squarer.

Checks: `npx tsc -b`, production build, `go vet ./...`, `go test ./...`, `npm run test:i18n`,
Playwright home-navigation, ux and i18n journeys (38 passed). Visual check: screenshots of
Continue Watching with Arc thumbnails at 1440px and 390px (logo, title fallback, progress bar) and
the Edit Home menu.
