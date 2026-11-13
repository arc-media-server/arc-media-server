# Profile poster-size controls and Home heading/menu

Owner-requested controls: Home heading on the left, poster-size slider beside a three-dot
menu on the right, with Edit Home inside. Library's slider is beside its existing right-hand
menu. Reuse the shared page header, dropdown, preference patch and error/Retry components.

Two additive integer profile preferences save separate Home/Library scales: 75–150%, with
missing/0 preserving100%. Drag locally for immediate preview and save only completed
interactions; no request per movement. Failed saves roll back with Retry; failed preference
reads explain unavailable controls. Ordinary viewers control their own sizes without admin
library actions. Poster rows and grids (including collections/skeletons) share a scoped CSS
scale; landscape/backdrop cards retain their sizes. No migration/dependency or media changes.

API/client contract: docs/HOME-NAVIGATION.md and regenerated OpenAPI/v1 snapshot. Existing
fields remain. No native app edits or claimed native sizing support. Caption PR #449 is separate.

Logging: no per-input/card/request-success logs. Existing preference/request failure boundaries
remain responsible; range errors are ordinary validation, and UI failures use existing notices.
Only completed interactions write the existing small profile record; no background jobs.

Validation: Go vet/full suite (37 tested packages), focused preference/concurrency race checks,
production/app/e2e TypeScript, build and generated API freshness/contract checks pass. All
24 affected browser journeys pass (2.2m, zero failures/skips/retries), including immediate drag
preview with one completed-interaction save, reload/restart persistence, independent Home/
Library/profile values, validation, rollback/Retry, initial read failure recovery, all library
menus/tabs and existing browsing/filtering/navigation. Desktop/phone captures inspected.
Evidence: /workspace/arc-cloud-evidence/poster-size-controls/.
Temporary stores/generated clips/local provider fixtures only; no owner data or deployment
acceptance. The whole browser/race suites are not repeated for this focused preference/UI work.
