# Automatic PostgreSQL schema upgrades

Owner requested seamless Arc updates after moving SQLite to PostgreSQL. Implemented on
`codex/postgres-schema-upgrades`, based on PostgreSQL #337 at ccfaf58. Plan:
[POSTGRESQL-UPGRADES.md](../plans/POSTGRESQL-UPGRADES.md). Operator/developer guide:
[POSTGRESQL-UPGRADES.md](../POSTGRESQL-UPGRADES.md).

Preserved the original PostgreSQL release and added an explicit release-2 migration for version
bookkeeping. Startup recognises versioned fingerprints, checks actual catalog/migration history,
creates a verified old-schema portable safety backup through the existing hook, applies the complete
pending native PostgreSQL migration chain in one transaction on the advisory-lock connection, checks
the resulting structure and starts normally. A current-schema restart takes no upgrade backup.
Unknown/newer/modified structures, missing/no-op/failed backup hooks and nontransactional scripts
are refused. Failures/cancellation/connection loss roll back; there is no stale SQLite fallback.

Portable helpers use immutable historical columns, row identities, text sorting and SQLite migration
prefixes for before-upgrade export. Backups verify every table's rows/digest and references and sync
before publication; POSIX publication syncs the containing directory. Retention protects the just-created
safety copy even when existing timestamps are ahead of the clock. Completed transfer recovery can
verify an archived schema before the normal startup upgrade. Existing transfer ownership/report fields
are preserved. No API payload/routes or product UI changes; Claude handoff updated accordingly.

Logging adds one upgrade-start and one successful-completion info record, with versions and duration.
Existing startup error handling reports failures once; existing backup outcome logs remain. No per-row,
query, progress or heartbeat logs, new credentials or SQL arguments. Test assertions cover log volume/privacy.

Actual validation (all isolated, prepared tools, cached PostgreSQL images):

- PostgreSQL 16.10 and 18.0: store/selector/server suites each passed 139 test/subtest results across
  three packages; each skipped one separately configured TLS-fixture test. Additional ordinary-role,
  multi-step/disconnected rollback and populated-table future-column checks passed on both majors.
- PostgreSQL 17.6: full store suite passed (132 test/subtest results), corrected selector/server run
  passed (seven), and final focused backend run passed 45 results across four packages. Initial
  selector privacy assertions failed because a passwordless trust fixture made empty-string secret
  checks always match; a nonempty fake fixture password corrected the setup, with no assertions removed.
- Targeted PostgreSQL/store/backup race checks passed (29 test/subtest results), plus final populated
  row/default/logging/ordinary-role race checks and the complete backup-package race recheck.
- Go vet, production web/i18n/app TypeScript build, E2E TypeScript and diff checks passed. Three pinned
  Chromium backup/restore journeys passed with zero skips/retries; the supervisor built the embedded server.
- Full default-backend Go run: 584 passing results, 35 PostgreSQL skips, one failure in unchanged
  TestConversionWaitsForThePlayer. It also fails identically on unchanged #337 (10 segments versus
  expected about one), and on an isolated package recheck. Not fixed or represented as a clean full pass.

Evidence: `/workspace/arc-cloud-evidence/postgres-upgrades/REPORT.md` and raw logs/summary.json.
No owner data/settings/media, production credentials, Unraid or production database used. No tool or
dependency installs. Temporary PostgreSQL fixtures are stopped after validation. Full browser/container,
real-device, verified-TLS fixture, Windows filesystem, large-library downtime and sustained-load acceptance
were not repeated. PostgreSQL engine/container major upgrades remain manual; each future Arc schema
change must ship explicit native SQL, immutable snapshots and preservation tests. Nontransactional
maintenance and automatic destructive recovery/downgrade are outside this runner. #308 stays open for
remaining UI/deployment work. Follow-up PR depends on #337; no PRs merged.
