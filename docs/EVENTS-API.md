# Live change hints (app handoff)

`GET /api/v1/events` accepts the same cookie or Bearer authentication as other profile endpoints.
Use an SSE parser over authenticated fetch for native apps; never put tokens in query parameters.
The web app can use cookie-authenticated EventSource. This PR adds no UI integration.

Events are `invalidate` with JSON `{ "topic": "profile" }` (optionally `library_id`), or `resync`.
Invalidate means refetch the existing appropriate API, not apply a record patch. Topics currently
include `catalog`, `library`, `profile`, `playback`, `sessions`, and `activity`.
Personal mutations target their profile; library hints respect current library permissions;
activity/session hints are admin-only. No item bodies, names, paths, or user identifiers are sent.
Generic catalog hints can mean that visible data *might* have changed, not that a specific item exists.

Keep the SSE `id`, reconnect with `Last-Event-ID`, and handle `resync` by refetching client state.
Replay is process-local (256 retained hints); initial connection, restart, stale cursors and overflow
require resync. Changes are coalesced until a consumer reads (every 500 ms). This is deliberately not
an audit log or guaranteed per-operation delivery. Continue periodic/refocus refresh as a fallback:
external database writes and some autonomous metadata/asset changes have no producer hook yet.

The server hooks successful API mutations, scan completion, and activity changes. Read-only POSTs
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
