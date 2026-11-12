# Durable original-download preparation (#62)

Owner authorized the first backend slice now, with later integration/retesting, separately from pending review integration #418. SQLite
migration0068 adds explicit profile download grants and bounded device/profile-scoped preparation
records. It follows0064–0067 reserved by #418, whose changes are not imported into this branch.

Authenticated create/list/get/release routes support explicit original movie/episode file selection,
idempotent admission, ownership/visibility checks, safe DTOs and strict bounded input. Admin profiles
always allow preparation; others default denied and require the existing admin profile API to grant
access. One root-owned, joined worker hashes sources, pauses for playback, rechecks revocation/source
freshness and safely restarts interrupted work. No media writes, detached requests, playback/history
updates or duplicate full copies. Records/grants participate in existing SQLite backup/restore.

Only downloads_preparation is advertised. No media/manifest/asset transfer, converted downloads,
offline watch sync, local-device storage or frontend controls are implemented. The actual API/client
handoff and integration/latency limitations are in DOWNLOADS-PREPARATION-API.md. Proposed broader
contracts remain in planning PR #420; PostgreSQL is deferred. Reconcile/retest after #418/#419 merge
and rehearse upgrades from both branches before claiming combined acceptance; do not infer deployment/device acceptance.

Regression coverage includes a real-router permission failure before implementation, grants,
owner/device/hidden-item privacy, bounded JSON/pagination, idempotency/concurrent admission, source
mutation/deletion, interrupted work/reopen, stale/deleted publication, session revocation, backup/
restore, source-byte preservation and no playback/watch-state effects. Actual commands/results are
in the PR; test fixtures use temporary stores and synthetic/local media only.

Logging: one bounded preparation start/outcome with safe ID, count/duration; source/access failure
reason codes; expected cancellation debug-only; queue/bookkeeping errors retry at most once/minute.
No per-chunk/poll info lines, paths, auth/session hashes, keys, raw URLs/errors or request bodies.
