# Consistent unwatched playlist aliases

Use the normalized saved-filter query when pruning watched episodes from a smart series playlist. Both unwatched=true and watch_state=unwatched now select the same unwatched episode IDs. No API shape, visibility or ordering change.

Quality review CQR02, based on the combined pending-review snapshot.

Regression failed before the fix. Go vet, full Go tests (`go test -p 1 ./...`) and focused API race tests pass. An earlier concurrent full run failed the unchanged HDR picture-caption test; the final serial run passes.

Logging: existing request failure logging remains responsible; expected validation and successful profile/list reads need no extra logs. No secrets, bodies, raw URLs or SQL arguments are added to logs. Temporary fixtures only; no production/device acceptance.
