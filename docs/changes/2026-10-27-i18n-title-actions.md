# Title actions, collections and preplay translation preparation — 27 October 2026

Phase 4d of #122, independent codex/i18n-title-actions branch from main 7b1e4d3.
Collections dialogs, bulk selection/editing, optimization and preplay options now use a bundled English namespace, static readable keys, complete phrases,
explicit label maps and plural families. English wording, punctuation, accessible names,
API behavior and layout are preserved. No translations, picker, dependencies or server schema changes.

Actual validation with prepared Go 1.27.0, Node 24.19.0, npm 11.9.0 and pinned Chromium rev1248:
- Production/e2e TypeScript, production build and embedded-server build passed.
- 16/16 existing i18n/catalog/plural/fallback checks passed.
- All-package Go vet and 449/449 Go tests/subtests passed (275 top-level; 30 test-bearing
  packages, six packages without tests). No Go failures/skips.
- Existing focused browser journeys: 8/8; no skipped or flaky tests or retries.
- Two additional desktop/phone visual journeys passed. Eight desktop/phone pairs: six byte-identical; two differ at seven background pixels each.

The collection card belongs to browsing PR #238; its legacy grid-label export is retained until integration.

Temporary stores, generated clips and local provider fixtures only. Existing browser tests
and selectors are unchanged. Screenshot fixture GET presentation normalizes random media roots;
Settings uses a fixed fixture maintenance clock. No owner media/settings, production keys,
Unraid, persistent server, physical-device, deployment or full-suite acceptance claim.
Evidence: /workspace/arc-cloud-evidence/i18n-remaining/title-actions-*.

Keep this PR unmerged for the owner's batch review. The other #122 areas are independent
PRs; final integration will resolve shared imports/sections and remove compatibility helpers
only after every caller is converted. Do not close #122 from this area PR.
