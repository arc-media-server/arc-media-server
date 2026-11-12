# Atomic profile creation

Validate library restrictions before any write and insert restrictions, download grant and friend invite state in one SQLite statement. A rejected request cannot leave a new unrestricted profile. Existing setup/default/all/none and invite semantics remain.

Quality review CQR01, based on the combined pending-review snapshot.

Regression failed before the fix. Go vet and full Go tests (`go test -p 1 ./...`) pass; focused API/store race tests pass; two pinned-browser profile/switching journeys pass. An earlier concurrent full run failed the unchanged HDR picture-caption test; serial full verification passed, and the initial log is retained.

Logging: existing request failure logging remains responsible; expected validation and successful profile/list reads need no extra logs. No secrets, bodies, raw URLs or SQL arguments are added to logs. Temporary fixtures only; no production/device acceptance.
