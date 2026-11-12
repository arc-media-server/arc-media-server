# Original-download preparation — backend/client handoff

First runtime slice of #62, built independently of the unmerged #418 server-review work. SQLite
only; PostgreSQL remains deferred. The owner explicitly authorized parallel implementation on
11 November, with reconciliation/retesting after the review PRs merge. The broader design is in PR #420. This guide describes the
implemented preparation API; the future byte/manifest/offline-sync API in that proposal is not
implemented here. Use `/api/v1` and the actual generated [OpenAPI](openapi.json).

## Capability and permissions

`GET /api/v1/capabilities` adds `downloads_preparation`: supported indicates these routes exist,
available indicates a preparation manager is attached, allowed is this profile's permission.
There is deliberately no downloads_original, downloads_converted or downloads_progress_sync
capability yet. Do not add a working Download button or promise offline playback from this slice.

Profile replies add `allow_downloads`. Administrators always report true; ordinary profiles
start false. Admin-only POST/PATCH `/profiles` accepts optional allow_downloads to grant/revoke it.
An ordinary profile cannot grant itself access. The grant controls Arc's download workflow;
normal playback already delivers copyable bytes and this is not DRM. No frontend permission control is included; Claude
can add one later using this existing profile API. Omitting the field preserves an existing grant.

All download records belong to the selected profile AND server-issued authenticated session.
Reuse the same normal token; a device's display name is not identity. No session hash, path,
idempotency key or URL token appears in a descriptor. Profile switching cannot read another
profile's records. A newly issued token is a new scope; cross-token adoption is not implemented.

## Routes

| Method/path | Result |
|---|---|
| POST /downloads | 201 new durable original preparation, 200 identical replay. No hash runs inside the HTTP request. |
| GET /downloads?limit=50&offset=0 | 200 items/total/offset for visible owned records, stable created_at/id order. Limit 1–100, offset 0–10000; invalid/duplicate bounds are 400. |
| GET /downloads/{id} | 200 descriptor. Ready sources are stat-checked; detected external changes return 409 before the next scan. |
| DELETE /downloads/{id} | 204 releases an owned record and interrupts preparation at its next check. 404 unknown/foreign/already released. No source/local device files are removed. |

The full prefix is `/api/v1`. These are JSON endpoints; no manifest/asset/byte/range/download
completion route exists yet. GETs never probe, hash or start a converter. Lists describe the last
preparation state, not an OS-level guarantee of source availability; fetch a descriptor before
using it. Responses are private/no-store.

Create body:

```json
{"item_id":"returned-movie-or-episode-id","file_id":42,"mode":"original","idempotency_key":"client-operation-1"}
```

Select a file ID returned for that exact accessible movie/episode. Multiple versions can be
selected explicitly. Multipart titles and files shared by multiple episodes are rejected with
422; this slice does not silently prepare half a movie or several episodes as one episode.
Unknown/missing/foreign file selections give 404; series and unsupported modes give 422.
JSON bodies are bounded to 4096 bytes and reject unknown fields/trailing JSON. Oversize is 413.
Item IDs are nonempty/up to 200 bytes; keys nonempty/up to 80 bytes with no edge whitespace.
Keys are operation IDs scoped to profile/session, separate from diagnostic request_id.

Example descriptor (illustrative values):

```json
{"id":"download-example","item_id":"returned-movie-or-episode-id","file_id":42,"revision":"attempt-example","state":"preparing","size":1048576,"created_at":1794398400}
```

Poll every few seconds with backoff while preparing. Ready adds a 64-character lowercase SHA-256;
failed adds a bounded reason_code: source_unavailable, source_changed or source_or_access_changed.
A digest describes the bytes read for that source revision. It is not a strong HTTP ETag or proof
that the source is immutable/playable, a transfer completed, or the device can decode HDR/audio.
No physical compatibility guarantee is made.

Identical retries return the same record while it exists, including its ready/failed state. Reusing
a key for a different item/file gives 409. To retry a failed/stale preparation, release its record
and create a new operation with a new key/revision. DELETE removes the idempotency record too;
after release an old create key can create new work, so do not retry an intentionally released
operation. No indefinite idempotency tombstones or client-local receipts exist in this slice.

There are at most 100 records per profile/session and 2000 per server, with atomic admission.
Saturation returns 429/Retry-After:60; release unused records rather than repeatedly retrying a
full scope. These are backend bounds, not a new user-facing worker-settings screen.

Invalid/unselected authentication follows existing middleware. No grant gives 403 before item
lookup; inaccessible/foreign records give 404. Storage/DB handler failures use the normal error
boundary. Release remains allowed for a signed-in owner after its grant/library access is withdrawn,
so denied records do not consume the owner's quota permanently. Session deletion cascades records.

## Preparation and cancellation

One root-owned worker reads original files in 256KiB chunks and computes SHA-256; no source is
written and no duplicate full copy is retained. It pauses for foreground active playback. State
stays preparing during that pause. A periodic check (250ms target between completed I/O operations/pause waits)
revalidates owner/session/grant, visibility, file linkage and saved source metadata. Cancellation
also checks between chunks. An already blocked filesystem read/stat cannot be interrupted by a Go
context; a stalled NAS can exceed these check intervals. No hard filesystem/shutdown latency bound
is claimed.

Before/after hashing the worker checks the opened regular file's identity, size and nanosecond
mtime and the final path identity. It publishes no digest after a detected change. Metadata-preserving
external edits are not proven absent: the eventual transfer phase must verify the complete assembled
file digest and handle changed sources. Do not use size/mtime as a strong If-Range validator.

Pending records survive restart and hash again from the beginning; partial digests are never saved.
The worker is cancelled/joined before the DB closes. Failure to read/update the queue backs off for
one minute. The API does not create detached worker goroutines. Deletion/old attempts cannot recreate
released rows. Records/grants are part of SQLite backups; no generated media artifacts are created
by preparation. Restored records must revalidate source state; a backup is not a media copy.

Preparation does not call stream/playback/progress and creates no play-history/now-playing entry,
conversion, provider request, probe, rescan or watch-state change. No offline progress sync yet.

## Logging

One preparation start and success outcome with safe ID/bytes/duration. Detected source/access
failures get one outcome warning with a bounded reason code. Expected delete/cancel/interruption
is debug-only; no chunk/hash/progress/poll info log. Bookkeeping/queue failure warnings back off
with retry (one minute). No source paths, raw filesystem/SQL errors, keys, tokens, bodies, URLs or
idempotency keys in worker logs. API failures reuse normal diagnostics and bounded request stages.

## Integration and remaining phases

- Rebase/reconcile after reviewed #418 merges and rerun checks before claiming combined acceptance. Migration0068 follows its
  reserved0064–0067; this branch has not imported them. Test both fresh creation and upgrades from
  databases created by each branch. Incorporate the new file cache_key/source identity and common
  root-worker lifecycle where appropriate; current provenance does not use that pending column.
- Reconcile generated API/profile changes with #419. Preserve all existing v1 promises and the
  profile null/all/none corrections there. Keep owner PostgreSQL deferral notes from #419.
- This branch retains main's existing auth middleware; #418's retryable sign-in DB-error fix and
  broader security fixes are not imported or claimed here.
- Next implement authenticated manifest/asset delivery with range/integrity/revocation tests,
  then versioned idempotent offline progress sync. Only then wire real native download controls.
- No native repo, Mac signing, external database, production data/server or physical-device test
  is part of this backend slice. Original/converted transfer and native acceptance remain open.

## Source identity and restore integration (D1)

Download provenance privately binds to the persistent media-file cache key as well as path/size/mtime.
A reused row ID or changed saved source key cannot reuse preparation. Migration0069 keeps old
records but marks them failed/source_changed and clears their digest: the old schema cannot prove
that binding. Release and prepare a new operation. The key is never exposed in API replies/logs.

A preparation worker reserves catalog work before capturing its pending record and retains that
reservation through publication. Online restore/destructive maintenance remains busy while it runs;
maintenance admission contention pauses the worker without failing records. Direct synchronous
preparation uses the same guard. Root shutdown joins it before store closure. An uninterruptible
filesystem read can still delay safe shutdown. Download reads use a narrow source/access projection;
no full media-stream hydration is needed for periodic checks. No transfer endpoint is added by D1.
