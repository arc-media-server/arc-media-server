# PostgreSQL for new Arc installations

Tracking issue [#308](https://github.com/arc-media-server/arc-media-server/issues/308).
Drafts [#337](https://github.com/arc-media-server/arc-media-server/pull/337) and
[#339](https://github.com/arc-media-server/arc-media-server/pull/339) are reference implementations.

## First-run contract

- Show the database choice before saving an admin, server settings or libraries. Preselect SQLite.
  Put PostgreSQL under an advanced option and require a dedicated empty database.
- For existing data directories, open the selected engine as usual. An existing SQLite store keeps
  SQLite and goes through the normal update path. An unreadable database is a startup error.
- Use the same trusted setup boundary as initial admin creation. Keep the setup API same-origin and
  bounded. Accept explicit connection fields, verify TLS by default and return actionable,
  credential-free errors. Keep credentials out of URLs and logs.
- Persist the selected database in the Arc data volume with restricted permissions. The bootstrap
  state must distinguish an unfinished PostgreSQL setup from a completed installation. Bind any
  retry after an interruption to the original initialization attempt so it cannot adopt another
  Arc installation's database. Serialize competing setup requests.
- Finish initializing the selected schema before serving the normal onboarding flow. Once completed,
  the selection stays fixed. A PostgreSQL connection failure leaves that engine selected.
  Keep the Arc data volume for file-backed assets, backups and the bootstrap record.

## Implementation sequence

1. **Derive the initial schema.** Reuse #337's generator with current SQLite migrations and
   prove that real PostgreSQL can apply the result in a transaction. The tool opens its own
   temporary SQLite store. This stage produces DDL. The runtime initializer will seed installation
   values and record schema history in the same transaction that creates the tables.
2. **Build the optional store on current main.** Reuse the parts of #337's pool, error handling
   and SQL compatibility that still fit. Test current store and API behavior on both engines.
   Include PostgreSQL 19 in the candidate support matrix and verify it before listing it as supported.
   Address the data-folder lock review without adding a new lock prerequisite for SQLite installations.
3. **Prove updates and recovery.** Adapt #339's verified backup and transactional upgrade plan.
   Establish one PostgreSQL release baseline before publication. Use a synthetic next schema to
   exercise a populated upgrade, failure rollback and restore. Agree on a compact schema history
   format and put both database engines in release verification.
4. **Expose first-run selection.** Test fresh SQLite and PostgreSQL starts, interrupted PostgreSQL
   setup, correct restart after selection, bad credentials, unavailable databases, and the unchanged
   path for an existing SQLite install. Check that the first admin and settings land in the chosen
   store.

Lossless transfer of a populated SQLite installation is follow-up work. It needs its own
interruption and cutover tests. The existing drafts should remain available while the first-run
path is developed.

## Generate the starting schema

From the repository root, run `go run ./tools/dbschema > /tmp/arc-postgres-baseline.sql`.
Apply that SQL in a transaction on a **disposable, empty** PostgreSQL database when testing.
The SQL is derived from the current SQLite schema. PostgreSQL releases will need an explicit
migration history once supported installs exist.
