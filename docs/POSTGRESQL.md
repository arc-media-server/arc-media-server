# Optional PostgreSQL backend

SQLite remains Arc's default, with the same local `ams.db`, zero-service setup and existing
backup/restore. PostgreSQL is an explicit advanced option, for one Arc server. This first backend
implementation is part of #308; the Settings UI is not included. See POSTGRESQL-UI-HANDOFF.md.
Do not describe #308 as complete until the UI and deployment acceptance have been reviewed.

## Supported scope

PostgreSQL 16–18, a dedicated empty database, and an ordinary login role owning that database.
Arc needs to create its own tables, indexes, SQL functions and operators; it does not require
superuser, role creation, database creation, extensions, pg_dump or pg_restore. The database
must already exist. PostgreSQL must be reachable from the Arc container/server.

The current logical schema is all 63 SQLite migrations and all 43 persistent tables. The
PostgreSQL baseline is generated from that history, preserving constraints, indexes, foreign
keys, default values, 64-bit integers and ASCII-only SQLite case matching. PostgreSQL-specific
identity counters and stable internal row numbers preserve IDs and existing ordering choices.
Shared domain methods remain the source of business logic. No ORM or separate store per feature.

One serialized PostgreSQL writer preserves Arc's existing transaction assumptions; up to eight
read connections are read-only, plus a control connection holding an advisory lock. Connections
use a five-second connect timeout, 30-second statement timeout and 60-second idle transaction
timeout. These are internal defaults. This does not make Arc multi-node or promise faster scans.

**Schema upgrades are deliberately fail-closed.** This initial version opens only its exact
PostgreSQL schema fingerprint and matching migration ledger. It does not replay SQLite SQL on
PostgreSQL. Future SQLite or PostgreSQL schema changes must supply an explicit tested PostgreSQL
upgrade path, with a portable pre-upgrade safety backup, before shipping to PostgreSQL users.
Regenerating a fresh baseline alone is not an existing-installation upgrade implementation.
Until that upgrade path exists, keep the matching Arc binary with a PostgreSQL installation;
a different schema is rejected without altering it. SQLite upgrades retain their existing flow.

## Moving an existing instance

The admin API provides connection preflight and a confirmed transfer request. The UI should
explain downtime before submitting it. API details and JSON examples are in the handoff.

1. Check target connectivity, TLS verification, PostgreSQL version, emptiness and create rights.
2. Save a protected transfer journal, reject new requests with 503/Retry-After, stop background
   work, drain HTTP requests and close the old Store. Long active streams are closed after the
   normal ten-second drain. The data-directory lock excludes another Arc process.
3. Read SQLite, including WAL, into a consistent owner-only safety backup without upgrading,
   rescanning or rewriting the source. Retain it in the existing backups directory using the
   existing before-upgrade safety-backup naming scheme.
4. Create the PostgreSQL schema and copy every persistent table in one transaction with bounded
   memory. Defer relationship checks until records are copied, then check constraints and compare
   each table's complete canonical row content/count with its source. Preserve row identities,
   nulls, numeric values, metadata, hashes, settings, watch state, ordering and job bookkeeping.
5. Advance numeric counters without rewinding existing counters, record the ownership/completion
   marker and commit only a verified target. Persist verified journal state, then atomically switch
   the local selector to PostgreSQL and restart Arc automatically.
6. Reconnect using existing credentials/sessions. Ordinary configured startup scan/background
   policy remains; migration does not reset libraries, schedule a mandatory rebuild or rematch.

The transfer itself takes no live provider calls and never reads media. The same Arc data volume
and media mounts are still required. Uploaded artwork, certificates, cached/generated assets and
optimized files stay on disk; only their database references move. A database backup is not a
full instance archive.

## Recovery and returning to SQLite

Before selector cutover, SQLite remains authoritative. A cancelled/failed copy rolls back its
new PostgreSQL schema/data. If target commit succeeded immediately before a crash, startup uses
the transfer ownership ID and rechecks all source/target table contents before completing the
selector switch. A different, populated or changed target is refused. Pending recovery blocks
normal startup rather than guessing which database is authoritative.

Fix the reported connection problem and restart Arc to resume pending recovery. To abandon a
pre-cutover move, stop Arc and run the same binary with its data directory:

```sh
ams -data /path/to/arc-data -cancel-database-transfer
```

Then start normally. This cancels only a pending SQLite move; it cannot revert active PostgreSQL.
The target and safety backup are retained. An incomplete crash-time snapshot may require this
explicit cancellation before retry; Arc will not silently discard a questionable safety file.
A retry needs an empty destination; never delete someone else's database to make it pass.

After PostgreSQL accepts new writes, retained SQLite is an older snapshot. There is **no automatic
fallback** on PostgreSQL outage, bad credentials or a damaged/missing selector. A separate marker
also rejects stale SQLite when an active PostgreSQL selector is missing. Restoring the old SQLite
file would lose later PostgreSQL writes. Guided reverse transfer is not implemented; current
portable backups can be opened by a matching SQLite Arc build as an explicit recovery operation,
with downtime, the same files/mounts and appropriate deployment/bootstrap handling.

## Connection secrets and TLS

Connection requests accept host, optional port (5432), database, username, password and optional
TLS mode. `verify-full` is the default and verifies the server certificate and hostname; there is
no automatic plaintext retry. Use a certificate trusted by Arc's system trust store. pgx also
supports deployment-provided `PGSSLROOTCERT` for a private CA. `disable` must be an explicit
advanced choice for a trusted network. TCP connections are supported; this UI contract does
not offer Unix sockets or arbitrary connection-string options.

`database-bootstrap.json` in Arc's data directory contains credentials and journal state. It is
written through a synced temporary file/rename with mode 0600, and unsafe POSIX permissions,
symlinks, unsupported versions or malformed contents are rejected. Windows builds use their
filesystem ACLs; a Windows compile is not physical Windows ACL/durability acceptance. Keep the
bootstrap, `.postgres-required` marker and data volume protected and persistent. Never paste
bootstrap contents into diagnostics/issues. API status never returns the connection or password.

Driver errors retain safe SQLSTATE/operation categories, omitting connection strings, backend
row details and secrets. Transfer logs record bounded phase/outcome/count/duration information,
not each row, path, key or payload. No successful-query/poll info logging is added.

## Backups, restore and maintenance

PostgreSQL backups use a consistent repeatable-read snapshot streamed into a standard Arc SQLite
`.db` backup. The existing retention/download/restore API remains. These files include settings,
keys and password hashes and retain existing admin-only protection. No external database utility
is required. Canonical content and foreign keys are checked; incomplete exports are removed.

PostgreSQL restore upgrades an older supported backup only in a disposable copy, then validates
and replaces application records in one transaction. DELETE preserves existing MVCC snapshots;
TRUNCATE is deliberately avoided. Identity counters only move forward, because PostgreSQL
sequence operations survive transaction rollback. The existing backup manager still takes its
before-restore safety backup. File-backed assets are outside database backup scope.

Optimize database updates PostgreSQL statistics with ANALYZE and reports database size. PostgreSQL
manages vacuuming; Arc does not claim SQLite integrity checks, WAL checkpointing or compaction ran
on it. `DatabaseUpkeep.backend` is additive. Client help text should reflect the selected engine.

## Verification and remaining limits

Actual results are recorded in the change note and /workspace/arc-cloud-evidence/postgres-implementation.
Tests use disposable PostgreSQL instances, temporary stores, fake keys and local/generated media.
Shared store fixtures run against PostgreSQL, while explicit SQLite migration/full-disk/physical
page tests continue to run against SQLite; those are not PostgreSQL parity claims. The all-table
fixture populates every persistent table and includes large integers, nulls, Unicode, HDR/audio
probe data, access, edits, history, custom orders, queues and file-backed references.

Owner-server/Unraid deployment, real media, sustained load, large-library downtime measurements,
physical Windows behavior and future-version PostgreSQL upgrades are not accepted by these tests.
No multi-instance failover, automatic old-SQLite rollback or complete filesystem migration is offered.
