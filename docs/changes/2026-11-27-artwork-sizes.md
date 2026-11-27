# Artwork at the size each screen needs (`?w=`)

## What changed

Every artwork endpoint (item, season, collection and person images) takes an optional `w`, the
width in pixels the app will draw the image at. Arc rounds it up to one of 320, 480, 780, 1280,
1920 or 3840 and serves a copy at that width, made from the cached original the first time any
app asks for it and kept on disk (`<data>/artwork/sized/<width>/…`). Every later request, from
any app or device, is served from the kept copy: nothing is resized twice.

- Images are never enlarged: an original already narrow enough is served as it is.
- Opaque pictures become JPEG (quality 85); images with transparency (logos) stay PNG.
- A backdrop asked for wider than 1280 is made from the provider's original upload, so a 1920 or
  3840 hero is sharp; narrower requests use the 1280 copy cards already use.
- Without `w` nothing changes: the same files, links and caching as before.
- Copies go with their original when the artwork cleanup removes it.
- A resized copy has its own ETag (`<key>-w<width>`) and keeps the year-long immutable caching.

## Why

The Android TV app (Chromecast with Google TV) downloaded and decoded the TMDB original for the
Home and title-page hero, often 3840×2160 and several MB, to draw it on a 1920×1080 surface.
Decoding and holding that bitmap costs a weak TV's memory and time on every change of focus.

## Decisions

- **Made on first request and kept, not pre-generated for the whole library.** Plex's photo
  transcoder works the same way: clients ask `/photo/:/transcode` for a width and height, and
  Plex caches the result. Pre-generating every width for every image would multiply the artwork
  folder for sizes that may never be asked for. The first view of an image at a new width waits
  for one resize (about 0.6 s for 4K to 1920 on four cores); every later view is a plain file
  read. Pre-warming the 1920 hero copy in the background is an easy follow-up if that first wait
  shows on a TV.
- **A fixed set of widths** rather than any width: a handful of copies per image at most,
  however many different screens ask, and copies shared between apps.
- **Width only, no height or crop**: artwork keeps its shape; apps already crop with their
  layout. Plex also accepts a height and can crop; Arc can add that if an app needs it.

## Checks

- `go vet ./...`: clean.
- `go test ./...`: all pass except `internal/transcode`'s FFmpeg tests, which need Jellyfin's
  FFmpeg build (`tonemapx`, `dovi_rpu`) that this container doesn't have; that package doesn't use
  any of the changed code.
- New tests: width snapping; resize keeps shape and transparency; a copy is made once, kept and
  reused (provider asked once, file not rewritten), never enlarged, removed with its original;
  an undecodable image serves the original; the endpoint returns a 1280px JPEG with its own ETag
  and immutable caching for `?w=1000`, and the original without `w`; wide backdrop requests use
  the original upload.
- `cd web && npx tsc -b` and `npm run build`: pass. `go run ./tools/apidoc` regenerated
  `docs/openapi.json`.
- Not tried on a real TV yet: that comes with the Android change that asks for `w`.

## Logging

- Debug: "artwork resized" with the image key, width and duration, once per copy made.
- Warn: "artwork resize failed; serving the original" with key, width and error (the request is
  still answered with the original). Cancelled requests aren't logged.
- No URLs or query strings are logged; the key is a hash.
