# Profile card-size controls and Home heading/menu

Owner-requested controls: Home heading on the left, card-size slider beside a three-dot
menu on the right, with Edit Home inside. Library's slider is beside its existing right-hand
menu. Reuse the shared page header, dropdown, preference patch and error/Retry components.

Two additive integer profile preferences save separate Home/Library scales: 75–150%, with
missing/0 preserving100%. Drag locally for immediate preview and save only completed
interactions; no request per movement. Failed saves roll back with Retry; failed preference
reads explain unavailable controls. Ordinary viewers control their own sizes without admin
library actions. Portrait posters, landscape thumbnails and backdrop cards share a scoped
CSS scale, including Continue Watching, Up Next, collection grids, Other Videos thumbnail grids
and folder covers; each keeps its aspect ratio. The slider is labeled Card size. Hero banners
and cast portraits remain outside media-card density. No migration/dependency or media changes.

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


## Owner follow-up: include thumbnails and backdrops

Extended the same scale to all three media-card styles without changing the saved preference
fields or ranges. Added real Other Videos folder/thumbnail checks and explicit Home thumbnail/
backdrop presentation fixtures, checking desktop/phone widths, preserved16:9/4:3 aspect ratios
and no document overflow. Follow-up validation: Go vet/full tests (37 tested packages), app/production/e2e TypeScript,
production build and API-document freshness checks pass. All26 affected browser journeys pass
(1.7m,0failures/skips/retries), including all six sizing journeys; desktop/phone captures
inspected. Backend runtime/validation is unchanged in this follow-up, so the earlier focused
preference race result remains historical evidence and is not claimed as a fresh race run.
Actual logs/artifacts: `/workspace/arc-cloud-evidence/card-size-all-styles/`; earlier24-journey
evidence remains intact.
