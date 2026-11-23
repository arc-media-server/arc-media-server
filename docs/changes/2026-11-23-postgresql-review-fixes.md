# PostgreSQL reconnect, backup and recovery safeguards

[#308](https://github.com/arc-media-server/arc-media-server/issues/308)

Arc checks the selected installation ID when either connection pool opens a connection.
A wrong identity stops the operation and closes the rejected connection, releasing any
advisory lock. The first connection still initializes and verifies a new selection
through the existing bootstrap transaction.

The Docker build includes the version-matched recovery command. TCP recovery defaults to
certificate and hostname verification and rejects insecure TLS fallback. An explicit
private-network opt-out remains available. The recovery guide covers protected setup,
coordinated backups, restore and cutover on a separate database, and reversal.

Manual and automatic PostgreSQL backups sync their directory before success or pruning.
They share the existing durable publication step for pre-upgrade backups. No schema or
API changes are needed.

Logging uses Arc's existing safe database errors. An identity mismatch names the problem
without exposing the saved ID, DSN or credentials. No new query logging is added.

## Validation

The PostgreSQL 19 race gate passed. It covered refusal after both pools reconnect,
release of the rejected writer's lock, restore, manual backup, and the recovery
configuration's verified TLS defaults and explicit opt-out. Go vet, the default Go
suite and container-style builds of the server and recovery command passed. The
Docker image, a live replacement-host disconnect and real certificate handshakes
remain for the deployment rehearsal.
