# Optional PostgreSQL backend and journaled SQLite transfer (#308)

Keep SQLite as the zero-configuration default. Add pgx-backed PostgreSQL 16–18, a generated
baseline covering all 63 migrations/43 persistent tables, shared SQL compatibility and bounded
read/serialized-write pools. Preserve IDs, ASCII case matching, sorting, relationships, metadata,
watch state and file references. Portable backups retain the existing `.db` workflow; PostgreSQL
restore uses transactional DELETE plus forward-only sequences, and older backups upgrade only
in a disposable copy. PostgreSQL maintenance uses ANALYZE, without claiming SQLite integrity,
compaction or WAL work. No performance or multi-server claim.

Admin API: database status, read-only preflight and explicitly confirmed transfer. Quiesce the
server, take a read-only WAL-consistent SQLite safety snapshot, copy all tables transactionally,
verify canonical content/counts/constraints, journal target completion and atomically switch a
protected local selector before automatic restart. Ownership/content verification recovers a
crash after target commit. Never fall back to stale SQLite after active PostgreSQL failure.
Database credentials remain write-only in APIs and owner-only on POSIX. Bounded phase/outcome
logs contain no passwords, URLs, row contents or per-row info flood. TLS verifies by default.

Handoff: docs/POSTGRESQL-UI-HANDOFF.md. Operations/limits: docs/POSTGRESQL.md. Backend work only;
Settings UI is pending. Exact schema guards reject future/different PostgreSQL schemas until an
explicit tested upgrade path is supplied. Initial snapshot crash recovery can require explicit
pre-cutover cancellation; there is no live progress/cancel UI or guided reverse transfer. These
limits keep #308 open rather than advertising all acceptance criteria complete.

Recorded validation during implementation of 184ec72 (3 November); broad runs followed
by final focused boundary/schema rechecks where noted:

- Default SQLite full Go run: 35 packages, 580 test/subtest pass events, 12 explicit
  PostgreSQL-only test skips, no failures (six additional packages contain no tests). PostgreSQL
  cases ran separately, not credited as successful checks in the SQLite run.
- Real PostgreSQL 16.10 and 18.0: three-package shared store/recovery/server suites passed
  (112 test/subtest pass events each), followed by focused final schema/older-backup/role checks.
  PostgreSQL 17.6 final three-package run: 116 test/subtest pass events, no failures; one optional
  separate TLS-fixture skip was exercised successfully in its dedicated run.
- All 43 persistent tables populated and copied with canonical checksums/counts, original source
  byte preservation, ID continuity and working PostgreSQL backup/restore. Verified earlier-backup
  upgrades modify only a disposable copy. Crash after target commit, pre-cutover failure/cancel,
  middle-of-copy cancellation, secret boundaries, ordinary-role operation, reader isolation and
  forward-only sequences on rollback passed. Real TLS cutover, wrong-host and untrusted-CA
  rejection passed using a generated local test certificate.
- Race: store/API/databaseconfig/datalock passed; the initial server test exceeded its two-second
  HTTP client deadline during real bcrypt under race/load. A test-only ten-second deadline kept
  the same outcome assertions; the server race recheck passed. Final copy/restore/schema/cancel
  boundaries passed under race too. No data races were reported; the original failed run remains.
- Production/app TypeScript, production UI build, E2E TypeScript, full vet, Linux embedded build
  and Windows cross-build passed. The first Windows compile overlapped an import edit and was
  discarded; final builds passed. Windows execution/ACL/durability acceptance is not claimed.
- Full pinned-browser run: **150 passed, 8 failed, 0 skipped/retried** (158 journeys, 22m32s).
  Seven failures reproduced on unchanged main product code: duplicate library-menu selectors,
  Home card click interception, obsolete Edit Home link, stale maintenance menu expectations and
  old quality help text. The remaining phone letter-rail timeout passed unchanged-main comparison
  and an isolated changed-branch recheck. This does not erase its failed full-run result. No
  product UI or browser assertions were changed. Baseline fixtures used separate ports only.

Raw reports/summary: /workspace/arc-cloud-evidence/postgres-implementation. Initial schema/query,
fixture compilation/permissions, wrong helper URL, login-status and TLS-message test failures
were retained, corrected and rechecked; they are not claimed as clean passes. Full-browser raw
reports retain failure details; auxiliary artifact directories were reused by later focused runs.

Prepared tools, pgx 5.7.6 and disposable PostgreSQL images/temporary fixtures only. No owner
DB/settings/keys/media, Unraid or production services were accessed. #308 remains open for the
frontend, future-version upgrade implementation, deployment and sustained-use acceptance.
