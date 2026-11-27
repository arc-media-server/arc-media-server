# Show when PostgreSQL is unavailable

A connection loss returns HTTP 503 with `database_unavailable` and a retry delay
on affected requests. The web app shows a notice, keeps the signed-in shell
available, and checks for recovery. Once the API responds again, Arc refreshes
the current screen. Retry any action that failed while the database was down.

An already configured Arc server that starts before PostgreSQL serves a
startup status screen while it retries. That listener uses `AMS_LISTEN` or
loopback port 8099. The normal server loads its saved listener and TLS settings
after the database reconnects. Arc keeps its configured database engine.
Unexpected query, schema, and credential errors retain their specific handling.

Startup and running request paths log when an outage is first detected and when
recovery is observed. Credentials and connection details are excluded.

Vet, the default Go suite, API documentation generation, focused API and store
rechecks, web production build, app/e2e typechecks, and one pinned Chromium
outage journey passed. The browser journey used fixture responses. A real
PostgreSQL 19 rehearsal with the local branch build separately verified
certificate-verified TLS, backup and restore into a separate database, and both
startup and running outage recovery. There was no SQLite fallback or lost
session. The prepared Docker workspace ran out of space before it could run the
published Arc image. No owner server or data was used.
