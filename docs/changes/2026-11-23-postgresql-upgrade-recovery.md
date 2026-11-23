# PostgreSQL schema updates and recovery

[#308](https://github.com/arc-media-server/arc-media-server/issues/308)

Opening an older recognized Arc PostgreSQL schema now requires a verified safety archive.
Arc writes the archive from a repeatable-read snapshot on its locked writer, then applies
the required schema steps, migration history and final fingerprint in one transaction.
An unknown version or a table/sequence layout that differs from its saved fingerprint
blocks startup. Failed SQL rolls back and leaves the archive available.

The archive contains native PostgreSQL row data, typed column lists, row counts, and the
identity sequences. It is written to a private temporary file, synced and read back to
verify ZIP integrity before publication. Restore recreates the recorded schema and checks
the row counts, layout and migration history. It only accepts an empty target and leaves
the original database selected. Arc does not require PostgreSQL client utilities.

## Rehearsal and recovery

- Local and release verification run `bash tools/ci/check-postgres.sh` with a disposable
  PostgreSQL 19 container. Pass a fixture image as the first argument to check another
  major version. The existing SQLite checks continue in the same workflow.
- To recover a verified archive, provide the connection to a dedicated empty replacement
  database through `ARC_POSTGRES_RECOVERY_DSN`, then run
  `go run ./tools/postgres-recovery /path/to/archive.pg.zip` from the repository root.
  The tool reports success after the restore transaction commits. It does not select the
  replacement or restart Arc.
- Archives contain private application data and belong in protected storage. They cover
  the database. Uploaded assets, generated assets and media stay outside the archive.
  This work provides before-upgrade archives and destination recovery. The ordinary server
  still selects SQLite and its existing backup workflow.

The initial PostgreSQL baseline becomes immutable once first-run PostgreSQL is offered.
New Arc changes to the schema add a corresponding entry to the PostgreSQL registry and
a PostgreSQL 19 upgrade/recovery test where the change affects preserved records. The
initial baseline contains no shipped upgrades. The added-column rehearsal is test-only.

## Logging

The startup caller receives a safe error for one failed backup or migration,
including the registered step name where applicable. The recovery tool logs one outcome.
Neither path logs database URLs, SQL arguments, credentials or archive contents.

## Validation

- PostgreSQL 16.10 and 19.0 with ordinary fixture roles passed the release check under the
  race detector. The rehearsal added a column to populated data, refused a failed backup,
  rolled back an injected SQL failure, rejected an incomplete archive and restored into
  the same empty target. It preserved the server ID, profile, watch history and a sequence
  gap. A repeat start created no extra archive and a second restore refused the populated
  destination.
- Go vet, the default Go suite with Jellyfin FFmpeg 8 and the embedded server build passed.
  Production web, app and E2E TypeScript and the pinned Chromium SQLite onboarding journey
  passed. One PostgreSQL race recheck timed out hashing the fixture password while the
  broader checks were running. The final gates passed with a longer fixture HTTP deadline.
- The release check script ran locally. Hosted CI, PostgreSQL engine major-version changes,
  Windows durability, production data and regular PostgreSQL backups were outside this check.
