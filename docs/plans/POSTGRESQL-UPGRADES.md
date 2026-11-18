# PostgreSQL schema upgrades

Owner-authorized backend follow-up to #308/#337. SQLite stays the default; transferring from
SQLite is separate from updating an instance that already uses PostgreSQL.

## Implementation

1. Preserve the original PostgreSQL baseline as immutable release 1. Introduce release 2 with
   explicit PostgreSQL schema-version bookkeeping, plus an ordered SQL upgrade from release 1.
   Future releases retain historical SQL/table/catalog snapshots and the SQLite migration prefix
   each snapshot represents. Refuse unknown/newer schemas and incomplete migration paths.
2. Check the actual schema (columns/defaults/constraints/indexes/sequences/functions) against the
   recognised catalog snapshot, rather than trusting its marker. Retain the existing process-wide
   advisory lock throughout startup. Normal requests/background work start only after success.
3. Invoke the existing BeforeMigrate hook and require a verified portable backup before DDL. Export
   using the old schema's columns and SQLite migration prefix, never newer columns. Use a consistent
   repeatable-read source; preserve all portable rows/identities and verify content and constraints.
   Persist the safety file before permitting upgrade. A missing/failed backup blocks the upgrade.
4. Apply the complete pending PostgreSQL migration chain in one transaction with bounded lock wait.
   Record versions/fingerprints and SQLite migration history in that transaction. Check the resulting
   catalog before commit; failed/cancelled DDL rolls back. Never fall back to stale SQLite.
5. Retain the safety backup for explicit restore/recovery; do not automatically roll back a database
   after the upgraded server has accepted writes. Compatible restarts do no backup/migration work.
6. Document the developer recipe for future columns/tables/indexes, operator recovery and Claude's
   existing UI/API integration. No new configuration controls or API changes are needed.

## Acceptance

Real PostgreSQL 16/17/18 fixtures: upgrade the original release with stored users/settings/history,
compare portable contents before/after, reopen without repeated backup, restore the safety backup,
exercise a future-column migration, reject tampered/newer history/schema, fail safely on missing or
failed backup, roll back broken/cancelled DDL, and exclude simultaneous startup. Verify baseline
catalog parity on fresh initialization and SQLite/transfer/restore regression tests. All fixtures
are disposable; no owner media, production database, credentials or Unraid are used.

Nontransactional operations (for example CREATE INDEX CONCURRENTLY) are not supported by this
startup runner. Such work needs an explicit separate maintenance design. PostgreSQL major-version
upgrades remain the operator's responsibility. No production/hardware/load acceptance is implied.

## Implementation handoff

Implemented with immutable releases 1/2, historical-layout backup support, transactional chain
execution, catalog verification and existing startup integration. Actual results and limitations:
[change note](../changes/2026-11-04-postgres-schema-upgrades.md). Future migrations remain explicit
reviewed release work; this runner does not infer arbitrary schema transformations.
