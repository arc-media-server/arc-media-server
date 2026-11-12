# Downloads backend foundation

Status: **design and proposed contract only**, 11 November 2026. Related: #62.
No download endpoint, capability, database migration or worker is implemented by this document.
The app contract is [DOWNLOADS-API-DRAFT.md](../DOWNLOADS-API-DRAFT.md); treat it as a proposal,
not the generated v1 reference or a promise that current servers support these requests.

## Goal and release boundary

Let a native client save a movie or episode, play it without contacting Arc, and reconcile its
resume/watched state after reconnecting. SQLite is the implementation target. External databases
are deferred by the owner (#308, PRs #337/#339); do not maintain a parallel PostgreSQL path.

Prioritize a reliable small release: explicit individual movie/episode downloads in original
quality, durable device-scoped records, authenticated resumable transfer, integrity checking,
local metadata/artwork and reconnecting watch-state sync. Add prepared compatible copies after
that path is proven. No automatic season rules, cloud services, DRM, subscription infrastructure,
client filesystem management, casting of offline assets or new settings dashboard is needed.
Downloading an original does not make its codecs playable on a particular device.

The first PR is this groundwork. Runtime PRs should follow reviewed integration of #350/#418,
then start from the resulting main. Do not merge the draft review aggregate merely to unblock
this feature. Documentation #419 is independent; coordinate overlapping handoff/agent notes.
Native client repositories remain separate; no client implementation is included here.

## Current implementation evidence

Inspected main d94283c (PR #349/Jellyfin FFmpeg 8). These are reuse points, not download support:

| Existing code | Reuse / boundary |
|---|---|
| internal/api/api.go: itemFromPath, access | Check library/age/label visibility and hide inaccessible IDs. Recheck throughout downloads. |
| internal/api/api.go: stream, serveFile | Range serving exists, but stream calls playing.openStream. A download must not create a playback session, consume a playback slot or change watch history. Extract suitable serving primitives after #418; do not route through stream. |
| internal/api/auth.go: currentSession/currentUserID | Bind records to server-issued session plus selected profile; the device's display name is not identity. Never expose session hashes. |
| internal/store/play_queues.go; migrations/0062_play_queues.sql | Existing profile/session ownership, bounded lists and session revocation patterns. Do not overload play queues with download records or inherit their seven-day lifetime. |
| internal/store/optimized.go; internal/api/optimized.go | Existing source/attempt/audio provenance and generated-copy lifecycle. Completed copies need freshness validation and protection while being transferred. Admin-only optimize is not a non-admin download permission. |
| internal/api/watch.go; internal/store/userdata.go | Watched thresholds/resume persistence exist. reportProgress also touches live playback/history; offline replay must not impersonate an active player. |
| internal/store/store.go | Ordered SQLite migrations and BeforeMigrate safety backup. Allocate migration numbers from current main at implementation time. |
| internal/api/identity.go | Capability detection and persistent server ID. Advertise download features only after each usable end-to-end path exists. |

Pending #418 changes auth/revocation, file identity, conversion safety and root-owned shutdown.
Reverify these integration points after merge. No assertion that the current code already provides
safe download admission, strong source validators, device-local storage or offline reconciliation.

## Ownership and policy

- Require an authenticated selected profile and the same existing device session token. An opaque
  download ID is an identifier, never a bearer credential. Do not put tokens or signed owner URLs
  in manifests. Original-quality bytes require an explicit download permission; ordinary playback
  permission alone must not silently grant export of original files.
- Proposed first default: downloads allowed for the administrator; other profiles require an
  explicit administrator grant. Add one profile permission when runtime work needs it; UI is a
  separate handoff. No route for ordinary users to enable themselves. Permission changes are
  authoritative server-side even if a client cached an older capability response.
- Records belong to (profile, authenticated session). Profile switching cannot reveal another
  profile's records. A newly issued token is a new device session; automatic cross-token adoption
  is outside v1. The client can reauthenticate and explicitly register a new request for the same
  accessible title while retaining its local file until integrity/provenance is reconciled.
- Revoking a session/profile/grant prevents new metadata/byte requests and cancels active server
  work/transfers within a bounded interval. Already copied bytes cannot be remotely recalled;
  this is not DRM. Apps should hide profile-bound local data on sign-out and offer deletion.
- Bind cached state to server ID, profile ID and download ID/revision. A restored database keeps
  server identity but does not prove a restored session, source revision or artifact is current.

## Selection, representation and integrity

1. Start with one explicit movie/episode and one of its accessible main file IDs. Validate ownership
   of that file; never accept a path. Reject multipart items or shared multi-episode-file extraction
   in the first implementation with an actionable unsupported-selection result. Do not export only
   one part or silently claim that one episode contains no others.
2. Options describe recorded source/container/codec/track data without probing, scanning or starting
   FFmpeg. Mark unknown facts unknown. The client assesses actual decoder/local-storage support.
3. On an explicit create, capture source revision/provenance and selected policy. Preparation checks
   existence and source freshness. Computing an original's SHA-256 is a bounded background read,
   not work done inside a metadata GET or library scan. Persist its digest and size in the manifest.
   Cancel hashing on source drift, revocation or shutdown; never write alongside the source.
4. Original bytes can be served from the source without retaining a second full server copy.
   Revalidate an opened file descriptor's identity/size/nanosecond mtime and saved revision before
   serving, and reject detected drift. These metadata checks alone are NOT a strong content ETag
   or a guarantee against a writer preserving all metadata during transfer. Do not emit a strong
   ETag/If-Range promise for this mode. The client must verify the manifest SHA-256 after assembly;
   a mismatch discards the partial result and obtains a fresh manifest. Do not mark an unverified
   file ready for offline playback. Document that concurrent external edits may require a restart.
5. Prepared copies later use immutable, atomically published artifacts with a content digest and
   strong ETag. Range/If-Range can then use that exact immutable representation. Do not use a weak
   ETag as If-Range, concatenate different revisions, or reuse live HLS session URLs offline.
6. Manifest metadata and cached artwork must suffice offline. Persist a bounded snapshot of title,
   series/episode identity, numbering/display context, duration, source technical facts and selected
   subtitle/audio choices; retain opaque Arc identities for later sync. No host paths, raw provider
   URLs or secrets. Embedded tracks remain in an original; selected supported external text subtitles
   are separately verified assets. Unknown/image subtitle delivery must be explicitly unavailable
   until implemented, not silently dropped. Source HDR labels are not output acceptance.

A completed HTTP response does not prove an app retained a file. Separate server preparation state
from client-reported/verified local availability. Range retries/overlaps cannot be summed into a
truthful completion percentage. The app owns transfer progress, pause/resume and its storage checks.

## Durable records and lifecycle

Design the SQLite migration after #418, not against an assumed next number:

- downloads: opaque ID, owner profile/session FKs, item/file references, request idempotency key,
  request fingerprint, representation/source revision, state, timestamps and bounded safe failure code.
- download_assets: download ID, opaque asset ID, internal storage reference, media type, bytes,
  digest, source/artifact provenance and readiness. Internal paths/hashes of auth tokens never leave
  the store. Derived artwork/sidecar snapshots belong in Arc-owned storage if needed.
- offline_sync_receipts: profile plus event ID uniqueness, canonical request fingerprint and outcome;
  replaying a different payload under the same ID conflicts. Receipts must survive session renewal
  long enough to make retry safe; authentication/visibility is always checked before returning them.
- watch-state revision: monotonic per (profile,item), updated by EVERY relevant existing writer,
  including progress, watched/unwatched, bulk changes and merges. A new offline-only revision would
  miss online edits. Preserve identity/receipt behavior across rename/Fix Match/merge explicitly.

Do not keep an FK cascade that removes the last file reference before scheduling generated-asset
cleanup. Use a transactional cleanup/outbox or existing root-owned reconciliation to clean only
Arc-owned unreferenced artifacts. Never delete source media. Reconcile after crash, restore, library
removal and session expiry. Backups preserve DB state; generated files require explicit backup scope
and unavailable assets must be reconciled rather than assumed present after restore.

Server state: preparing -> ready; preparing -> failed/cancelled; ready -> stale/expired/cancelled.
A retry creates a new attempt/revision; an old worker cannot publish over it. Record insertion and
job admission are atomic/recoverable. A root-owned worker resumes/reconciles durable unfinished
records after restart. No detached request goroutine. Source disappearance never triggers automatic
rescan/rematch or destructive library cleanup.

Proposed first bounds: one active original-hash job, one selected title per create, <=100 records
per device, paged lists <=100, <=100 events per sync batch and bounded JSON bodies. Return clear
429/Retry-After for admission saturation. Confirm byte-serving concurrency against existing stream
infrastructure; separate download admission from playback slots and reserve playback capacity.
Hashing/preparation must yield or pause when foreground playback needs resources. Do not surface
internal worker counts as ordinary settings.

Original mode does not reserve full server media-copy space. Still bound metadata/artwork/subtitle
staging and orphan retention. Before adding converted copies, implement low-space admission,
bounded staging/quota accounting, artifact reference leases and cleanup. No eviction of an artifact
currently being read; never evict uploads or source media. Device-local file deletion stays local.
The app may release the server record only after pending offline state has synced or been explicitly
discarded; otherwise reconnect sync cannot identify its download. Active server records are not
silently expired by play-queue timeout. Abandoned revoked/failed/cancelled records use bounded cleanup.

## Offline progress and conflict behavior

Do not replay offline events through the live progress route. Use existing pure store/domain rules
where possible, in the same transaction as revision checks and receipt writes. No fake now-playing
entry, transcoder session, notification or invented online watch-history duration.

Persist events on the client before considering them delivered. Events carry an opaque event ID,
download/item context, base server watch revision and explicit action. Sync is a bounded batch with
an outcome per event. The server applies only visible authorized records with a matching revision,
then atomically stores the receipt/new revision. Repeating an identical event returns its existing
outcome without double counting. A different body with the same ID conflicts. Expired dedup windows
must produce an explicit resync requirement, not silently reapply an old event.

A newer online/other-device change conflicts: retain the offline event and return current permitted
watch state/revision. The app offers Keep server or Apply this device; applying uses the newly read
revision and a new event ID. Never pick by untrusted wall-clock time, maximum position, or assumed
'watches only move forward'. Rewatching and explicit Mark unwatched are legitimate. Process one
item's events in order; advance the local base revision from acknowledged outcomes. Other items in
an otherwise valid batch may succeed; malformed envelopes reject the whole batch before writes.

First sync release covers resume/watched state only. Detailed offline watch-history sessions are
later work; do not invent reliable viewing durations from two client timestamps. State thresholds
follow current server profile policy; the client caches enough policy to explain local behavior.
Permissions removed while offline cannot erase local bytes but do prevent their state from being
written back until the profile is authorized again.

## Implementation phases and acceptance

| Phase / PR | Deliverable | Acceptance before merge |
|---|---|---|
| 0 — this PR | Source inventory, decisions, proposed contract and client handoff | Planning prose/JSON/link consistency; #62 records status; no feature advertised. |
| 1 — ownership and original preparation | SQLite migration, permission, durable create/list/get/cancel, idempotent admission, manifest/digest worker | Real handler/store regressions for hidden/foreign files, profile/session switch, revocation, duplicates/body limits, stale attempts, interruption/restore; no full-file hash during GET or scan. |
| 2 — byte/asset delivery | Original single-range transfer, manifest assets, drift/integrity protocol | Generated clips/sidecars; HEAD/200/206/416, disconnect/restart/resume/mismatch, no playback/history side effects, bounded revocation, safe filenames, no leaked paths/tokens. Original-only capability may now be advertised after phases 1+2 pass together. |
| 3 — reconnect sync | All-writer watch revision and bounded idempotent offline events | Repeat/batch/conflict/rewatch/unwatched, online race, merge/rename and receipt retention tests; transactional cancellation/DB-failure recovery; no fake active playback. Advertise separate sync capability only now. |
| 4 — native original-mode acceptance | Client local storage/player/sync workflow using stable implemented API | Actual target device: background suspension/relaunch, airplane-mode playback/captions, low storage, sign-out/profile switch, reconnection/conflict UI. Server fixture checks alone do not close #62. |
| 5 — optional compatible copies | Durable bounded preparation of complete MP4 outputs, immutable leases/quotas, selected audio/subtitles | Exact Docker Jellyfin FFmpeg 8, real generated-media conversion/selection/output checks, low-space/shutdown/cancel/stale-worker tests, no duplicate independent conversion pipeline. Then advertise converted mode. |

For runtime PRs follow CLAUDE: failing scenario regressions first, Go vet/all tests, race on touched
packages; production/e2e TypeScript/build and affected journeys for web work. Regenerate API docs
from actual handlers and preserve every v1 contract entry. Log bounded admission/preparation outcomes
with safe IDs/counts/duration; cancellations are expected. No chunk/progress/heartbeat info logs,
request bodies, credentials, raw URLs or paths. Guard expensive debug work; redact FFmpeg errors.

## Plex comparison boundary

Official references: [Downloads overview](https://support.plex.tv/articles/downloads-overview/)
and [Downloads FAQ](https://support.plex.tv/articles/downloads-sync-faq/).
Both returned HTTP403 during this investigation (11 November). Current Plex client requirements,
quality controls, permission defaults and exact offline conflict policy were not freshly verified.
Do not present Arc's proposed defaults, schema or internal workers as Plex behavior. This plan uses
Arc's verified current code and explicit proposed decisions; no feature-parity claim is made.

## What is deliberately left for later

Automatic upcoming-episode/season rules, smart retention, device-to-device transfer, detailed offline
viewing-history imports, remote deletion guarantees, TV-platform-specific local storage, browser/PWA
offline storage, and broad copy/transcoding choices. First prove an explicit original download plus
safe reconnect flow on one native client. No Mac, physical device or production server is required
for server fixture development; each platform's real acceptance remains a separate client task.
