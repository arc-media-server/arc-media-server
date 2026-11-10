# #350: reserve catalog quiescence across safety backup and restore/delete

The previously plausible check-then-act gap was reproduced through both real HTTP handlers: while
the actual safety backup was held in its normal filename-collision wait, a new scan was admitted.
Use one shared catalog reservation across the safety backup, destructive mutation and runtime reload.
Runtime operations that retain item/file identities hold shared work leases; the reservation is
exclusive. No new schema or media changes.

The existing busy refusal remains conservative: active library/background work returns 409 before
backup/mutation rather than being forcibly aborted. Once reserved, new mutation requests return
503/Retry-After and managed workers defer work. Read-only requests remain available. Cover scanner
scans/scoped refresh/Match and its async continuation/trash, background admissions/upkeep, marker and
ratings passes, optimized-copy jobs, and API async bulk refresh/import ownership. These are work
leases, not a ban on every internal SQL statement: the controlling restore/reload writes intentionally
run under the exclusive reservation.

Requests queued by watchers/restored runtime settings are retained while reserved. After release,
resume only current library ids, invalidate old automatic-trash proofs, and wake media workers.
Work leases outlive the async work and are released before scanner Wait completes. Queue admission
cannot slip between the old Running check and the safety backup. A long active catalog task may
need to finish or be stopped before restore/delete is retried; no new force/cancellation UI added.

Tests first reproduced real restore/delete admission windows; fixed cases reject the new scan.
Additional boundaries cover already-active work, mutating-request 503, active background Match,
queued scan resumption and discarded deleted-library requests. Only scratch SQLite stores, generated
fixtures, local providers and temporary backup names; no owner systems, keys, settings or media.

Logging uses existing backup/restore/delete and job outcome boundaries. Busy/deferred admission is
ordinary validation, not a server error or a warning per poll. Queue-resume failures have one safe
boundary diagnostic. No per-row logs, bodies, raw URLs/queries, keys/tokens or SQL arguments added.
Physical deployment/concurrent-owner recovery acceptance remains unverified.
