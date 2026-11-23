# Experimental PostgreSQL runtime

[#308](https://github.com/arc-media-server/arc-media-server/issues/308)

Arc's integration tests can open a real PostgreSQL store and run the normal HTTP server on it.
The server binary continues to use SQLite. The first-run database selector and the PostgreSQL
backup and upgrade path are separate work.

The store reuses #337's PostgreSQL query adapter and #509's schema generator. It initializes
an empty database in one transaction, including settings and migration history, and refuses
schema fingerprints it cannot open. A single advisory-locked writer and a separate read-only
pool keep the existing write queue model. The writer acquires the lock before executing any
SQL on a new connection. SQLite-only maintenance is explicitly unavailable on the experimental
store.

If the current SQLite migrations have changed during development, regenerate the embedded
first-install schema from the repository root:

```sh
go run ./tools/dbschema > /tmp/arc-postgres-baseline.sql &&
  mv /tmp/arc-postgres-baseline.sql internal/store/postgres/baseline.sql
```

Run `go test ./cmd/ams -run '^TestPostgresScratchServer$' -count=1` with
`ARC_TEST_POSTGRES` pointing to a dedicated empty fixture database and role. The test starts
Arc on loopback with a temporary data directory and seeds a catalog record and file link.
It does not scan or play media. Use a disposable database per run.

The driver returns bounded failure categories and safe SQLSTATE codes. It never reports
connection details or SQL arguments in its errors. Normal successful requests add no log lines.

## Validation

- PostgreSQL 19.0 with a regular database owner: Arc setup, catalog and search, watchlist
  writes and reads, second-writer rejection, reopen, and playback-history insertion passed.
  The test also verified that the Arc data directory did not contain a SQLite database.
- Go 1.27.0: vet, embedded build, baseline regeneration comparison and focused PostgreSQL race
  checks passed. The initial full Go run found a SQLite regression in optimized-copy links.
  The fix preserves all source links and narrows the change to the all-copies list. The complete
  store and server packages passed after the fix. The full suite was run once.
- Production web build, app and E2E TypeScript, and the pinned Chromium SQLite onboarding journey
  passed. The first build ran out of space in the workspace, so validation used a temporary checkout.
- PostgreSQL checks covered one version and a small catalog. Production deployment, scan and
  playback, schema upgrades, backup restore, performance and comprehensive query parity remain
  unverified. The scratch Arc process and PostgreSQL container were stopped.
