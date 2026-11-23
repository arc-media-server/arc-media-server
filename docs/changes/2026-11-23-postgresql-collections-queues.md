# PostgreSQL collections and play queue coverage

[#308](https://github.com/arc-media-server/arc-media-server/issues/308)

Add 19 existing collection, playlist, play queue and catalogue change scenarios to
the PostgreSQL release gate. These scenarios also run against SQLite in the default
Go suite.

- Collections cover automatic and manual membership, duplicate additions, ordering,
  hiding, smart filters, and rollback when a member write fails.
- Play queues cover editing, competing revisions, expiry, device and profile
  isolation, revocation, and persistence through an engine-native backup and restore.
- Merge and cleanup scenarios check that queues, watch state, arrivals and other
  references follow retained identities. The SQLite query-plan assertion still runs
  on SQLite. PostgreSQL runs the same episode cleanup result assertions.
- The selected v1 handler scenarios also run against PostgreSQL, covering collection
  creation, smart filters and queue ownership.

The initial replay reached a SQLite-only plan assertion after verifying the episode
cleanup results. This batch changes tests and release verification. Existing queries,
schemas, API behavior and logging stay as before. Downloads and imports remain separate.

## Validation

All 46 shared scenarios passed in the disposable PostgreSQL 19 race gate, including
the 19 added here. The default SQLite Go suite, vet and embedded build passed. The
build reused unchanged web assets. There were no browser or deployment changes.
