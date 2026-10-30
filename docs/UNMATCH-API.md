# Unmatch show: backend handoff

`POST /api/v1/items/{series-id}/unmatch` requires an admin profile. No request body. Returns the
updated item (200), including `metadata_state:"unmatched"` and `match_suppressed:true`.

Claude: add Unmatch to the existing show action menu and confirmation dialog. Explain that it
removes downloaded metadata/artwork, manual metadata edits/locks and all display-order overrides.
Filename-derived labels and a file ordering replace provider information. This deliberately clears
uploaded artwork references too; it never deletes upload files or writes to media folders. A normal
metadata refresh does not undo it; the user chooses Fix Match to identify it again.

Preserved: show/episode IDs with file or user references, file links, watch/resume/history, playlists,
manual collections, arrival history, technical file/probe/chapter/marker data. Access labels are
retained because they are access policy, not provider enrichment. File episode numbering is retained
because it describes the user's filenames; display orders/custom placements are reset. Unknown age
is restored for parental restrictions. Automatic provider collection membership ceases naturally when
provider collection IDs are cleared; manual collection membership remains.

Provider-only cached episodes are removed only if no file, watch state/history, playlist or manual
collection references them. Ambiguous multi-episode/date identities stay visible unplaced rather
than being guessed/deleted. Ordinary numbered files keep their known episode IDs; new files still
appear during scans. Local NFO/artwork does not silently re-identify a suppressed show.

401: sign in/select profile. 403: admin needed. 400: endpoint accepts shows only. 409: library work or
background Fix Match is active; wait and retry. 503: scanner unavailable. Storage failures use the
existing 500 envelope. A transaction failure leaves the prior metadata intact. A cancelled request
before commit rolls back. After commit, client reconnect can read the item to determine outcome.

`POST /items/{id}/refresh` returns 409 for explicitly unmatched items. Explicit Fix Match remains
`PUT /items/{id}/match {"provider":"tmdb"|"tvdb","id":...}`. A successfully chosen provider clears
suppression and uses existing identity-preserving adoption; a failed lookup leaves the reset state.

The new field is additive and defaults false on existing items (migration 0061). Suppression survives
restart/backups. New runtime capability: `unmatch_show` (supported, availability, allowed).

Logging: one successful outcome with safe item/library IDs, episode count and elapsed time. Storage
failures use the existing failure boundary. Busy requests, normal polling and expected cancellation
add no error floods; no provider keys, raw paths/URLs or request bodies are logged.

This PR depends on API foundations (migration 0060 and contract fixes). UI and native-device
acceptance are left to Claude/the owner; generated/temp fixture checks do not establish deployment.
