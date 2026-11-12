> Implemented subset: see [DOWNLOADS-OFFLINE-API.md](DOWNLOADS-OFFLINE-API.md) for the
> stacked original-transfer implementation. This broader proposal still includes unimplemented
> offline-sync/converted functionality; do not treat this entire draft as a live contract.

# Downloads API — proposed client handoff

**Not implemented.** This is groundwork for #62, dated 11 November 2026. Do not call these routes
on current Arc, add them to the generated OpenAPI document, or ship client controls based on this
proposal. Use [the implementation plan](plans/DOWNLOADS.md) for decisions, dependencies and tests.
The eventual generated contract from actual handlers takes precedence. All paths below are proposed
under `/api/v1`; requests require the selected profile's normal session Bearer token.

## Native client responsibilities

Bind local records to server ID/profile/download revision; keep the normal session token in the
platform credential store. Never put it in a media URL, downloaded manifest or diagnostic log.
Use the OS download manager only if it can authenticate/resume safely; do not create public token
links to accommodate one. Pause/relaunch/network retry and device free-space checks belong to the
client. Advertise download compatibility only after verifying the selected file's local decoder.

Separate server preparation from device transfer status. Store partial bytes outside the playable
library, verify the complete manifest digest, then atomically publish locally. Keep enough metadata,
artwork and supported captions locally to browse/play without Arc or a provider. Persist pending
watch-state events before reconnecting. No server response can remotely erase a copied original.

## Proposed discovery

Missing capability keys mean unsupported. Introduce `downloads_original` only when original
preparation AND transfer work together, `downloads_progress_sync` only after reconciliation is
implemented, and `downloads_converted` only after complete-file compatible preparation exists.
Follow Arc's supported/available/allowed axes. Capabilities are hints; each request rechecks access.

`GET /items/{item-id}/download-options` returns saved source choices and implemented modes;
no scan, provider call, file hash or FFmpeg start. File IDs are integers, item/download/asset IDs
opaque strings. Technical data describes source properties, not proof of device HDR/audio output.
A missing required analysis fact must be reported explicitly; no guessed codec compatibility.

## Proposed endpoints and status rules

| Method/path | Contract intent |
|---|---|
| POST /downloads | One item/file/mode; create durable preparation. 201 new record; 200 identical idempotent replay; 409 reused key with different body. |
| GET /downloads?limit=50&offset=0 | Owner profile/session only; stable created_at/id order; items/total/offset, limit 1–100; validate invalid bounds. No internal paths or foreign counts. |
| GET /downloads/{id} | Owner record/state/revision, 200. Foreign/hidden/missing are indistinguishable 404. |
| GET /downloads/{id}/manifest | 200 only when ready/current/authorized; 409 while preparing or detected stale, 410 for an owned expired record. No readiness change or expensive work on GET. |
| GET or HEAD /downloads/{id}/assets/{asset-id} | Authenticated, authorized original/media/sidecar/artwork bytes; range/size/type semantics below. No playback tracking. |
| DELETE /downloads/{id} | 204 cancels/releases owned record/server work; it does not delete the client's local file. Hidden/unknown gives 404; clients can treat an already absent record as released. |
| POST /downloads/progress-sync | Later phase: bounded offline event batch; 200 per-event outcomes, not a fake live progress report. |

Normal auth errors follow the existing API contract: 401 for invalid/unselected session, 403 for a
valid profile without the download grant, 404 to hide inaccessible item/file/record IDs. Storage
or database outages are retryable server errors, not invalid credentials. 400 invalid JSON/fields,
413 oversized body, 422 unsupported selection/mode, 429 bounded admission with Retry-After.
Safe error code/request_id fields follow current server middleware; preserve unknown future codes.
Examples below are illustrative JSON; they are not live request/response fixtures yet.

### Create an original download

```json
{"item_id":"example-episode","file_id":42,"mode":"original","idempotency_key":"client-operation-1"}
```

idempotency_key is a client operation identifier scoped to owner/session, separate from the
server-issued diagnostic request_id. Store a canonical request fingerprint. The same key/body returns the same active attempt even after
a lost response/restart. A changed selection needs a new key. Token is authenticated normally;
never accept user_id, session_hash, owner_device or a filesystem path supplied by the client.

```json
{"id":"download-example","revision":"representation-example","state":"preparing","item_id":"example-episode","file_id":42,"mode":"original","created_at":1794398400}
```

state describes server work only: preparing, ready, failed, cancelled, stale or expired. The client
has its own queued/downloading/paused/verifying/available state. Error text must be safe; stable
reason codes, e.g. source_changed, unsupported_selection, storage_unavailable, are more useful
than parsing worker/FFmpeg errors. Final enums are fixed alongside implemented tests.

### Ready manifest shape

```json
{"download_id":"download-example","revision":"representation-example","server_id":"server-example","item":{"id":"example-episode","kind":"episode","title":"Example episode","series_title":"Example series","season":1,"episode":2},"mode":"original","assets":[{"id":"media-example","role":"media","url":"/api/v1/downloads/download-example/assets/media-example","media_type":"video/mp4","size":1048576,"sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}]}
```

Expand the bounded item snapshot with recorded duration/technical/ordering details and optional
artwork/caption asset entries during implementation. Never embed the full unrestricted item DTO
(which can include paths), provider URLs or authentication state. Asset URLs are server-relative,
require normal authorization and do not bypass permission or revocation checks.

## Byte protocol

- GET returns 200 for full bytes, 206 plus Content-Range for a satisfiable single range; invalid
  or unsatisfiable ranges get 416 with bytes */size. HEAD reports equivalent metadata without a
  body or playback work. Return accurate Content-Length, Content-Type and Accept-Ranges.
- Asset paths are opaque lookups, never filesystem paths. Use sanitized Content-Disposition
  filenames derived by Arc, preventing separator/control-character/header injection.
- Every request validates ownership, current access, preparation revision and source freshness;
  actively streaming responses must stop after revocation/shutdown within the tested bound.
- For original live sources, preserve the manifest's representation ID in client state and
  request record identity; reject detected source drift. Do not claim a strong ETag just from
  size/mtime. Resume with Range only against that same manifest and verify SHA-256 over the final
  assembled bytes. A checksum failure discards/re-prepares; never append another revision.
- Only an immutable prepared artifact may promise the digest-based strong ETag/If-Range behavior.
  With a mismatched If-Range it serves full 200; the client truncates its partial file before using
  that body. Match/don't-match, 416 and mid-transfer failure require real regression coverage.
- Download transfer never touches now-playing, watch history, live playback slots or conversion
  session URLs. A user's later offline playback is tracked locally, then synced separately.

## Proposed reconnect sync

Only advertise this after all online watch-state writers participate in revision tracking.
The manifest/download descriptor should include the last known per-item watch-state revision.
Clients store their exact baseline and queue one item's changes in order. Example batch:

```json
{"events":[{"event_id":"offline-operation-1","download_id":"download-example","base_revision":7,"action":"progress","position_ms":63000,"duration_ms":1200000}]}
```

Actions proposed: progress, mark_watched, mark_unwatched. Validate action-specific fields and
nonnegative bounded numbers; do not accept arbitrary user IDs or client wall-clock ordering.
Apply at most 100 events, with ordered per-item processing and atomic per-event state+receipt.
Do not make the entire mixed-item batch all-or-nothing after valid envelope parsing.

```json
{"results":[{"event_id":"offline-operation-1","outcome":"applied","revision":8,"state":{"position_ms":63000,"played":false}}]}
```

Outcomes: applied, duplicate, conflict, unavailable or resync_required. Conflict includes current
permitted state/revision; unavailable reveals no hidden title/path. Duplicate returns the original
applied revision/state without incrementing play counts. Process acknowledgements in per-item order;
an old duplicate receipt must not replace newer cached state or regress the client's known revision. Same event ID/different payload
is a conflict. Keep duplicate receipts keyed by profile, with a bounded documented retention and
explicit resync for events outside that window. Reauthentication never grants access to another
profile's receipts. Exact retention is an implementation decision requiring tests before release.

A conflict is not automatically resolved by time or larger playback position. Keep the event until
the user chooses; Apply this device creates a new event against the returned current revision.
Don't reapply an event using a new ID merely because its first response was lost. Detailed offline
watch-history sessions are outside the first sync release. DELETE/release a download only once
pending state has synced or the user explicitly discards it.

## Handoff checklist for client work

- Wait for the relevant backend phase PR and generated contract; the table above is not deployed.
- Use capabilities and ordinary error handling; keep controls absent on unsupported servers.
- Implement explicit download/delete, storage preflight, resume/digest verification, offline metadata
  and player-track selection before automatic series rules or retention features.
- Test actual device background suspension/kill/relaunch, airplane-mode playback, low space,
  interrupted transfer, sign-out/profile switch and reconnect conflict UI. Server fixtures alone
  do not establish iOS/Android/tvOS storage or decoder support.
- Report which platform/device/version is verified; no client changes, signing or Mac provisioning
  are part of this groundwork. Coordinate app screens with Claude after the contract is implemented.
