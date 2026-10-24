# Optimized-copy source revisions

Issue [#170](https://github.com/JonzaUK/arc-media-server/issues/170) fixes reuse of an optimized
copy after its original media changes. No new setting is needed.

## Selection and streaming

Each queued copy records its source file ID, path, size and saved modification time, plus a
unique attempt ID. Conversion records the actual filesystem modification time at nanosecond
precision. Playback selection and the final stream request check the current saved source,
actual source metadata and artifact existence/size. An unavailable or changed source, or a
missing artifact, makes the copy ineligible; normal playback planning can use the original,
remuxing or live conversion instead. This does not force a new full-file probe on every scan.

New optimized URLs contain the attempt ID. An old URL returns 404 after that attempt is replaced
rather than silently switching to a different generation. Legacy URLs without that ID can
still request the current valid copy. Source checks also run before a filesystem change has
been scanned. These are metadata revision checks, not a whole-file content hash: a change
that deliberately preserves all checked metadata is outside that guarantee. A stream already
opened is not retroactively revoked when an external program changes its source.

## Conversion and recovery

Scanning a changed file invalidates its old optimized record in the same store transaction,
preserving the item, edits, locks and watch state. An explicit Optimize request can regenerate
failed/changed copies and repair a missing artifact. Unchanged completed or queued work stays
idempotent.

Only the matching queued attempt may claim work; only that converting attempt may finish
while its source still matches the catalog. Source metadata and filesystem identity are checked
again after FFmpeg exits. Attempt-specific output filenames prevent late workers overwriting
newer files. Superseded results cannot complete or fail a newer queued/completed attempt.

Partial files are removed on normal failure/cancellation. Interrupted conversions receive new
attempt IDs on startup and are queued again. Startup pruning removes only unregistered files
matching Arc's generated filenames in its optimized directory. It retains registered artifacts,
including old failed copies that can still be removed through Settings, and unrelated files.
Successful regeneration removes its superseded owned artifact. No source media is modified.

## Upgrade behavior

Migration 0049 adds source/attempt fields. Older completed copies have no provable source
revision: a current catalog row cannot establish when those bytes were converted. Their files
remain removable, but their status asks the owner to **Optimize again to refresh this older
copy**. They are not silently trusted or automatically regenerated as a large background batch.
Queued/interrupted legacy work receives current saved source snapshots and is validated before
conversion. Normal backup-before-migration and restore/version-compatibility safeguards apply.

This issue does not change optimized audio selection (#171), bitrate eligibility (#172),
format/codec labels, quality presets or background processing policy. Source codec/HDR labels
still describe the source, not the capabilities of a generated copy or the actual device output.

## Verification boundary

Regression coverage includes unchanged and changed source revisions, delayed completion after
replacement/deletion/requeue, claim ownership, missing artifacts, revision-bound URL rejection,
restart and partial-file cleanup, real generated six-/nine-second clips, identity/edit/resume
preservation and an actual schema-48 upgrade with a valid pre-upgrade backup. Full-app HTTP
fixtures and focused Chromium playback/resume/restore journeys supplement the Go/store tests.
See READINESS and #170 for actual results and the separate hosted publication outcome.
