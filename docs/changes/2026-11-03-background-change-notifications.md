# Publish background changes through the existing authenticated event stream

Wire the shared bounded event hub into metadata/file inspection, artwork, critic ratings,
background maintenance and optimized-copy workers before they start. Publish coarse invalidation
hints after committed changes/outcomes, including partial failures and cancellation. Early
series identification and later episode/order completion both invalidate catalog data. New
topics are artwork, optimized, previews, markers and admin-only maintenance; no payload fields,
endpoints, database migration, dependencies or client UI change.

Existing library/profile/admin filtering, replay, resync and coalescing stay unchanged. Global
hints intentionally carry no item IDs, names, paths, URLs or errors. Existing worker diagnostics
remain; no per-item info logs or extra I/O/goroutines are introduced. These hints are best-effort,
not a durable event log or proof of job success. No throughput or real-device claim.

App guidance: docs/EVENTS-API.md. Cross-PR handoff: docs/BACKEND-FOLLOWUP-HANDOFF.md. Validation: the full Go suite passed 31 test packages and failed the unchanged transcode
package's TestConversionWaitsForThePlayer overshoot regression; the separate throttling PR #334
addresses it. Full vet, focused race checks across six worker/API packages, the full event-hub
race tests and embedded-server build passed. All changed packages passed ordinary tests.
Browser journeys were not repeated: no product frontend or playback path changed. Initial
fixture execution needed loopback-listener permission; test development also corrected an
unavailable test field, a disabled upkeep fixture and a done-only optimized-list lookup.
Those failed attempts remain in local evidence. Isolated fixtures only, no owner production access.


Integration check: applied #334, #335 and this source commit (`5fef77f`) to main `ad2f538` in a
separate detached temporary worktree, producing `4ae3c99`. The three applied without conflicts;
regenerating API docs produced no differences. All 32 Go test packages, full vet and embedded
build passed together. This does not erase the standalone 31-pass/1-fail result above. Merge
#334 first to carry the throttling fix. Combined logs are under
`/workspace/arc-cloud-evidence/backend-three/combined-*.log`; no hosted CI or real device checks.
