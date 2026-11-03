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

Validation results are recorded after final checks. Prepared tools and disposable PostgreSQL
containers/temporary fixtures only; no owner database, keys, media, Unraid or persistent server.
