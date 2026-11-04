# PostgreSQL: Claude frontend/API handoff

Backend work for #308, based on main ad2f538. Add this to an advanced server Database section,
reusing existing settings/forms/confirmation/status components. SQLite remains recommended;
ordinary users should need no new setup or decision. This PR implements backend/API only.
Read POSTGRESQL.md before presenting support or recovery promises.

## Admin endpoints (also under /api/v1)

| Method/path | Purpose | Success |
| --- | --- | --- |
| GET `/api/v1/database` | Engine, transfer availability/state, portable backup format, last verified table report | 200 |
| POST `/api/v1/database/test` | Read-only connection/version/empty-target/create-rights preflight | 200 |
| POST `/api/v1/database/transfer` | Explicitly confirmed maintenance and journaled move/restart | 202 |

All three require an authenticated admin profile. Ordinary profiles get 403. Use existing cookie
or Bearer authentication. Connection tests are excluded from change-event mutation hints.
Request JSON is bounded to 8 KiB, rejects unknown fields/trailing data and never echoes passwords.
No database connection details are returned by the status endpoint.

Preflight body:

```json
{
  "host": "postgres",
  "port": 5432,
  "database": "arc",
  "username": "arc",
  "password": "<write-only value>",
  "tls_mode": "verify-full"
}
```

Host/database/username are required. Port 0/omitted means 5432. Missing TLS mode means verify-full;
choices are verify-full or explicitly disabled on a trusted network. No arbitrary DSN is accepted.
A private CA must be trusted by the server; deployment PGSSLROOTCERT is also supported. Never
persist passwords in client storage, query strings, telemetry or error-report payloads.

A successful test returns `{version: 170006, empty: true, can_create: true}` (version number follows
PostgreSQL's server_version_num encoding). Failure returns existing `{error: ...}` JSON with 400;
messages distinguish safe auth/TLS/network categories without connection strings or row contents.
A successful connection to a populated target returns empty:false; do not enable Move for it.

Move body wraps the same connection and requires explicit downtime consent:

```json
{
  "connection": {
    "host": "postgres", "port": 5432, "database": "arc", "username": "arc",
    "password": "<write-only value>", "tls_mode": "verify-full"
  },
  "confirm_downtime": true
}
```

Success is 202 `{state:"requested"}`. Missing/false consent or invalid JSON is 400; unavailable,
concurrent, invalid or unsafe transfer is 409. Arc repeats preflight when Move is requested;
a prior successful test is not permission to overwrite a changed/nonempty target.

## Frontend flow

Detect `database_management` through `/capabilities` first: supported/available/allowed must all
be true. This is a wiring/role hint, not a connection test. Show SQLite and "Move to PostgreSQL"
only for an admin with transfer_available:true. Explain
that this uses an existing empty database and needs a brief service interruption. Present a
connection form, Test connection, safe inline validation and an explicit maintenance confirmation.
Mention active playback/background work will stop, original SQLite/safety backup is retained,
file-backed assets/media mounts stay in place, and that reverting to old SQLite loses later writes.
Do not promise faster scanning, uninterrupted playback, multi-server operation or easy rollback.

After 202, disable resubmission, show reconnecting/maintenance and retry status with backoff,
respecting 503 Retry-After:5. HTTP may briefly disappear while copying and restarting. The server
has no live progress HTTP endpoint while offline; do not invent percentage/count progress. On
reconnect, require backend:postgres and state:ready, then refetch identity/capabilities/settings,
libraries and current views using normal endpoints. Existing sessions/passwords remain valid.
No setup wizard, forced rescan, cache reset or rematch should be offered as part of this flow.

Status example:

```json
{
  "backend": "postgres",
  "transfer_available": false,
  "state": "ready",
  "backup_format": "arc-sqlite",
  "last_report": {
    "verified": true,
    "rows": 12345,
    "tables": [{"table":"items","rows":400,"sha256":"<canonical checksum>"}]
  }
}
```

The actual report includes all 43 persistent tables. Counts are database records, not titles,
episodes or percent completion. States are ready/requested/snapshotting/copying/verified; the
latter phases normally occur during offline startup recovery. No credentials or per-record data
are included. A missing last_report is normal for an existing SQLite instance.

If recovery cannot finish, startup stops with a safe log message and retains pending journal/source.
The UI cannot change a stopped server. Offer honest troubleshooting, never "start fresh": fix the
connection and restart, or stop Arc and explicitly cancel the pre-cutover journal using the CLI
in POSTGRESQL.md. A partial safety snapshot may need explicit cancellation before retry. A new
attempt requires an empty destination. There is no online cancel endpoint in this backend version.

## Existing administration

Backups stay on existing screens/APIs as `.db` files and use existing retention, safety restore
and admin download rules. Add help that PostgreSQL backups are portable Arc SQLite-format data
snapshots, not uploads/certificates/generated assets. Do not remove file-backed storage guidance.

For PostgreSQL Optimize database, say it updates statistics; vacuuming belongs to PostgreSQL.
Do not claim SQLite integrity/compaction/WAL work ran. The result adds optional backend:"postgres";
all existing v1 fields remain. No frontend/backend compatibility break is intended.

The bootstrap stores protected credentials locally and does not need a UI editor. Keep the
advanced feature labelled according to actual deployment readiness: this backend has an exact
schema-version guard. Future schema releases need a tested PostgreSQL upgrade path; generated
baseline regeneration alone is insufficient. Do not close #308 until UX, deployment and upgrade
acceptance have been reviewed. Native and physical-device acceptance remain separate.
