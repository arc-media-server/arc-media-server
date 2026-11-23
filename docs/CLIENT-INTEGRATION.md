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
require backoff. Clients providing the approval screen send the displayed code to
`POST /api/v1/auth/link/approve` from an already authenticated, selected profile; household
approval requires a password-authenticated session or an admin. Approval does not itself
return the TV token: the waiting device collects it through its poll identifier. A household token still follows the profile-selection step above; a friend token
already selects that friend. Poll identifiers are limited linking secrets, not general session URLs.

## Browse episodes and alternate orders

There are three separate choices: the library provider supplies show details; **file episode
order** describes on-disk numbering and supplies episode metadata; **display order** changes
browsing/playback sequence. Do not infer identity from a displayed season/episode number or use
provider IDs as Arc item IDs. Ordering IDs are integers; episode item IDs remain opaque strings.

1. Fetch `GET /api/v1/series/<series-id>/orderings`. Use its returned `orderings` IDs, names and
   sources, not a hardcoded provider menu. `source_ordering` is effective file numbering;
   `display_ordering_id` is the effective browse order. `display_choice_id` is the explicit
   selection, or integer 0 when following file order. The reset request below uses JSON null;
   that is different from the returned zero sentinel.
2. Browse `GET /api/v1/series/<series-id>/seasons`. Render both `seasons[].episodes` and `unplaced`;
   episodes outside an order must not disappear. `available:false` is not a playable file.
   To preview another order, use `?ordering=<returned-ordering-id>`; add `missing=1` to include
   provider-known missing episodes. This GET does not save a display choice.
3. If an administrator explicitly chooses a display order, send
   `PUT /api/v1/series/<series-id>/display-ordering` with a returned integer ID:

<!-- example: display-ordering -->
```json
{"ordering_id":12}
```

HTTP200 returns the updated ordering selection. This is an **admin-only server-wide mutation**,
not a personal sort preference. Cross-provider choices may need online episode mapping and take
longer or fail; keep feedback/retry local and refetch the selection after an uncertain result.
Ordinary profiles can browse/preview orders but cannot save this choice.

To return to following the show's effective file order, send the same PUT with:

<!-- example: display-ordering-reset -->
```json
{"ordering_id":null}
```

Refetch seasons and any displayed Next Up/queue state after a successful change. Start playback
with the returned Arc **episode ID**, not its displayed number. For sequential playback,
`GET /api/v1/items/<episode-id>/next` returns the next available episode in the saved display
order, or HTTP204 with no body when there is none. A preview GET does not change this traversal.
Play queues are snapshots with their own lifecycle; an existing queue is not rewritten by an
ordering change. See [PLAY-QUEUES-API.md](PLAY-QUEUES-API.md).

Changing display order does not rename media or change file numbering. Changing file numbering
is a separate admin metadata operation (`PATCH /api/v1/items/<series-id>` with `source_ordering`);
it can rematch known files and change episode metadata. A client that only presents alternate
views should not send that PATCH. See [METADATA-ORDERING.md](METADATA-ORDERING.md) for the full
provider/numbering policy.

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

## Watch Together: enter a room, then run ordinary playback

Check `features.watch_together` in capabilities: require supported, available and allowed before
offering the action. All requests require a selected profile; the participant is that profile
**on this device session**. Use the same device token for room reads, commands, heartbeats and
streaming. An invite code does not sign someone in or grant access to a library.

For the code-based flow, resolve a playable movie/episode, then send
`POST /api/v1/watch-together`:

<!-- example: watch-room -->
```json
{"item_id":"replace-with-returned-item-id","start_ms":0}
```

HTTP201 returns `room`, `invite_code` and `invite_expires_at_ms`. Keep the code private. A second
signed-in device sends `POST /api/v1/watch-together/join`:

<!-- example: watch-join -->
```json
{"code":"replace-with-private-invite-code"}
```

HTTP200 returns its personalized room snapshot. `GET /api/v1/watch-together/current` recovers
this device/profile's room after a reload or lost response; HTTP404 means none. GET
`/api/v1/watch-together/<room-id>/events` is an authenticated SSE stream of personalized **room**
snapshots, separate from the general invalidation stream. On reconnect refetch room state; do
not assume general event cursors/replay apply to it.

Each participant prepares and starts its own ordinary playback, initially paused. One participant
may direct-play while another converts. A room does not allocate a converter or report watched
state; every client still reports personal progress and releases its own playback session.
Send `POST /api/v1/watch-together/<room-id>/heartbeat` about every two seconds with the loaded
`generation`, source `position_ms`, actual `state` and `ready`. Buffering is state `waiting`,
ready false. Do not acknowledge a newer generation until its media/seek is actually ready.

For an explicit user action, POST to `/api/v1/watch-together/<room-id>/commands` using the latest
returned revision (the value below is illustrative):

<!-- example: watch-command -->
```json
{"revision":1,"action":"pause"}
```

HTTP409 includes the current personalized `room`: adopt it and retry only if the user's action
still makes sense. `revision` guards commands; `generation` guards readiness after seeks/item
changes. Never send a command for every progress tick or echo received commands back to the server.
On leaving, POST `/api/v1/watch-together/<room-id>/leave` (204) and stop/release local playback.
Rooms are temporary and do not survive a server restart. This walkthrough does not establish
multi-device timing or physical-player compatibility.

Profile invitations, restrictions/redaction, clock synchronization, host succession and buffering
policy are detailed in [WATCH-TOGETHER-API.md](WATCH-TOGETHER-API.md). These are required integration
reading before shipping a synchronized player; the short flow above is not a complete sync loop.

## Downloads: check support before showing offline controls

Original downloads are implemented on current servers (#62): preparation, offline bundles and
authenticated resumable transfer. Older servers don't advertise them, so always gate on
capabilities. Offline progress sync and converted downloads aren't implemented yet, and no
native app has been accepted on a real device.

Read capabilities after selecting a profile. A missing feature key means unsupported; do not
probe guessed transfer URLs or offer a download merely because ordinary streaming works.
`downloads_preparation` alone is not original-byte transfer. Enable original downloads only when `features.downloads_original` has
supported, available and allowed all true. Ordinary profiles need an explicit administrator
grant. Server availability is not a device free-space/codec guarantee.

The workflow is: choose a returned movie/episode file → create an owned original
preparation → wait for readiness → obtain its manifest → download its returned authenticated
asset URLs → verify assembled size and SHA-256 → publish locally. Bind partial bytes to the
manifest revision. If a resume request returns HTTP200, replace the partial file; never append
it as if it were HTTP206. Do not reuse ordinary playback routes as download APIs: downloads
must not create playback/history side effects.

Client-local storage, background transfers and offline playback remain app responsibilities.
Already copied bytes cannot be remotely recalled. Offline progress synchronization and converted
downloads are separate future work; do not infer either from original-download support.

The [original-download guide](https://github.com/arc-media-server/arc-media-server/blob/main/docs/DOWNLOADS-OFFLINE-API.md)
has the full contract. Check the running server's capabilities and OpenAPI before relying on it.
Repository links need network access; the core guidance on this page is bundled.

## Profile library selection

Profile library_ids:null means **all libraries**; [] means **no libraries**. A nonempty integer
array selects those libraries. Omit the field on PATCH to keep existing access. Do not normalize
null to [] in a client or SDK. Admin profiles always see every library. OpenAPI describes this
nullable array explicitly; minimal profile creation needs name, while PATCH fields are optional.
Profile exclude_labels may also be null; null and [] both mean no excluded labels.

## Empty lists and profile library access

Playback and playback-preview list fields (`audio_tracks`, `subtitle_tracks`, `versions`,
`chapters`, `markers`, `reasons`) are arrays, including empty `[]` values in needs-analysis and
no-video decisions. Optional `reason_codes` can be absent when empty; when present it is an array.

A profile's `library_ids` is deliberately different: `null` means all libraries, `[]` means no
libraries, and a nonempty array selects those library IDs. Do not replace `null` with `[]` when
saving a profile or generating an SDK. This is existing v1 access behavior, not missing metadata.
The recorded v1 type snapshot currently lists the array shape; clients must also accept this
legacy null sentinel. This change preserves that snapshot and the wire behavior.

## Browsing and identifiers

Treat IDs and returned URLs as authoritative:

| Value | Wire type |
|---|---|
| Arc item, profile and queue IDs | Opaque strings |
| File, library and episode-ordering IDs | Integers |
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

## Sync watched state, resume points and dated history

Authenticate with a bearer token for the intended profile. Resolve local items using
`GET /api/v1/items/lookup` and read their `user_data` through item details or batch lookup.
Keep the three kinds of state separate:

- **Watched status.** Use `PUT /api/v1/items/{id}/played` with `{"played":true}` or false.
  It clears the resume point, so apply any saved position afterward. Marking a movie watched
  takes it off the watchlist. Ratings and watchlists use their existing APIs.
- **Saved resume point.** Send `PUT /api/v1/items/{id}/resume` with `{"position_ms":123000}`
  or zero to clear it. This only changes the position. It leaves the last-played timestamp,
  watched state, play count and history alone and creates no playback session.
- **Completed viewings.** Page `GET /api/v1/watch-history` with `?before=<next_before>`
  until `next_before` is absent. It lists entries for visible items, newest by viewing ID
  first, including backdated entries added later. Each entry has `id`, `item_id` and
  `watched_at` in Unix seconds. For native Arc viewings this is the start time.
  Send `POST /api/v1/items/{id}/watch-history` with `{"watched_at":1700000000}`
  to record an original viewing date. Retries at the same second for the same profile and
  item return the existing entry. A different timestamp records another viewing.
  Delete a specific entry with `DELETE /api/v1/items/{id}/watch-history/{viewing}?watched_at=1700000000`,
  using that entry's ID, item ID and timestamp. Repeating a delete is safe. Adding or deleting
  entries leaves watched status, play count and resume state alone.

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
`Retry-After` when present on 429/503. Catalog restore/library removal reserves a quiet
window: active catalog work returns 409 before mutation; new mutation requests during that window
return 503 with `Retry-After`. Retry after it ends; do not discard authentication for a 503. Back off transient reads with jitter and cancellation. Arc
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

### Dolby Vision compatible bases

Source streams optionally expose `dv_bl_signal_compatibility_id` from structured FFprobe
metadata; absence differs from explicit zero. Keep source labels distinct from output capability.
A plan with reason code `dolby_vision_base_layer` uses a known compatible HDR10/HLG/SDR base
when the requested DV profile is unsupported. Repackaging strips DV metadata using FFmpeg 8;
SDR conversion may tone-map a compatible HDR base. Unknown bases remain refused. Older
stored files acquire this evidence through the existing Media analysis task.

### Converted-stream storage windows

Playback requests may opt into `rolling_hls: true`; a converted plan echoes `rolling_hls: true`.
Omit it to retain the existing full EVENT playlist. Opt-in clients receive a sliding HLS playlist:
its media sequence advances as completed segments more than two minutes behind successful segment
requests are removed. Preserve the media timeline. If seeking before the advertised window, POST
a new playback plan at the desired source-relative `start_ms`; never keep retrying a removed
segment. The hls.js web player implements this; native-HLS web playback keeps its existing path.
All converted sessions have best-effort free-space admission/monitoring. A low-space start returns
503; a running conversion fails its playlist instead of silently filling the disk. Native apps
should enable rolling windows only when they implement the older-seek restart behavior.

### Bulk metadata refresh admission

`POST /items/bulk` with `refresh: true` uses one bounded server worker. Repeated pending/in-flight
ids coalesce; the global bound is 5,000 unique titles and JSON bodies are limited to 2 MiB. A full
queue returns 429 with `Retry-After` before that request's synchronous edits are applied; temporary
catalog maintenance returns 503. Accepted jobs appear in Activity and survive HTTP disconnects,
but stop with Activity cancellation, server shutdown or the batch deadline. They are not durable
queued jobs across a restart. Metadata, watched-state and watchlist changes from one accepted
bulk request commit together; invalid or failed writes leave those changes unapplied.

### Profile-visible filesystem details

Media-file `path` remains a string: administrators receive the full server path; household/friend
profiles receive its filename only. Library `paths` arrays are empty for non-admins. Raw probe/scan
filesystem diagnostics are administrator detail; ordinary profiles receive a generic failure.
Use stream URLs/file ids for playback and indexed relative folder endpoints for browsing, rather
than reconstructing server paths. No stored file identity/path is changed by response masking.

### Event stream capacity

Event streams are bounded to eight simultaneous connections per profile as well as 32 per
server. A ninth connection gets `429` with `Retry-After: 5`; close disconnected streams and honor
that backoff rather than repeatedly opening replacements. Sessions/devices using the same
profile share its allowance. Invalid principals do not reserve a slot; disconnect releases it.

### Validate item paging and atomic playlist edits

Item paging accepts an explicit `limit` from 1 to 500 (default 100) and `offset` from 0 to
10000000. Invalid, nonnumeric or oversized bounds return `400`; they no longer silently reset.
Playlist edits validate the entire request before writing. Titles use the same trimmed
100-Unicode-character limit as creation; a bad filter or rules on an ordinary playlist leaves
the title unchanged. A valid combined title/rules edit is saved atomically.

### Return 405 and Allow for unsupported API methods

A request using an unsupported HTTP method on a known API path returns `405` and an `Allow`
header listing supported methods. Unknown paths still return `404`. Do not interpret a 405
as a missing server feature; check the endpoint's documented method.

### Preserve independent concurrent preference changes

Concurrent profile-preference PATCH requests merge against the latest stored preferences in a
transaction. Independent fields no longer overwrite one another from stale snapshots; two
writes to the same field still use the last committed edit. Read request bodies before saving,
and continue using partial patches rather than sending an old complete preference snapshot.

### Prevent pre-roll selection from exposing non-video files

Pre-roll paths select regular MP4/M4V/MOV/WebM/MKV files or folders of them. Non-video files,
file symlinks and special files are ignored; configure a linked video's real path instead.
Configured folder aliases remain supported. These clips are intentionally server-wide assets
and may live outside libraries; a valid profile can play them. This does not validate codecs or
make pre-rolls follow a library's visibility rules.

Action failures distinguish invalid input (`400`) from internal database/filesystem problems
(`500`, generic response with a request ID). Matching's actual upstream provider failures retain
`502`; a server database error is not an invalid match. Missing selected backups return `404`,
invalid/incompatible backup contents return `400`, and failure creating a safety backup is an
internal failure. If restore reports that the database was restored but runtime reload failed,
restart Arc as instructed rather than assuming no restore happened. Keep authentication on
internal/upstream errors; those statuses do not mean a device token was revoked.

## Building tools for Arc

Arc exposes its own stable `/api/v1` API. A tool that only speaks Plex or Jellyfin needs an Arc
integration; these endpoints do not emulate either server.

1. In the web app, an administrator opens **Settings → Access & devices → Devices → New token
   for a tool**, names it and copies the returned token immediately. It is shown once, then appears
   in Devices marked **Tool**, with the ordinary creation/last-used timestamps and revocation.
   Keep it in the tool's protected credential storage. Send `Authorization: Bearer <token>` over
   HTTPS for remote requests, never in a URL or query string. This is an ordinary device token:
   it starts with the creating admin selected, uses existing session checks, and follows the same
   password-change, stale-device cleanup and revocation rules. Last-used bookkeeping is best-effort,
   normally updated at five-minute intervals; busy storage can delay it. Revocation takes effect on
   subsequent requests (401); already admitted work may finish. The equivalent admin API is
   `POST /api/v1/devices/tools {"name":"My tool"}` → `{"token":"..."}`; list with `GET /api/v1/devices`
   and revoke with `DELETE /api/v1/devices/{id}`. Only creation returns the credential.
2. Resolve existing items without rescanning or calling a provider:
   `GET /api/v1/items/lookup?provider=tmdb&provider_id=603&kind=movie`.
   Use `provider=imdb&provider_id=tt0133093` or `provider=tvdb&provider_id=78874&kind=series`
   for the other supported identity types. Use returned Arc IDs for edits. Ordinary sessions can
   use these same read APIs, with their profile's library, age and excluded-label permissions.
3. Add/remove labels without replacing unrelated labels:

   ```http
   POST /api/v1/items/bulk
   Authorization: Bearer <token>
   Content-Type: application/json

   {"item_ids":["<arc-item-id>"],"add_labels":["Weekend"],"remove_labels":["Review"]}
   ```

   This existing admin bulk operation commits edits in one short write transaction. It supports
   `add_genres` and `remove_genres` too. Labels never lock metadata. A label in both lists is
   removed (removal follows addition). At most 5,000 movie/show IDs and 2 MiB per request;
   omitted fields stay unchanged. `GET /api/v1/items?label=Weekend&label=Favourite` matches **any**
   supplied label, case-insensitively, within all other filters and the profile's access rules.
   Excluded labels still win, even when explicitly requested. Discover label names/counts through
   `GET /api/v1/libraries/{id}/facets` (`labels`); hidden titles do not contribute.
4. Make a collection with `POST /api/v1/collections`:
   `{"library_id":1,"kind":"manual","title":"Weekend","item_ids":["<arc-item-id>"]}`.
   Add existing titles with `POST /api/v1/collections/{id}/items {"item_ids":[...]}`; see the generated
   reference for smart collection filters and poster upload endpoints. Reuse returned collection
   IDs rather than creating a new collection every run.

Tool tokens start as full administrator credentials, not per-library or read-only grants. Revoke a token
when retiring a tool; issue a replacement and update its credential store to rotate it. Never
print tokens in diagnostics or enable credential-bearing shell traces. All examples use placeholders.
