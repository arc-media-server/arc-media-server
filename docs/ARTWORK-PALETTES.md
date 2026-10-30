# Artwork palettes for apps

Arc extracts and caches four regional artwork colours on the server. Apps draw their own ambient
gradients, position the clear artwork and animate transitions. The palette is Arc's independent
`arc-regional-v1` algorithm, not Plex's private UltraBlur renderer. No web rendering was added.

## Request

```http
GET /api/v1/items/<item-id>/artwork/palette
Authorization: Bearer <your-session-token>
```

Choose a signed-in profile first, as described in [API authentication](API.md#signing-in). Any
profile can fetch palettes for titles it is allowed to see. Library/age/label restrictions apply
before cache revalidation; hidden and unknown titles return 404, unauthenticated requests 401.

The default `type=backdrop` uses the title's current backdrop. If **no backdrop is configured**,
it uses the poster/still (`primary`) instead. `?type=primary` requests that directly. Other types
return 400. A configured backdrop that fails to load does not silently switch to unrelated art.
Movie, series and episode IDs use their own selected image. No arbitrary URL/path is accepted.

Existing artwork rules apply: manually locked provider/upload choices win over local artwork,
then local artwork wins over automatic provider art. Missing local files can use the provider
fallback, matching image serving. Upload names stay confined to the server upload directory.

## Response

Example colours/version below are illustrative:

```json
{
  "available": true,
  "algorithm": "arc-regional-v1",
  "color_space": "srgb",
  "artwork_type": "backdrop",
  "artwork_url": "/api/v1/items/<item-id>/images/backdrop?v=<source-version>&palette_version=<artwork-version>",
  "artwork_version": "<opaque-artwork-version>",
  "colors": {
    "top_left": "#163064",
    "top_right": "#b84b2c",
    "bottom_left": "#172955",
    "bottom_right": "#643354"
  }
}
```

All colours are opaque `#rrggbb` strings for the decoded image's four regions. `artwork_type`
identifies the image actually selected, including a poster fallback. Treat `artwork_version` as
opaque; it identifies the algorithm and source bytes. Resolve `artwork_url` against your server
origin and authenticate the image request normally. It contains no token or filesystem path.

There is no generated background bitmap endpoint: use these colours in the app's gradient or
shader. A separate sharp artwork layer can use `artwork_url` or the item's full backdrop image.
The app chooses contrast overlays, saturation/brightness adjustments and layout for its screen.

If optional decoration is unavailable, the endpoint still returns 200, for example:

```json
{
  "available": false,
  "algorithm": "arc-regional-v1",
  "color_space": "srgb",
  "artwork_type": "primary",
  "artwork_url": "",
  "artwork_version": "",
  "reason": "no_artwork"
}
```

Reasons: `no_artwork`, `image_unavailable`, `unsupported_image`, `image_too_large`,
`no_visible_colors`. `colors` is absent when unavailable. Use the app's normal theme fallback;
do not treat this as an inability to browse or play. Retry transient failures with bounded
backoff, not a rapid loop. A cancellation may end the request before any response is written.

## App caching and navigation

Responses use `Cache-Control: private, no-cache` and an ETag. Retain the JSON and send
`If-None-Match` on revalidation; 304 means reuse that response. ETags cover the entire response,
so a different source URL is noticed even if the selected artwork bytes/colours are identical.
Permissions are checked on every conditional request. Do not share cached title information
across server/profile boundaries; clear or re-scope it when the profile changes.

Request the focused/featured title, and optionally prefetch the next few, rather than every
poster in a large grid. Transition the image and palette together, ignore stale responses when
focus changes, and honour reduced motion. A missing palette should not delay playback or the
rest of the screen.

## Server behaviour and limits

- Provider images reuse Arc's existing artwork cache. Palettes are lazy: first request downloads
  artwork if necessary and computes colours; later requests reuse memory/disk caches. No library
  scan, media probe, FFmpeg process, new provider key or external palette service is involved.
- Decoders support JPEG, PNG and WebP in Go (GIF uses its first frame). WebP uses x/image/webp.
  Version one works in decoded RGB pixel space: it does not normalize embedded ICC profiles or
  EXIF orientation. Most provider backdrops are already in their intended orientation/sRGB;
  unusual uploaded images may need normalization by their author or a later algorithm version.
- Inputs are capped at 25 MiB and 12 million pixels. At most two cache-miss extractions run at
  once, sampling at most 4096 pixels after bounded image decode. Transparent pixels do not add
  black; empty regions reuse a visible global sample, and entirely transparent images report
  `no_visible_colors`. Regional averages weight alpha and chroma; this is not a single global tint.
- Memory cache: 256 entries. Persistent derived cache: up to 1024 entries under
  `<data>/artwork/palettes`, with oldest entries removed on generation. It is disposable. Normal
  artwork pruning leaves this independently bounded directory alone; deleting the artwork cache
  removes it as well. Database backups need not preserve derived palettes.
- Local file cache invalidation uses path, size and modification time, with file-identity checks
  in memory. On a new stat fingerprint, source bytes are hashed into the version. Deliberately
  overwriting bytes while preserving both size and timestamps can evade this fingerprint; change
  the timestamp or delete the derived cache in that unusual case. Uploads have unique filenames.
- Palette requests have a 12-second context budget. Standard image decoding is synchronous and
  bounded by the input/pixel limits; cancellation is checked around it. Shared artwork downloads
  may finish warming the cache after a caller leaves, with four active downloads maximum and a
  one-minute overall deadline. Extraction/cache hits add no info-level log traffic. Failures
  produce at most one safe summary warning per minute, without URLs, local paths or tokens.

Changing colour selection in a future release requires a new algorithm identifier, invalidating
old derived palettes without changing this additive v1 response shape. Real app rendering and
performance on televisions/phones remain app-side acceptance work.
