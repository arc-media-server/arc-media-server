> D3 completes the durable original-bundle handoff in [DOWNLOADS-OFFLINE-API.md](DOWNLOADS-OFFLINE-API.md).
> This document records the D2 transport contract; no merged/deployed state is implied.

# Original transfer API — D2 backend handoff

D2 adds `/api/v1/downloads/{id}/manifest` and GET/HEAD
`/api/v1/downloads/{id}/assets/{asset}` on the existing original preparation API (#62).
Use normal profile/session authentication and the administrator's download grant. Foreign records
and asset IDs are404, absent grant403, unready/changed sources409, transfer saturation429 with
Retry-After:3, and maintenance admission503 with Retry-After:3. Owner release stays available.

Manifest returns download_id/revision/server_id/mode, a safe item snapshot and assets with opaque
id, role, server-relative url, media_type, size and SHA256. Authenticate every request; URLs are
not credentials. D2's minimal metadata becomes a durable offline bundle in D3 before advertising
`downloads_original`. `downloads_progress_sync`/`downloads_converted` remain unsupported.

Original GET supports full200 and single byte range206 (including open-ended/suffix). Invalid,
unsatisfiable or multiple ranges give JSON416 and Content-Range:bytes */size. HEAD ignores Range,
returns full metadata and no body, and consumes no transfer slot. Responses are private/no-store,
with exact Content-Length, safe attachment name and Accept-Ranges:bytes. No ETag or Last-Modified
is promised for mutable originals; any If-Range gives full200. Replace partial bytes on200.

Persist the manifest/revision with partial bytes; never concatenate assets from another revision.
Verify the final complete SHA256 and size before publishing locally. A server response does not
prove device storage, decoder support or offline playback. On416 reconcile local length/revision;
on409 release/reprepare. Do not start a new operation merely because a response was lost.

At most one live transfer per owner/session and four server-wide; no playback slot/session/watch
history side effects. Release, session/grant/profile changes, source drift and shutdown terminate
active work. After headers an invalid transfer is aborted, never completed with appended JSON.
The client must handle a truncated/reset connection and retry only after permission/freshness
reconciliation. Tests cover HTTP1.1 and HTTP2. Checkpoints target250ms between available I/O;
blocked filesystem calls and bytes already buffered by a proxy cannot be recalled. A transfer
holds catalog ownership, so online restore returns busy until it finishes/cancels.
