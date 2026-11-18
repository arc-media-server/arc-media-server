# Automatic PostgreSQL schema upgrades

Arc updates can now upgrade a recognised older Arc PostgreSQL schema automatically. This is separate
from the initial SQLite-to-PostgreSQL move. SQLite remains the default. Installing a newer Arc image
and starting it is the normal operator flow; no manual SQL or rescanning is required for supported
schema upgrades. The PostgreSQL container's own major-version upgrade remains an operator task.

## What happens at startup

1. Arc connects to the configured PostgreSQL database and holds its existing exclusive advisory lock.
   A second Arc instance cannot start against it. HTTP serving and background workers have not started.
2. It recognises the stored release fingerprint/version and checks the actual columns, types,
   collations, defaults, constraints, indexes, functions/operators, sequences and user triggers against
   that release's catalog. The SQLite-compatible migration history must match a complete known prefix.
   A newer, unknown or unexpectedly modified schema is refused before backup or DDL.
3. If an upgrade is pending, the normal startup backup hook creates a before-upgrade portable `.db`
   snapshot under the existing backups directory. Export uses the *older* release's tables, columns
   and SQLite migration prefix, so it never tries to read a column that has not been added yet.
   Every copied table's rows/digests and references are verified. The owner-only file is synced before
   publication; its final directory entry is synced on POSIX. Failure or a missing/no-op hook blocks DDL.
4. Arc runs every pending PostgreSQL upgrade in one transaction on the connection holding the advisory
   lock. The transaction updates version/fingerprint and migration history and checks each resulting
   release's structure. All pending steps commit together. Lock waits are limited to five seconds;
   the ordinary 30-second statement timeout is disabled inside the upgrade transaction to permit
   larger DDL. Startup cancellation still cancels the work. No fixed downtime promise is made.
5. After success Arc opens its normal read pool and starts. A compatible restart neither takes another
   upgrade backup nor repeats migrations. The retained backup follows existing safety-backup retention.

This implementation includes the original release-1 contract and a real release-2 upgrade adding
explicit PostgreSQL version bookkeeping. Future product schema changes still need their own reviewed
native PostgreSQL SQL and data-preservation tests. The runner does not guess SQL from a schema diff
or replay SQLite migration SQL against PostgreSQL.

## Failures and recovery

- Backup failure: fix the disk/permissions problem and restart. No schema upgrade has begun.
- SQL failure, cancellation or lost upgrade connection: PostgreSQL rolls back the whole transaction.
  The old schema and safety backup remain. Fix the reported problem/use the matching previous build,
  then restart; the next attempt checks the database afresh.
- Lost connection around commit: the next startup checks which complete release is present. It never
  assumes that an HTTP timeout means rollback, reruns completed steps blindly or selects stale SQLite.
- After a successful upgrade has accepted writes, do not simply downgrade the image or switch to the
  retained SQLite source. Older binaries are refused by the newer fingerprint. Deliberate data recovery
  uses a verified portable backup through the existing restore workflow on a compatible running build;
  restoring older contents upgrades a disposable SQLite copy before replacing PostgreSQL contents.
  Reverting an Arc binary/schema is a separate recovery operation and can lose writes after the backup.
- Unknown/tampered structures require investigation or recovery from a known compatible backup;
  never edit version/fingerprint markers to force startup. No automatic destructive restore is offered.

No media, artwork files, generated assets or PostgreSQL credentials are embedded into a portable
backup. Those existing file-backed volumes/bootstrap protections remain required.

## Adding a future schema change (developer workflow)

1. Add a new SQLite migration; never edit already released SQLite migrations.
2. Increment `postgresSchemaVersion` in `internal/store/postgres_upgrades.go`. Generate the latest
   PostgreSQL baseline/table manifest with `go run ./tools/dbschema`.
3. Add the next ordered `internal/store/postgres/releases/NNNN/` directory. Copy in the new baseline
   and tables manifest; add `manifest.json` with its version and full SQLite migration count. Preserve
   every historical directory. Add `upgrade.sql` containing native PostgreSQL transformations from
   the previous release, including any required change to the marker column's default. The runner
   updates marker values and appends SQLite migration-history records; do not duplicate those writes.
4. Keep migration SQL transactional: no BEGIN/COMMIT/ROLLBACK, VACUUM or CONCURRENTLY operations.
   The runner rejects transaction-control scripts. Nontransactional maintenance needs a separate design.
5. With `ARC_TEST_POSTGRES` pointing to a **disposable test database**, run `go run ./tools/pgcatalog`.
   It creates/drops uniquely named scratch schemas to capture baseline catalogs, leaving existing
   schemas alone. Inspect the diff: previously released artifacts must remain unchanged. Do not use
   an owner/production database for this developer tool. Runtime upgrades need no pg_dump/pg_restore.
6. Add migration-specific preservation/default/backfill tests and test original/previous release
   upgrades, chains, portable backups/restores, interruption and invalid schema rejection on supported
   PostgreSQL 16–18. Compare migrated catalogs with fresh initialization. A new column/table/index
   alone in the fresh baseline is insufficient: registry/catalog tests must pass for existing instances.

No multi-node coordination, PostgreSQL engine/container upgrade, nontransactional migration runner,
large-library downtime benchmark or production/Unraid acceptance is claimed. Schema catalog checks
are deliberately conservative; manual schema modifications are unsupported. Changes to the catalog
normalizer need cross-version tests and review of all immutable release snapshots.
