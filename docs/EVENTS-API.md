# Live change hints (app handoff)

`GET /api/v1/events` accepts the same cookie or Bearer authentication as other profile endpoints.
Use an SSE parser over authenticated fetch for native apps; never put tokens in query parameters.
The web app can use cookie-authenticated EventSource. This PR adds no UI integration.

Events are `invalidate` with JSON `{ "topic": "profile" }` (optionally `library_id`), or `resync`.
Invalidate means refetch the existing appropriate API, not apply a record patch. Topics currently
include `catalog`, `library`, `profile`, `playback`, `sessions`, `activity`, `artwork`,
`optimized`, `previews`, `markers`, and `maintenance`.
Personal mutations target their profile; library hints respect current library permissions;
activity/session/maintenance hints are admin-only. No item bodies, names, paths, or user identifiers are sent.
Generic catalog hints can mean that visible data *might* have changed, not that a specific item exists.

Keep the SSE `id`, reconnect with `Last-Event-ID`, and handle `resync` by refetching client state.
Replay is process-local (256 retained hints); initial connection, restart, stale cursors and overflow
require resync. Changes are coalesced until a consumer reads (every 500 ms). This is deliberately not
an audit log or guaranteed per-operation delivery. Continue periodic/refocus refresh as a fallback:
external database writes and unwired subsystems have no producer hook. These are best-effort
cache hints, not a durable change feed.

The server hooks successful API mutations, scan completion, activity changes and the background
workers listed below. Read-only POSTs
are excluded. Session validity/profile/library permissions are rechecked during delivery and at
10-second heartbeats; revoked/switched sessions close. Reconnect streams after 30 minutes.
There are at most 32 simultaneous streams per server (429 with Retry-After: 5), 128 pending
coalesced hints and 256 replay hints; writes have 5-second deadlines. Slow clients never block
producers on network I/O. Proxy buffering must be disabled (X-Accel-Buffering: no is supplied).

Logging: reuse request failure/debug records; no successful event, heartbeat or progress info logs.
Tests cover replay/coalescing/bounds/concurrency, permission filtering and authenticated HTTP
streaming with revocation. No owner deployment, native client, reverse proxy or sustained-load
validation is claimed.

Cross-origin clients use the versioned API with Bearer authentication. Its CORS preflight permits
`Authorization` and `Last-Event-ID`; responses expose `X-Request-ID` for diagnostic correlation.
Cookies/credentials and local-network login bypass remain disabled cross-origin. Cursor length is
limited to 80 characters. Parse complete SSE frames and retain their IDs; comments are heartbeats.
Unknown topics should trigger a safe refetch or be ignored without breaking the stream. Typed
`invalidate`/`resync` payloads are described by `x-arc-sse-events` in the OpenAPI operation; the HTTP
body itself remains `text/event-stream`. No tokens, user IDs or admin flags appear in those payloads.
See [CLIENT-INTEGRATION.md](CLIENT-INTEGRATION.md) for a bounded curl example and error handling.

## Background producers and client actions

| Topic | Background producer | Suggested app response |
| --- | --- | --- |
| `catalog` | Movie metadata, asynchronous series identification/completion, file inspection/reanalysis, scheduled metadata/analysis/trash completion, OMDb critic updates | Debounce and invalidate visible Home/library/item/technical-info queries. Use `library_id` when present; otherwise refresh only currently relevant permitted views. |
| `artwork` | Successfully installed artwork downloads; pruning that removed files | Retry failed visible image loads and artwork/palette queries. Existing working immutable artwork URLs need no cache busting. |
| `optimized` | Conversion claim, completion, interruption/failure and deletion | Refetch permitted item/playback-options queries; admins may refresh the server-wide optimized list. Do not restart active playback. |
| `previews` | Background preview generation attempt finishes | Refresh visible preview/trickplay availability as needed. |
| `markers` | Background marker generation attempt finishes | Refetch relevant visible marker data. |
| `maintenance` | Scheduled background work finishes, including database/cache upkeep | Admins refresh maintenance status. Existing `activity` hints cover progress/task changes. |

All new events retain the same SSE envelope; no record bodies, filenames, provider URLs, artwork
hashes or job errors are sent. Some producers know only a global topic. Those coarse hints do
not imply that a hidden library/item is present. Never infer permissions or request an admin API
from a global hint. `library_id` hints pass the existing current-library authorization filter.

A completion hint is published after work returns/persists its result, including failed or
cancelled attempts that may have committed partial changes. It is not a success notification;
read the normal API for status/errors. Multiple writes can collapse into one hint. Metadata may
produce an early identification hint and a later episode/order-completion hint. There is no
promise of one hint per file, job or stage, ordering between topics, or exact changed-item IDs.

Clients should batch invalidations rather than eagerly refetch every item after every hint.
Unknown topics can be ignored; keep refocus/periodic refresh and handle `resync`. No client UI
subscription is implemented here. Room synchronisation uses its own separately authorized
protocol; these events must not carry private Watch Together state.

Producer calls only update the existing bounded in-memory hub. They add no database reads,
network writes, polling goroutines or info-level logs. Publication still takes the hub's short
mutex and formats a coalescing key; no throughput claim is made without measurement. Existing
worker failure/outcome diagnostics remain responsible for errors. Tests cover actual async
metadata commits, image success/cache-hit/failure/prune, critic updates, job outcomes, conversion
claim/interruption/resume/failure, and existing authenticated SSE/revocation/privacy behavior.
