# Choose the database during first-time setup

[#308](https://github.com/arc-media-server/arc-media-server/issues/308)

A new install starts at **Choose a database** before Arc creates a persistent database.
SQLite is selected and needs only Continue. PostgreSQL is an advanced choice and needs a
dedicated empty database on PostgreSQL 16–19. Existing installs with an Arc SQLite database
skip the chooser. The existing account setup API continues to choose SQLite by default
when an older client submits a valid first-time account request.

## PostgreSQL setup

1. Create a dedicated database owned by the account Arc will use.
2. Open Arc directly from your home network. Choose PostgreSQL and enter the host, port,
   database name, username and password. Arc verifies TLS by default. On a trusted private
   connection, you can explicitly connect without TLS.
3. Continue with the usual admin profile and library setup. Keep the Arc data folder as well
   as the database. The data folder holds the private connection record, uploaded artwork,
   generated files, certificates and backups.

PostgreSQL initializes in one transaction. Arc saves a private pending choice before the
transaction commits and confirms its installation identity on restart. A bad login or a
nonempty database leaves the chooser active. A selected database that is offline, missing or
belongs to another Arc installation stops startup with a diagnostic. Existing SQLite data
stays independent of the PostgreSQL setup.

Manual and automatic PostgreSQL backups use `.pg.zip`, including safety archives before
Arc schema changes. Backups are shown in Settings. To recover one, use the
[PostgreSQL recovery tool](2026-11-23-postgresql-upgrade-recovery.md#rehearsal-and-recovery)
with a new empty PostgreSQL database. PostgreSQL Optimize DB refreshes query statistics.

[Onboarding screenshots](2026-11-23-database-selection/)

## Logging

Setup logs the chosen engine or one safe failure category. Connection details, passwords
and request bodies stay out of logs and diagnostic downloads.

## Validation

- Disposable PostgreSQL 16.10, 17.6, 18.0 and 19.0 passed the race-enabled startup, onboarding,
  upgrade and recovery checks. The rehearsal covered wrong credentials, cross-origin refusal,
  successful selection before profile setup, manual backup, reopening a pending selection and
  refusal to start on an unavailable PostgreSQL database. The data directory stayed free of SQLite.
- A scratch SQLite database with an existing admin and no choice record resumed directly on its
  original store. Go vet, the default Go suite, production web and embedded builds and app/E2E
  TypeScript passed. Generated API contracts and the startup-related Go checks passed after the
  last route adjustment.
- Five focused pinned-Chromium journeys passed, covering the database chooser and SQLite setup,
  legacy v1 setup, SQLite backup restore, browser protections and the served API documentation.
  Earlier attempts caught a setup fixture step and the missing pre-onboarding OpenAPI route.
  Those cases passed after correction.
- The fixtures exercised password authentication on a trusted loopback connection with TLS
  disabled. A deployment with a real database certificate, owner data, and the full browser
  suite were outside this check. The scratch Arc processes and database containers were stopped.
