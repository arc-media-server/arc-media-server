# Access, device linking and imports translation preparation — 27 October 2026

Phase 6c of #122, independent codex/i18n-access-settings branch from main ad42920.
Remote/home networking, HTTPS/ports, device listing/linking and watched-history imports now use a bundled English namespace, static readable keys, complete phrases,
explicit label maps and plural families. English wording, punctuation, accessible names,
API behavior and layout are preserved. No translations, picker, dependencies or server schema changes.

Actual validation with prepared Go 1.27.0, Node 24.19.0, npm 11.9.0 and pinned Chromium rev1248:
- Production/e2e TypeScript, production build and embedded-server build passed.
- 16/16 existing i18n/catalog/plural/fallback checks passed.
- All-package Go vet and 449/449 Go tests/subtests passed (275 top-level; 30 test-bearing
  packages, six packages without tests). No Go failures/skips.
- Existing focused browser journeys: 6/6; no skipped or flaky tests or retries.
- Two additional desktop/phone visual journeys passed. Four full-page desktop/phone pairs, all byte-identical.

Protocol values, source IDs, user names, server errors and raw device values stay untouched. No production keys or network configuration used.

Temporary stores, generated clips and local provider fixtures only. Existing browser tests
and selectors are unchanged. Screenshot fixture GET presentation normalizes random media roots;
Settings uses a fixed fixture maintenance clock. No owner media/settings, production keys,
Unraid, persistent server, physical-device, deployment or full-suite acceptance claim.
Evidence: /workspace/arc-cloud-evidence/i18n-remaining/access-settings-*.

Keep this PR unmerged for the owner's batch review. The other #122 areas are independent
PRs; final integration will resolve shared imports/sections and remove compatibility helpers
only after every caller is converted. Do not close #122 from this area PR.
