# Profiles and backups translation preparation — 28 October 2026

Part of #122, independent codex/i18n-profiles-backups branch from main ad42920.
Profiles/friends/invites, PINs/restrictions, backup policy/status and restore/delete confirmations use a dedicated bundled English namespace, static readable keys, complete phrases,
rich Trans markup and noun-specific plural families. Provider/user values, source/API enums and
server errors/logs remain raw. No dependencies, non-English translation, picker, API/schema or CSS changes.

Actual checks after the final independent source review:
- Production/e2e TypeScript, production build and embedded-server build passed.
- 16/16 existing i18n/catalog/plural/fallback checks passed.
- All-package Go vet and 449/449 Go tests/subtests passed (275 top-level tests; 30 test-bearing
  packages, six without tests), no Go failures/skips.
- 6/6 unchanged focused browser journeys passed; zero skips/flaky tests/retries.
- Two additional desktop/phone visual journeys passed. Four desktop/phone pairs: three byte-identical; one desktop header icon differs at six pixels.
  Compared with separately built clean main ad42920, with no wording/layout change found.

Complete admin/PIN variants retain English help without appending translated sentence fragments. Server backup reasons, dates and errors preserve their data boundaries.

Earlier pre-review results remain historical evidence; the reviewed checks above supersede them.
The first administration browser run was interrupted by a usage-limit environment replacement
before its final report; it is not counted as a complete pass. No existing tests/selectors changed.
Prepared Go1.27.0/Node24.19.0/npm11.9.0/pinned Chromium rev1248, temporary stores, generated clips,
local provider fixtures only. Screenshots cover representative empty/new report states; they do
not establish acceptance of every populated metric or actual translation. No owner media/settings,
production keys, Unraid, persistent server or physical-device/deployment acceptance claim.
Evidence: /workspace/arc-cloud-evidence/i18n-remaining/administration-*.

Keep unmerged for the owner's batch review. Final integration combines the areas, checks stable
internal tokens, removes the legacy plural bridge, documents the new-UI-text rule and runs the
combined browser suite. Keep #122 open pending review and future first-language work.
