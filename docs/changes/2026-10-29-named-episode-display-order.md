# Show matching named episode orders without duplicate follow choices

Owner requested File episode order changes select the matching named Display order (for example,
TheTVDB aired in both fields), while retaining the ability to choose a different display afterward.

- Existing inherited state renders its resolved named order; simply opening Edit, cancelling or
  saving unrelated details does not write a display choice. Existing explicit/custom choices remain.
- A real File episode order edit stages its matching named display, with a later manual selection
  taking precedence. Save refreshes only that show and resolves the resulting order ID before
  pinning it. An unresolved library-default source is resolved from the server after refresh.
- The duplicate follow/library-default display option is removed. Legacy inherited rows/API reset
  requests remain compatible; no schema, order entries, files or history are reset.
- Compare uses the staged display for its preview. Inside Edit, Use stages a selection; outer Save
  applies it and Cancel discards it. The shared comparison component retains its immediate mode
  for callers outside Edit.
- A missing matching order produces a recoverable error rather than pinning another provider’s
  fallback. Source changes can already have saved before display errors. The form explains this,
  and updates the item cache after a successful source response so Cancel/reopen/Retry use the
  actual server state. This is not a promise of transactional rollback across provider work.

Logging review: this changes a staged UI workflow, not background scheduling. Existing scoped
refresh Activity/error handling and the form’s recoverable error/Retry cover failures; adding
per-select or successful-page-load logs would add noise without diagnostic value. No new log
volume, provider, dependencies or data migration.

Validation on the prepared cloud tools (Go 1.27.0, Node 24.19.0, pinned Chromium rev1248):

- UI production build, app TypeScript, e2e TypeScript and 20/20 translation checks passed.
- 6/6 focused browser journeys passed together (49.2 seconds), with zero failures/skips/retries:
  named source-to-display selection, independent later choice, inherited state without extra
  writes, Compare staged preview/Use/Cancel, Save failure/Retry, explicit and inherited partial-save
  cache recovery, unavailable DVD-order recovery, phone geometry, owner/profile restrictions,
  scoped refresh with no discovered/probed files, and preserved episode identity/files/resume.
- Existing store `TestDisplayChoiceIsTheServerDefault`, scanner
  `TestFullProviderRefreshAndSourceSwitchPreservesEpisode` and API
  `TestMetadataOrderingErrorsAreVisibleAndRetryable` passed against isolated stores/providers.
- Affected-package vet (`internal/api`, `internal/store`, `internal/scanner`), disposable embedded
  server build and `git diff --check` passed. No Go source or schema changed.
- Phone screenshot inspected: named TVDB aired display matches the file order, aligned fields and
  visible Save/Cancel, without the duplicate follow choice.

Initial focused runs exposed a real stale series-cache issue after a partially saved source
change and two incorrect test selectors (ambiguous Close and an existing unlabeled Title input).
The series/order caches now reflect successful phases, and metadata-edit comparisons retain
opening values so retries do not lock untouched stale provider fields. Final six-case run passes.
Existing Vite configuration/dynamic-import warnings remain. Full browser/Go/race/container suites,
owner-server catalogue checks and physical-device acceptance were not run. Prepared dependencies
were reused; no tools were installed, no production systems/data accessed and no persistent
server started. The source/display save sequence retains its documented partial-save boundary.
