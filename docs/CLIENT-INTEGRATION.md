# Integrating an Arc client

Use `/api/v1` for applications. `/api` is the web app's evolving interface. The complete generated
contract is [openapi.json](openapi.json), the readable reference is [API.md](API.md), and an Arc
server serves its bundled interactive viewer at `/docs/`. New fields and capability keys are additive;
ignore unknown fields. This guide describes this branch; check the deployment's capabilities first.

## Authentication and discovery

Use a Bearer token for native and cross-origin clients. The web app uses an HttpOnly session cookie.
Never put a session token in a URL or log it. Login with `POST /auth/login` (`password`, `device`),
then select a profile with `POST /auth/profile` (`id`, optional `pin`). Friend login selects its own
profile. TV linking uses `POST /auth/link`, displays `code`, then polls `GET /auth/link/{poll}`:
202 means waiting, 200 returns the token once, 404 means expired. See [API.md](API.md).

Read `GET /identity` after selecting a profile: `id` persists across restarts, renames and database
restore; `name` is editable by an administrator. Identity is not proof of trust or a remote relay.
Read `GET /capabilities` on connection/profile changes. Missing feature keys mean unsupported.
`supported` means implemented, `available` means the dependency exists now, and `allowed` describes
role permission. Item/library permissions are still checked on each request.

## Browsing and identifiers

Treat IDs and returned URLs as authoritative. Arc item IDs are strings; file/library IDs are integers;
provider IDs are strings in their own namespace. `GET /items/lookup` resolves existing local items by
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

## Executable read-only examples

Use a fixture/development server and a token whose profile is already selected. These commands do
not start playback or mutate settings. Don't enable shell tracing with credentials in scope.

```sh
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
