# Artwork: hero copies made ahead of time, and a minimum height (?h=)

Builds on #541 (`2026-11-27-artwork-sizes.md`), which added `?w=` and kept the resized copies.

## What changed

**Hero copies made ahead of time.** When the artwork Keeper has downloaded a backdrop's full-size
original (TMDB's "original" upload, `FullSource`), it now also makes that backdrop's 1920-wide
copy if there isn't one yet. Before, the first visit to a title on a TV waited about 0.6 s while
the 4K original was resized.

- Only full-size backdrops, and only at 1920 (`artwork.HeroWidth`). Posters, stills, the w1280
  card backdrops and every other width are still made on request.
- Originals 1920 wide or less, and ones over the 50-megapixel decode limit, are skipped. So are
  originals not cached yet: warming never downloads.
- It uses #541's resize path (`Cache.resize`, `resizeToDisk`, `Resize`). A request that arrives
  while a copy is being warmed waits for that same job instead of starting a second one.
- It runs after the pass's downloads and cleanup, so resizing never delays fetching originals. It
  uses the same four download slots, goes most important first like `Prefetch`, and stops when
  the Keeper is closed. A cancelled warm-up job stops too; a viewer's request still finishes its
  copy if the viewer moves on.
- Clean-up hasn't changed: copies share their original's file name, so `Prune` removes them with
  it.

**`?h=`, at least this tall.** Every artwork endpoint that takes `w` now also takes `h`. The copy
is made wide enough to be at least `h` tall:
`target = max(w, ceil(h × width ÷ height))`, which is then rounded up like `w`.

- This is for an app that crops an image to fill a box taller in shape than the image. For
  example, a 16:9 backdrop in a 400×400 square: `w=400&h=400` gives the 780 copy (439 tall)
  instead of 480×270.
- Copies are still stored by width only, so `h` never makes a new kind of copy. It never crops,
  never enlarges (the original is served when it isn't tall enough) and stays capped at 3840.
- `h` alone works. An invalid `h` is ignored, like an invalid `w`. `w` alone behaves exactly as
  in #541.
- A backdrop whose resolved width is over 1280 is made from the full-size original, as a wide
  `w` already was. The width is worked out from the card image's shape (same shape, header read
  only).
- `OpenSized` now returns the width it actually served (0 for the original), and the ETag uses
  it. Before, the ETag used the requested width even when the original was served.
- Without `h`, the stored-copy fast path is unchanged: it opens the copy before reading anything.
  With `h`, the original's header is read first (`image.DecodeConfig`).
- API: `h` is described next to `w` (integer, minimum 1); `docs/openapi.json` was regenerated.
  This is additive: no contract lines were removed.

## Decisions

- **Warming ahead of time.**
  - *What Plex does:* its photo transcoder makes resized copies on request only, and its
    `minSize` option is the same idea as `h`.
  - *Why Arc differs for this one size:* the hero is the one image every title page and featured
    row shows full screen. Android TV draws its interface at 1920×1080 even on 4K TVs and asks for
    1920 (arc-android#74), so this copy is certain to be wanted and is the slowest to make.
  - *What it costs:* one ~300–600 KB JPEG per full-size backdrop, made in the background.
  - *What stays on request:* every other width varies by app and screen.
  - *The alternative:* warm nothing, and accept the first-visit wait.
- **How the warm-up list is passed.** `Keep`'s list function now returns `artwork.Images{Sources,
  Warm}` instead of a plain `[]string`. `Warm` is the full-size backdrops the server wiring
  (`cmd/ams/main.go`) already gathers separately, in the same most-important-first order.
  - *Why:* a second list from the same listing is the simplest way to say which sources are
    full-size backdrops, without the cache guessing from URLs.
  - *Rejected alternatives:* a Keeper option, or a predicate on URLs. Both are more machinery for
    one list.
- **`h` uses the card image's shape** to decide whether a backdrop needs the original. The card
  is cached by the Keeper, so the decision is a header read, not a download of the 4K original.
- **The Android app will send `h` in a follow-up PR** (arc-android `data/ArtworkWidth.kt`, which
  already knows Coil's height). No app sends it yet.

## Checks

- `go vet ./...`: clean.
- `go test ./...`: everything passes except `internal/transcode`'s FFmpeg tests. They fail in this
  container because it has Ubuntu's FFmpeg 6.1, not Jellyfin's (`dovi_rpu`, `tonemapx`, and the
  remux offset and picture-subtitle cases). Those tests are unrelated to this change.
- `go test -race -count=2` on `internal/artwork` and the artwork tests in `internal/api`: pass.
- `cd web && npx tsc -b` and `npm run build`: pass.
- `go run ./tools/apidoc`: `h` added to the four artwork endpoints in `docs/openapi.json`; no
  contract lines removed.
- **New tests**, all with generated images and httptest servers only:
  - A Keeper pass over a 3840×2160 backdrop leaves its 1920×1080 copy and no other width.
  - Posters, card backdrops and a 1920-wide original get no copy.
  - A second pass does no work and leaves the copy untouched.
  - Warming never downloads.
  - Cancelling while warm-up waits for a busy slot stops it promptly and leaves no copy.
  - `h` cases:
    - `w=400&h=400` on 16:9 gives 780, from the kept copy the second time;
    - `h` alone gives 780;
    - an invalid `h` is ignored;
    - `h` taller than the original serves the original with the plain ETag;
    - 3000×2000 with `w=1920&h=1080` gives 1920;
    - the ETag carries the width served.
  - Backdrop source: `w=400&h=800` and `h=1080` use the full-size original, `w=400&h=400` keeps
    the card, `w` alone is as before, and posters ignore `h`.
  - `fitWidth` table, including the 3840 cap.
  - #541's tests pass with only the call to `OpenSized` adapted to its new signature.
- **Not tried:** a real library, a real TV, timings on the owner's hardware. The 0.6 s figure
  comes from #541.

## Logging

- The Keeper's info line "artwork cache updated" gains `resized` and `resize_failed`. It's still
  logged only when something happened.
- Each warmed copy gets the existing debug line "artwork resized" (key, width, duration).
- Warm-up failures get one debug line each, at most three per pass ("artwork warm-up resize
  failed", key and error).
- Logs carry the key, never URLs or query strings. Nothing new is logged for `h`.
- **Memory:** decoding a 4K image takes about 33 MB per worker, with at most four at once (the
  shared slots).
