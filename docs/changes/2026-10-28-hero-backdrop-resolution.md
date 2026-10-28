# Sharper banner backdrops (owner request, 28 October)

The owner noticed some background art in the home page's featured banner looked about 720p.
It was: every backdrop was fetched from TMDB at its `w1280` size, which is 1280×720, then
stretched across the full width of the page. On top of that, TMDB's default backdrop for a title
(its most liked) is sometimes itself a 1280×720 upload even when sharper ones exist.

What changed:

- **Banners get the full-size picture.** Items and collections now have
  `images.backdrop_full` (`…/images/backdrop?size=full&v=…`), TMDB's `original` upload. The web
  app uses it for full-width banners: the home page's featured titles, movie and show pages, and
  collection pages. `images.backdrop` stays at `w1280` for cards (landscape cards, episode
  fallbacks, Dashboard, playlists), so rows of cards don't download multi-megabyte pictures.
- **Low-resolution backdrops are swapped for sharper ones.** When TMDB's default backdrop is
  narrower than 1920 pixels, Arc picks the most liked backdrop that is at least 1920 wide,
  preferring the same kind of picture (textless stays textless, English text stays English),
  then textless ones. With no full-HD backdrop at all it takes the widest one, if wider. TMDB's
  choice is kept whenever it is already full HD. This uses the image list already requested
  with each title's details (no extra TMDB calls); collections now ask for it too.
- **Cache:** the artwork keeper downloads the full-size backdrops after everything else, and
  the cache cleanup keeps both sizes. Full-size TMDB backdrops are usually 0.3–2 MB each, so
  the artwork folder grows by roughly that per title. It is still just a cache (safe to delete).

Existing artwork: the bigger size applies straight away (the banner link changes, so browsers
fetch it, and the keeper downloads the rest on its next pass). The better choice of backdrop
applies when a title's metadata is refreshed (the scheduled metadata refresh or Refresh
metadata); backdrops picked by hand (locked) and local `fanart.jpg`/`backdrop.jpg` files beside
the media are left as they are and served as they are. Media files are never touched.

Limits: TheTVDB backdrops and uploads are served at one size (their own); series whose library
takes details from TheTVDB have no provider backdrop today, only local art. No new setting:
Plex likewise serves full-size art for its backgrounds.

Checks: go vet, go test ./... (new tests for the backdrop choice, the full-size source and the
banner links), `npx tsc -b`, production UI build, `go run ./tools/apidoc`.
