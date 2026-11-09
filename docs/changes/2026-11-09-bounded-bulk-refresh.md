# #350: bound and own bulk metadata refresh

Reproduced six requests for the same title starting concurrent metadata requests. Replace detached
per-request goroutines with one on-demand worker and a global bound of 5,000 unique reserved,
queued or in-flight titles. Coalesce duplicates, store only ids and read each item afresh at work
 time. Reserve capacity/catalog ownership before synchronous edits; reject a full queue with
429/Retry-After before changing metadata. Failed synchronous edits release their reservation.
Limit JSON to 2 MiB and normalize duplicate input ids.

Accepted work outlives its HTTP request but belongs to the server root context. The worker ends
when idle, so there is no persistent idle goroutine. Activity cancellation and a 30-minute batch
budget stop work; shutdown cancels/joins accepted workers before the store closes, including early
server-error returns. Listener/port changes retain the root lifetime. Missing or changed-kind
items are skipped. A late old completion cannot release a new reservation for the same id.

Each ticket retains a catalog read lease until all its unique work finishes/aborts, including
queued time. Reuse the identical catalog primitive from #365 and safe-error helper from #356;
merge all three preserving queue ownership, restore exclusion and trace redaction. This queue
coalesces bulk work, not every independent scan or manual matching entrypoint (separate Low item).
Synchronous bulk edit/played changes keep their existing transaction boundaries; no all-or-nothing
promise for all bulk operations is introduced.

Tests cover local-provider coalescing, one worker across different titles, capacity-before-edit,
shutdown cancellation/join/catalog release, request-lifetime independence, exclusive maintenance,
late completion ownership and body limits. No owner data, production keys, media files or persistent
server used. The provider fixture intercepts every metadata connection onto loopback; changing the
package-init URL with t.Setenv alone was an invalid initial fixture, retained separately from the
meaningful failing reproduction.

Logging: one start and one outcome per batch with duration/counts, one failure summary with first
safe title/error, debug-only per-item failures. Prepare details only for that first summary or when
debug is enabled. Cancellations are ordinary outcomes. No raw URLs/keys/tokens/bodies/SQL arguments
or per-item warnings. No delivery durability, physical deployment or performance parity claimed.
