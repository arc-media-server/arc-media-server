# Original downloads — backend handoff for native clients

Implements original movie/episode preparation, durable offline bundles and authenticated resumable
transfer for #62. Use `/api/v1` and the generated OpenAPI reference. D1/D2/D3 are stacked review
PRs; this guide does not mean they are deployed on main. Offline progress sync and converted
media are separate future capabilities. No native controls or physical-device acceptance included.

## Discover and select

`GET /capabilities` adds `downloads_original` with supported/available/allowed axes. Available means
the bundle manager is configured, not that disk space/source media is guaranteed. Preparation
remains separately advertised. Missing `downloads_progress_sync`/`downloads_converted` means
unsupported. Admins are allowed; other profiles need the admin-managed `allow_downloads` grant.

Use the existing accessible item/file API to select an explicit original file ID. POST/downloads
still accepts item_id/file_id/mode=original/idempotency_key, with no path or caller-supplied owner.
Multipart and shared-episode files remain unsupported422. An original retains all embedded tracks;
its stored HDR/audio facts do not prove the device can decode/output them. No conversion occurs.

Records are bound to profile AND authenticated device session. A new token is a new scope; there
is no cross-token adoption. Normal session/Bearer authentication is required for every route,
including asset URLs. Store credentials in the platform credential store, never in URLs or logs. Retain registered download IDs until explicitly released, including
failed/unavailable operations; this slice has no bulk release or abandoned-record expiry API.

## Prepare and read a manifest

POST/downloads is durable and idempotent within that scope:201 new,200 replay,409 changed selection
under the same key. Poll the descriptor with backoff. State is preparing/ready/failed; ready adds
SHA256. A failed record needs release and a new operation. The server hashes in background and
pauses hashing during foreground playback. GET/HEAD never starts a hash, probe, scan or provider call.

`GET /downloads/{id}/manifest` returns200 only for current ready preparation with its stored bundle.
Its fields are:

| Field | Meaning |
|---|---|
| download_id, revision | Opaque attempt identity. Persist with partial/local files. Never mix attempts. |
| server_id, profile_id | Bind local records to the correct server/profile. A database restore is not proof of fresh media. |
| mode | `original` only. |
| item | Explicit prepared snapshot: ID/kind/title, optional overview/year/series ID/title/season/episode, duration_ms/container/streams. Unknown facts remain unknown. Episode numbers use the server display order at preparation time. |
| assets | Original media plus supported available artwork/caption snapshots: id/role/url/media_type/size/sha256 and optional language/forced/hearing_impaired. URLs are server-relative and authenticated. |
| unavailable | Known omitted optional assets, with role/reason_code and optional language. Do not infer that every possible sidecar format has been discovered or supported. |

The title/technical/numbering snapshot is fixed for that preparation, independent of later metadata
edits. No host paths, provider URLs, session hashes, idempotency keys or private source keys.
Manifest assets are immutable attempt-owned copies except the original media, which remains on the
source disk. Source freshness and final client integrity verification remain necessary.

Supported small assets: uploaded/local/already-cached poster/backdrop/logo and up to eight discovered
external SRT/WebVTT text captions. Preparation never fetches artwork/providers or extracts captions
with FFmpeg. Original embedded tracks remain unchanged. ASS/SSA, image/unknown external formats,
unavailable cached artwork and oversized/non-UTF8 text assets are explicitly unsupported/unavailable.
Clients choose among the actual returned assets; there is no server-side track conversion/selection.

Internal bounds:5MiB per artwork,2MiB per caption,24MiB per prepared small-asset bundle,
256KiB serialized metadata and256MiB logical derived-asset storage budget. One temporary bundle
can be staged before the global publication check. These are internal safety bounds, not an
ordinary settings screen or a device free-space guarantee. Release unused records to reclaim
Arc-owned snapshots; no original source is removed. Budget/storage failure leaves preparation
failed/bundle_unavailable; it does not publish a partial ready manifest.

## Transfer and resume

GET each asset URL with the same token. Full200, single-range206; open-ended/suffix ranges work.
HEAD returns equivalent full headers without a body and ignores Range. Content-Length/type,
Accept-Ranges:bytes, safe generated attachment name and private/no-store are supplied.
Malformed, unsatisfiable or multiple ranges return JSON416 with Content-Range:bytes */size.

No ETag or Last-Modified resume validator is promised. Any If-Range returns full200: replace the
partial file, never append that response. Plain Range can resume the exact same manifest asset.
The server does not silently replace an asset with a newer revision. If a source changes, obtain
fresh preparation rather than concatenate different representations. HEAD does not validate a
client's complete digest or establish device-local completion.

Always verify full assembled byte count AND SHA256, then atomically publish the local file.
A digest mismatch discards/reprepares; it can occur when external software edits source bytes
without changing all detectable metadata. Repeated ranges and successful responses are not a
truthful global completion percentage. Track queued/downloading/paused/verifying/available locally.

One live transfer per profile/session, four server-wide; saturation429/Retry-After:3. Transfer does
not create now-playing, consume a playback slot, change progress/history or start FFmpeg. Disk/network
contention can still affect playback; no throughput or production playback-performance claim.

## Errors, revocation and recovery

401 normal invalid/unselected session;403 withdrawn/missing grant;404 unknown/foreign/hidden record
or asset.409 unready/changed source, absent snapshot or missing/changed stored asset: release and
prepare a new operation if needed.503/Retry-After:3 for maintenance admission. Ordinary database
failure follows retryable server errors, not an invented credential failure.

After headers, source/access failure aborts the stream; there is no appended error JSON. Handle
truncated bodies/reset streams and recheck descriptor/permissions before resuming. Server checks
target250ms between available operations and can interrupt blocked socket writes. Blocked OS
read/stat operations and downstream/proxy-buffered bytes cannot be recalled. Already copied media
cannot be remotely erased; this is not DRM.

DELETE/downloads/{id} releases server bookkeeping and derived copies after active reader leases
finish, and interrupts active work. It never deletes source or client-local media. Release remains
allowed to an authenticated owner whose grant was withdrawn. Session/profile/library deletion,
FK cascades, restart and restore trigger root-owned orphan reconciliation. Cleanup runs periodically,
not inside each request. A database-only backup does not contain derived files; missing copies give
409 and need reprepare. Pending work restarts safely; no partial snapshot is promoted to ready.

Preparation, transfer and cleanup retain catalog ownership. Online restore/destructive maintenance
returns busy until active work finishes/cancels; long transfers can postpone restore. Store closure
waits for actual owned work. Migrations0069/0070 invalidate earlier unbound/no-snapshot records,
retaining their IDs but clearing digests and requiring release/new preparation. Reconcile migration
numbers with the final merged main before shipping the stack.

## Claude's client work

1. Gate controls on the implemented capability/grant and ordinary per-item/file availability.
2. Register an operation; persist server/profile/revision and poll with backoff.
3. Save manifest and partial paths before starting authenticated background transfers. Check free space.
4. Resume only the same asset; replace on200, reconcile on416, reprepare on confirmed stale409.
5. Verify complete size/digests and publish locally; retain metadata/artwork/captions for offline browsing.
6. Test actual device suspension/kill/relaunch, airplane-mode playback/captions, storage exhaustion,
   profile switching/sign-out and reconnection. Hide profile-scoped local media on sign-out and offer
   local deletion. Do not promise remote deletion, decoder/HDR output or OS background execution.
7. Keep offline watch-state local until a separately implemented versioned sync API exists. Do not
   send delayed offline events through live playback/progress routes or invent online watch history.

API field additions are additive. The generated reference and tests take precedence over the old
broader proposal. Server fixtures do not establish iOS/Android/tvOS acceptance or close #62.
