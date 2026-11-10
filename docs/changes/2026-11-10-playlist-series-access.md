# #350: do not expand restricted shows into playlists

The real playlist route reproduced a restricted show expanding into episodes without checking the show's visibility. A mixed selection returned added: 4 and stored all four entries, including three hidden episodes, while a missing title returned a generic error. Check the parent show's CanSee policy at the shared orderedEpisodeIDs boundary before reading its episodes. Reuse existing library, label and age rules; episode visibility is inherited from the parent. No per-episode access-query loop or new policy is introduced.

Hidden and nonexistent selections use the same existing safe validation message. All expansion completes before AddToPlaylist, so a mixed invalid selection writes no entries. Visible shows retain their normal order and unplaced-episode handling. The helper is also used by smart playlists and play queues; their existing checks remain.

Tests exercise library, label and age restrictions, mixed-input no-write behavior, missing/hidden response equivalence and visible-show controls, plus existing inline-special/queue tests. Fixtures contain temporary store records and synthetic paths only; no media reads/writes, provider credentials or persistent server.

Integration: preserve #389's independent inputError classification at playlist callers and database-error propagation when merging the overlapping file. This fix does not duplicate that PR's wider error-boundary changes. No success-response shape, new frontend control or API contract type change.

No new logs or jobs. Expected hidden/missing input remains ordinary validation, and no item count or private title is logged. No secrets, raw URLs, request bodies or SQL arguments are recorded. Required validation results follow when checks finish.

Validation: the library/label regressions failed against unchanged main with hidden counts and stored entries. Corrected library/label/age cases and existing inline-special/play-queue controls passed. API-doc generation, Go vet, full Go suite and complete API race tests passed (258.707s). No generated API/contract diff; no browser checks required.
