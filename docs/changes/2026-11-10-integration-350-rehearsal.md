# Combine the #350 review fixes for an isolated integration rehearsal

The independently tested review PRs share API, scanning, playback, lifecycle and test foundations. Combine pinned heads #351–#417 in a separate codex branch based on main d94283c, preserving each original head and resolving 29 conflicting merges across 20 files. Main and the original PRs are unchanged; this rehearsal is not authorization to merge or publish.

The review handoff in `docs/INTEGRATION-350-REHEARSAL.md` lists every pinned PR and resolution. Preserve atomic rate limiting alongside constant-time friend checks, cancellation/storage/Dolby Vision/scaling behavior together, transactional library changes and typed errors, bounded bulk/import work under complete process shutdown, safe webhook retries, shared filters with narrow playlist summaries and both allocation regressions. Remove a duplicate import admission wrapper introduced by automatic merging. Regenerate additive API documentation from the combined handlers.

Two settings test callbacks needed the newer before/next signature. A backup-restore logging assertion now distinguishes its one request error from the separately required scan-resumption failure after a deliberately closed database; it still checks exact counts/messages/levels. No production diagnostic is suppressed.

Actual checks, original failed attempts and limitations are recorded in the handoff and PR. The complete test results are filled in after execution. Use only disposable stores, generated clips, local providers and named container volumes. No owner data, media, keys, Unraid or client repositories. PostgreSQL PRs #337/#339 are excluded and need separate compatibility work.

No new runtime logs or settings. Existing failure boundaries and bounded background diagnostics remain responsible; successful reads add no info logging, and no secrets, bodies, raw URLs or SQL arguments are recorded. Physical HDR/GPU/receiver/NAS, sustained-use and production acceptance remain separate.
