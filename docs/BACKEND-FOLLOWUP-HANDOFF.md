# Backend follow-up: Claude/app handoff

Three independent PRs start from main `ad2f538` (3 November 2026). Merge through GitHub; no
frontend work is included apart from waiting correctly for an existing playback-speed event
in a browser test. Watch Together itself is separate PR #333 and not a dependency of these PRs.

## 1. Conversion throttling — PR #334

https://github.com/arc-media-server/arc-media-server/pull/334

Fast conversions react to playlist updates/player progress instead of depending only on a
periodic timer. A completed conversion is no longer reported as throttled. No API field,
quality decision, HLS URL or player timeline contract changes. Apps continue using the existing
playback response and `offset_ms`; no UI changes are required. See `CONVERSION-THROTTLING.md`
in that PR for fallback behavior, diagnostics, actual test results and limits.

## 2. Response contracts — PR #335

https://github.com/arc-media-server/arc-media-server/pull/335

Generated OpenAPI now describes fixed-key response envelopes, simple helpers, optional branch
keys and nullable collection/pointer values. The v1 guard also protects nullable children and
typed dictionaries. Auth status/remote check use equivalent typed DTOs preserving runtime JSON.
No endpoint migration is required. Regenerate native models if useful; preserve null/optional
handling and allow additive fields. See `API-CONTRACT-COVERAGE.md` in that PR for exact limits,
including deliberately opaque nested diagnostics. No product frontend changes are needed.

## 3. Background change hints — this PR

Use the existing `GET /api/v1/events` stream to refresh stale screens after autonomous work.
The stream now receives worker updates that API mutation hooks alone could miss. Additive
`artwork`, `optimized`, `previews`, `markers` and admin-only `maintenance` topic values accompany
existing `catalog` hints. There are no new endpoints, settings, payload fields or dependencies.

Example SSE frame:

```text
id: <opaque-cursor>
event: invalidate
data: {"topic":"catalog","library_id":12}

```

See `EVENTS-API.md` for the topic/action table, authentication, replay/bounds and permission
rules. Integrate once in the app's shared authenticated state layer; debounce visible-query
refreshes, close/reconnect on profile changes, persist the cursor for reconnects, and perform a
full permitted-state refetch on `resync`. Global hints contain no title/library identity and
must never grant access. An event marks possible changes, including partial failure/cancellation;
read normal APIs for actual state. Do not show a success toast or restart playback just because
an invalidate arrived. Keep existing periodic/refocus refresh as a fallback.

For artwork, retry failed visible images/palette requests; don't invalidate working immutable
images on every hint. For optimized versions, ordinary profiles refresh permitted item/options
queries; the server-wide optimized-list endpoint remains admin-only. For markers/previews,
refresh visible supporting data without changing the active media stream.

This is best-effort process-local invalidation, not durable CDC or a complete per-record event
log. Private Watch Together events remain separate. No app subscriptions, real-device behavior,
reverse-proxy streaming, owner deployment or sustained-load acceptance is claimed.

## Validation and diagnostics

Prepared Go 1.27.0/FFmpeg and local generated/temporary fixtures only; owner data and production
services were not used. Each PR records its own checks. #334 passed the full Go suite/vet,
transcode race tests, embedded build, E2E TypeScript and five selected browser journeys after
fixing a test synchronization race. #335 passed full Go/vet, focused API/apidoc race checks and
embedded build. The background PR records its actual final checks in its change note.

No client/frontend or API schema changes are needed to consume background topics. Worker
publications add no new I/O or goroutines and use existing bounded coalescing/replay. Existing
worker outcome/failure logs remain; no per-item/segment/heartbeat info flood or sensitive
payload logging is added. Native apps and hardware/mixed direct/transcoded room playback still
need their own acceptance checks.
