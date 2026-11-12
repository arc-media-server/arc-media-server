# Recently Added episode batches

Owner-approved direction, 12 November 2026: episode / season / series grouping following the
pattern described on Plex's forum, with an explicit Arc 24-hour window. Plex's exact internal
interval is unverified. This plan preserves actual arrival history, access rules and row options.

## Contract

1. Start with the existing access-filtered, available episodes ordered by genuine added time and
   arrival tie-breakers. Reuse the current complete-parent query; do not reintroduce a pre-group
   episode cutoff or let one bulk import starve the row.
2. Group by show and arrival batch before looking at season. The newest episode anchors a fixed
   86,400-second span: exactly 24 hours is included; older episodes form a separate batch. Do not extend
   an anchor through a chain of additions. This is elapsed time, independent of midnight/DST.
3. One episode gives an episode card, including a newly imported show. Multiple episodes entirely
   in one current display season give a season card. Multiple seasons give a series card. Multiple
   episodes with an unknown/unplaced season conservatively use a series card; no episode vanishes.
4. Apply the card limit to complete batches, newest first. Keep counts, timestamps and IDs tied
   to those episodes; watch/unwatch and metadata refresh do not regroup or rewrite arrivals.
5. Preserve existing API fields, row styles/shows-only option, season-artwork eligibility/fallback
   and item navigation/actions. Add the batch timestamp to React keys for repeated season/show
   cards. No schema, scan, provider call, source-file mutation or public API removal.

## Implementation and verification

- Add failing real-store/API regressions for new/established shows, same/multiple seasons,
  exact/over 24-hour and non-chaining windows, repeated batches, complete-group limits, unplaced and
  unavailable episodes, access denial, watched/refresh stability and display-season mapping.
- Replace only the grouping/classification logic; preserve source query and cached order lookup.
- Add focused UI coverage with explicit batch fixtures for repeated card identity, counts,
  images/fallback, navigation and refreshed row state. Pair these with actual API regressions;
  injected UI cards do not prove the server's algorithm.
- Regenerate API documentation after updating the route description; retain every v1 contract
  entry. Run Go vet/tests, affected API/store race checks, production/app/e2e TypeScript and the
  affected browser journeys with prepared tools/local fixtures. Inspect desktop/phone captures.
- Record actual results and any failed initial assumptions in the PR/change note. Open a codex PR;
  do not merge to main. No owner media/settings/keys, native repos or persistent server.

## Diagnostics

Grouping is a read-only page request. Existing store/ordering failures already report at their
request boundary. Add no successful-page, per-episode or progress logging; no new provider work
or expensive disabled-log preparation. PR records scope and remaining device/data limitations.

## Comparison sources and client handoff

- [Plex official big-screen navigation guide](https://support.plex.tv/articles/navigating-the-big-screen-apps/)
  describes season stacking; it does not specify a precise arrival window. Its indexed content
  was accessible during investigation; direct retrieval returned 403.
- [Plex forum explanation](https://forums.plex.tv/t/organization-of-recently-added-in-shows/863498)
  describes episode/season/show grouping. This is community/support discussion, not an official
  API contract or a controlled Plex reproduction. Arc deliberately specifies its own 24 hours.
- `GET /api/v1/libraries/{id}/recent-shows` retains the existing response shape. `added_at` is
  the newest episode's arrival timestamp in that card. Multiple cards can share an item ID
  and season; include type, item ID, season and `added_at` in UI keys. Re-fetch after library
  changes; do not independently merge cards by show or season. Ordinary item actions still
  target the item's ID, not the batch. Existing shows-only preferences bypass these cards.
- Implemented in `internal/api/recent.go` and Home's existing row. Regression coverage lives
  in `internal/api/recent_batches_test.go` and `web/e2e/recent-batches.spec.ts`. Actual validation
  results and limits are recorded in the dated change note and PR.
