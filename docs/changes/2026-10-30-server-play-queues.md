# Persistent per-device play queues

Added backend queue CRUD and revision-checked append/Play Next/remove/reorder/clear/select/advance/
shuffle operations. Queues belong to one profile/session, have distinct duplicate entry IDs, preserve
current/past order, snapshot playlists and follow existing series ordering expansion. They survive
restart/backup and expire after seven inactive days; revocation cascades deletion. Access changes
produce metadata-free placeholders. Maximum 1000 entries, bounded request bodies and hourly-or-less
read lease writes. Queue header and entries use one consistent database statement snapshot.

No player controls or UI changed. Queue position does not start/stop/seek playback or mark watched.
File/edition/track choices still use existing playback APIs. Claude integration: docs/PLAY-QUEUES-API.md.
Cleanup and Unmatch protect queue-referenced episode identities. No new dependencies or external
service. This PR is stacked after foundations and Unmatch migrations.

Logging: one safe creation summary (queue ID and count), no edit/poll info traffic, existing storage
failure boundary. No session hashes, tokens, raw paths/URLs or bodies. Actual validation: full Go tests pass across 31 tested packages (six have no tests), full vet, queue store/API race checks, generated OpenAPI/additive v1 checks and embedded-server build pass. A final saved-reference cleanup safeguard passed focused store/scanner/API race checks after that full run. The first full run caught the missing session foreign-key index; it was added and the full suite passed on rerun. Existing production UI build is reused because no frontend source changed; no browser suite was repeated.
no physical-device/owner-server or frontend acceptance claimed.
