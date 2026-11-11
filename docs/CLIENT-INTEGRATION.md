# Integrating an Arc client

Use `/api/v1` for applications. `/api` is the web app's evolving interface. The complete generated
contract is [openapi.json](openapi.json), the readable reference is [API.md](API.md), and an Arc
server serves its bundled interactive viewer at `/docs/`. New fields and capability keys are additive;
ignore unknown fields. Check the running server's capabilities first. The core guide is bundled at `/docs/client-integration.html`;
links to specialist guides open the repository documentation, which may describe a newer server.

## First client: sign in and browse

The requests below use the stable prefix explicitly. Replace the sample password, token, profile
and item IDs with your own values; IDs shown in examples are illustrative. Send JSON with
`Content-Type: application/json`. Use HTTPS for remote access and keep tokens in your client's
protected credential store. Never put a session token in a URL or log request credentials.

### Get a device token

On an already configured development server, send `POST /api/v1/auth/login`:

<!-- example: device-login -->
```json
{"password":"replace-with-household-password","device":"Example client"}
```

A **nonempty device name** selects the app flow. HTTP200 returns:

```json
{"token":"replace-with-private-session-token"}
```

Omitting device selects the browser-cookie flow: HTTP204, no JSON token. A friend instead sends
name/password/device to `POST /api/v1/auth/friend`; that token already selects their profile.
The bundled viewer's Test Request uses the current browser session and enforces its profile's
permissions. GET requests read state; POST/PUT/PATCH/DELETE may change it. Use a temporary server
for mutation examples, not a production store.

### Select a profile

Send `Authorization: Bearer <token>` on subsequent requests. `GET /api/v1/auth/status` returns
household profiles. A newly signed-in household device returns profile:null until it selects a profile. Pick a returned
ID with `POST /api/v1/auth/profile`:

<!-- example: select-profile -->
```json
{"id":"replace-with-returned-profile-id"}
```

Include pin when that profile requires it; HTTP401 with needs:pin asks for the PIN. A password may
be needed for an admin profile entered through the home-network bypass. Profile selection returns
HTTP200. Keep using the **same device token**; `GET /api/v1/identity` now returns the server identity.
Do not create a new login/token each time you switch profiles or restart the client.

Read `GET /api/v1/capabilities` on connection and profile changes. Missing feature keys mean
unsupported. supported means implemented, available describes attached services, and allowed
reflects this profile's role. Library/item permissions are still enforced on every request.
Server identity survives normal updates/restores, but is not a credential or a remote relay.

### Browse and choose a playable title

`GET /api/v1/libraries` lists permitted libraries. Then request
`GET /api/v1/items?library_id=<returned-id>&kind=movie&limit=20&offset=0&sort=added&reverse=1`.
Use kind=series for shows. The response contains items/total/offset. Keep filters/sort stable while
paging. Fetch `GET /api/v1/items/<returned-item-id>` for details; resolve a series to a movie or
specific episode before playback. Empty items:[] is normal for an empty or filtered library.

### TV linking

TVs can avoid typing the household password. `POST /api/v1/auth/link` accepts:

<!-- example: device-link -->
```json
{"device":"Example living-room TV"}
```

HTTP200 supplies code/poll/expires_in. Display code and ask the person to approve it through
Arc's profile menu → Link a Device (`/link`). Poll `GET /api/v1/auth/link/<poll>` every few seconds:
HTTP202 means waiting, HTTP200 returns the token **once**, and HTTP404 means expired or already
consumed. The displayed code lasts ten minutes; do not save it as a credential. Rate-limit responses
require backoff. A household token still follows the profile-selection step above; a friend token
already selects that friend. Poll identifiers are limited linking secrets, not general session URLs.

## Preview, play, report progress and stop

`GET /api/v1/items/<item-id>/playback-options` discovers available files/audio/subtitles and may
analyse a file. Keep returned file IDs and stream indexes; do not guess indexes from language names.
`POST /api/v1/items/<item-id>/playback-preview` is read-only and uses saved metadata. Example body:

<!-- example: playback -->
```json
{"caps":{"containers":["mp4"],"video":["h264"],"audio":["aac"],"max_height":1080,"max_audio_channels":2,"hdr_formats":[]},"start_ms":0}
```

This example assumes a player verified to support MP4/H.264/AAC and fMP4 HLS. Adapt caps to the
actual player; [] for hdr_formats explicitly declares SDR only. A declaration does not prove
physical HDR/audio output. To select tracks/versions, include a returned file_id/audio_stream or
supported subtitle choice. For resuming, use saved source-relative progress instead of start_ms:0.

Preview returns decision_only:true and can_start. needs_analysis:true means offer analysis first;
a preview has no usable playback URL. HTTP422 on a real start explains an unsupported fallback.
When the user presses Play, send the same body to `POST /api/v1/items/<item-id>/playback`.
HTTP200 supplies method/url/hls/offset_ms and, for conversion, session_id. Use the returned URL;
resolve relative URLs against the server origin without adding `/api/v1` a second time.
Original files support byte-range seeking and HLS plans need an HLS-capable player. Media and
segment requests need authentication too; a scoped cast ticket is a separate option for receivers
that cannot send the normal authorization header.

Progress always uses the **source file's timeline**. For a converted stream:
`source_position_ms = offset_ms + player_current_time_seconds * 1000`. Report progress periodically
and when pausing/closing with `PUT /api/v1/items/<item-id>/progress`:

<!-- example: progress -->
```json
{"position_ms":30000,"duration_ms":7200000,"state":"paused"}
```

Replace both times with the actual source position/duration in milliseconds. State is playing,
paused or stopped; omitted state means playing. HTTP200 returns the saved user data (played,
last_played_at and an optional position_ms resume point). A resume point is kept after the first
minute; completion clears it. The response does not echo duration_ms. A queue advance does not mark
an item watched: retain progress reporting. For converted playback, ping
`POST /api/v1/transcode/<session-id>/ping` while paused. On close report state:stopped and delete the
session with `DELETE /api/v1/transcode/<session-id>` (204). Direct playback has no conversion session.

Seeking within available media uses the player. Seeking outside a conversion's available timeline
requires a new playback request at the desired source start_ms; apply the new offset and release
the old session. Never add offsets twice or report converted-stream time as source progress.
See [playback decisions and native capabilities](PLAYBACK-PREVIEW-API.md),
[queues](PLAY-QUEUES-API.md), [live invalidations](EVENTS-API.md) and
[Watch Together](WATCH-TOGETHER-API.md) for their separate contracts.

## Profile library selection

Profile library_ids:null means **all libraries**; [] means **no libraries**. A nonempty integer
array selects those libraries. Omit the field on PATCH to keep existing access. Do not normalize
null to [] in a client or SDK. Admin profiles always see every library. OpenAPI describes this
nullable array explicitly; minimal profile creation needs name, while PATCH fields are optional.
Profile exclude_labels may also be null; null and [] both mean no excluded labels.

## Browsing and identifiers

Treat IDs and returned URLs as authoritative:

| Value | Wire type |
|---|---|
| Arc item, profile and queue IDs | Opaque strings |
| File and library IDs | Integers |
| Item tmdb_id / tvdb_id | Integers |
| IMDb ID | String such as tt0133093 |
| provider_id in a lookup URL | Text in the chosen provider's namespace |

 `GET /items/lookup` resolves existing local items by
provider identity, without a provider lookup. Batch lookup, facets, filters and recommendations are
covered in [ITEM-LOOKUP-API.md](ITEM-LOOKUP-API.md) and [FILTER-DISCOVERY-API.md](FILTER-DISCOVERY-API.md). Respect permission-filtered discovery results.

For `GET /items`, `limit` defaults to 100 (maximum 500), and `offset` defaults to zero. Continue with
returned offset plus the number of returned items. Stop at total/empty pages; retain the same filters
and sort across pages. Lookup and recommendation pages default to 50 (maximum 100), and validate
bad pagination with 400. Legacy browse/history endpoints may reset invalid limits to defaults.
Library flags such as `reverse` and `letters` are strings: send `1` for enabled, `0` or omission for
disabled. Do not let an SDK serialize `reverse=true`. The schema's `x-arc-logical-type` is a hint,
not a different wire encoding. Watch-state flags also accept `true`/`false` where documented.

## Errors, correlation and retries

Check HTTP status before decoding JSON. Common JSON errors preserve `error` (human-readable) and
optional `needs` (for example `pin`, `password`, `stopped`), and add `code` plus `request_id`.
`request_id` matches the `X-Request-ID` response header and existing request diagnostics. IDs are
server-issued decimal strings, process-local and unsuitable for identity, auth or global ordering.
Caller-supplied request IDs are ignored. Success bodies do not acquire error fields.

Codes describe HTTP categories: `invalid_request`, `authentication_required`, `forbidden`,
`not_found`, `method_not_allowed`, `timeout`, `conflict`, `gone`, `payload_too_large`,
`unsupported_media_type`, `range_not_satisfiable`, `validation_failed`, `rate_limited`,
`internal_error`, `upstream_error`, `unavailable`, or fallback `request_failed`. Preserve unknown
future/domain codes. Older servers may lack codes/correlation. Plain-text transport errors and
errors after streaming starts are not JSON; show a safe status fallback and retain the header ID.

401 requires sign-in/profile selection; 403 does not imply a retry will help. Refetch state on 409,
particularly queue revision conflicts; don't blindly repeat a destructive operation. Honor
`Retry-After` when present on 429/503. Back off transient reads with jitter and cancellation. Arc
does not provide a generic idempotency-key contract: a timeout does not prove a mutation failed.

## Playback and assets

Discover options and preview decisions before starting playback; see [PLAYBACK-PREVIEW-API.md](PLAYBACK-PREVIEW-API.md).
Preview is read-only. A real start returns its selected URL/session and `offset_ms`; converted
streams' clocks must be translated to the source timeline. Report progress, ping converted sessions
while paused, and stop sessions on exit. Source HDR/Atmos labels do not establish output capability.
A cast ticket is a separate scoped delivery mechanism, not a general session-token URL.

Use returned image URLs and their `v` cache key. Local/cached files support byte ranges and conditional
requests; remote artwork can redirect. Palettes support `If-None-Match`/304 and private revalidation:
see [ARTWORK-PALETTES.md](ARTWORK-PALETTES.md). A palette isn't a layout prescription. Raw image uploads
accept JPEG/PNG/WebP up to 20 MiB, not multipart forms. HLS playlists and streaming subtitle text do
not support byte-range seeking; source files and MP4 segments do. Subtitle extraction may fail after
HTTP 200 has already begun; clients need a playback/subtitle failure fallback.

## Live updates

Use authenticated fetch plus an SSE parser for native/cross-origin clients; browser EventSource can
use same-origin cookies. Cross-origin `/api/v1` allows Bearer auth and `Last-Event-ID`, exposes
`X-Request-ID`, and never enables credentials/cookie bypass. Retain the last complete event ID,
reconnect with `Last-Event-ID`, debounce invalidations and refetch from ordinary APIs. A `resync`
requires a fresh state fetch. Unknown topics must not crash clients. See [EVENTS-API.md](EVENTS-API.md)
for privacy, bounds, heartbeat and replay limits. Replay is process-local, not durable delivery;
retain periodic/refocus refresh for changes without producer hooks. Watch Together has a separate
room snapshot protocol; do not assume the general event stream's replay semantics apply to it.

## Bash read-only examples

Use a fixture/development server and a token whose profile is already selected. These commands do
not start playback or mutate settings. Don't enable shell tracing with credentials in scope.

```bash
# Bash and curl are required. PowerShell users can send the HTTP examples above.
# Supply these locally; never commit the token. curl is required.
export ARC_BASE_URL='http://127.0.0.1:8099'
read -r -s -p 'Arc token: ' ARC_TOKEN; echo
export ARC_TOKEN
curl --fail-with-body -sS -H "Authorization: Bearer $ARC_TOKEN" "$ARC_BASE_URL/api/v1/identity"
curl --fail-with-body -sS -H "Authorization: Bearer $ARC_TOKEN" "$ARC_BASE_URL/api/v1/capabilities"
curl --fail-with-body -sS -G -H "Authorization: Bearer $ARC_TOKEN" \
  --data-urlencode 'limit=20' --data-urlencode 'offset=0' --data-urlencode 'sort=added' \
  --data-urlencode 'reverse=1' "$ARC_BASE_URL/api/v1/items"
# Bounded live stream; curl exit 28 is expected after this example's 15-second time limit.
curl -N --max-time 15 -H "Authorization: Bearer $ARC_TOKEN" "$ARC_BASE_URL/api/v1/events"
# For reconnection, add -H "Last-Event-ID: $ARC_EVENT_CURSOR" using the last received id.
unset ARC_TOKEN
```

In production code supply authorization through the client's protected credential store, use HTTPS
for remote access and redact headers from diagnostics. No keys, media or owner data are needed to
test these contracts with temporary stores and local fixtures.
