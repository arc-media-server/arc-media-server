# Database and Backup settings

Settings now uses **Database and Backup** for the page at `/backups`. SQLite
keeps its existing backup and restore controls. PostgreSQL gets connection fields,
a description of its `.pg.zip` backup format, and a link to its recovery procedure.

The admin enters the database password to verify and save a change over HTTPS or
local loopback. Arc checks the credentials, schema permissions and that the address
reaches the same live database. The running connection stays active until Arc
restarts. The password and connection URL are excluded from responses and logs.
Complex connections and changes while Arc cannot start still use the offline
recovery procedure. This change needs no database migration.

Arc logs a successful configuration save and whether it needs a restart. A
persistence failure emits a warning without addresses or credentials. Validation
errors are returned to the form.

## Validation

The SQLite Go suite, vet, production web and embedded builds, and five pinned
Chromium journeys passed. PostgreSQL 19 tests confirmed saving and reopening the
same database after an Arc restart, rejecting invalid credentials, and rejecting
a second database with the same Arc identity. An existing download revocation
test failed in the broader PostgreSQL run and passed a focused race recheck.
Browser coverage used a fixture response for the PostgreSQL form, with actual
PostgreSQL verification exercised by Go. No owner server was used.
