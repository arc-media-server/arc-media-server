# #350: compact backups must not occupy the catalog writer queue

BackupTo ran VACUUM INTO through the store's single writer connection, queuing watch-state
and scanner writes behind a whole database copy. The regression held an uncommitted writer
transaction and reproduced the backup timing out while waiting for that connection.

Keep SQLite's existing compacted, consistent VACUUM INTO snapshot, but run it on a dedicated
source connection opened with mode=ro. Discover the real file path from SQLite and URI-escape
it, including spaces/hash/percent characters; close the connection when the operation ends.
The source cannot be modified. The normal query_only read pool cannot run VACUUM INTO (confirmed
by an experiment), so borrowing that pool and disabling its protections would be unsafe.
No writer queue slot is taken for the copy, and no incremental engine or new configuration is
needed. Caller context/cancellation and existing destination/no-overwrite behavior remain.

Tests prove a backup finishes while a writer stays uncommitted, contains only committed data,
and leaves the writer usable afterward. Additional checks refuse source writes, preserve an
existing backup byte-for-byte, handle special source filenames and respect pre-cancellation.
All stores/backups are temporary. This removes in-process writer serialization; disk/CPU
contention and physical deployment performance are not measured or claimed. Windows path
normalization is included but no physical Windows/NAS deployment is tested.

No API/media changes or new long-lived jobs/logs. Existing backup outcomes and failures remain
responsible for diagnostics; successful page/file operations add no logs. No secrets, raw URLs,
request bodies or SQL arguments are logged.

Validation: focused snapshot/read-only/no-overwrite/cancellation tests, Go vet, the full Go
suite and store race checks passed. One full-suite process was interrupted before completion;
its partial log was retained and the unfinished command rerun using Go's completed-package
cache. Store race: 237.145s. Windows/amd64 test binary cross-compilation passed; it was not run
and does not establish physical Windows/NAS behavior.
