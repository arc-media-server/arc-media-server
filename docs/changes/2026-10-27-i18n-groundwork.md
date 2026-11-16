# Translation groundwork (#122, Phase 1, 27 October)

The shared web interface now reads its existing English text from bundled `common` and `auth`
namespace JSON catalogs using react-i18next. Converted areas: the shell/not-found state,
desktop/mobile navigation, profile and Settings navigation, Activity and News menus, shared
error/retry and dialog controls, horizontal-row accessibility text, setup, sign-in, profile/PIN
selection and invitation acceptance. Buttons/fields/Page that only receive caller content keep
those APIs; page-specific Save/Cancel labels and feature dialogs are converted with their area.

English is available synchronously on first paint. Only languages actually present in the build
are detected from the browser and loaded as separate chunks; missing/failed chunks and keys use
English. No translated language, language picker, service account, server change or new setting
ships here. Only i18next, react-i18next and i18next-browser-languagedetector are new direct
dependencies. Date/number/language-name helpers follow the resolved locale and preserve existing
English presentation, including file-size rounding. Existing server errors/task text, metadata,
route/anchor/storage identifiers and `/docs` remain outside the conversion.

`npm run test:i18n` audits static TS/TSX references for missing/unused English keys, including
namespaces, rich text and plural families. It runs during `npm run build` and also verifies
formatting and real initialization/detection/loading/failure fallback with isolated instances.
`docs/TRANSLATION.md` explains keys, placeholders, plurals, Trans, adding language files and
later Weblate integration. Existing Playwright tests and selectors remain untouched.

Validation on main base `bc272ab9b0544744cbeaf24a63949baab765f2e9`, Linux amd64, prepared
Go 1.27.0 / Node 24.19.0 / npm 11.9.0 and pinned Chromium 156 revision 1248:

- Full browser suite: **113/113 passed** in **679.637 seconds**, no failures, skips, flaky
  results, retries or suite errors (111 unchanged existing journeys plus 2 new fallback checks).
- All-package `go vet ./...` and `go test ./...`: **444/444 tests/subtests**, 270 top-level tests,
  30 test-bearing packages, no test failures/skips. Seven other packages contain no tests.
- Production `npx tsc -b`, e2e TypeScript, `npm run build` and the real embedded-server build passed.
- **11/11** translation/formatting/audit checks and **6/6** adaptive-policy checks passed.
- **26 desktop/phone screenshot comparisons** match separately built clean main: 25 byte-identical;
  one phone confirmation differs only by one RGB level at 360 blurred-background pixels. Text,
  layout and controls are unchanged. Temporary folder labels were normalized for this comparison.

Evidence is retained outside the repository at `/workspace/arc-cloud-evidence/i18n-groundwork/`.
The new browser tests initially assumed API setup stops at the profile picker, although it selects
the administrator directly; only those new tests were corrected. The initial screenshot harness
also had overlapping runners; the isolated main comparison replaced that nonpassing attempt.
The extra visual harness first compared random temporary path text behind a modal; after fixture
normalization its exact-byte assertion exposed only the one-level blur rounding described above.
The catalog audit caught and corrected an auth reference to a common label. Original logs remain.
No existing journey or selector was changed.

This UI foundation does not establish native/physical-device, translated-language, RTL,
production-container or owner-server acceptance. Race tests were not run for this UI-only phase.
No owner data/settings/media, real provider keys, Unraid or persistent server was used. No PR
Actions are expected under the current policy; all results above are local.

## Remaining phases

After this PR merges, start a fresh `codex/i18n-…` branch from latest main for each area:
Home/Watchlist/Playlists; browsing; title pages and feature dialogs; Player; Settings in smaller
PRs; management tools. Keep each PR around 2–4 pages/components, list touched files and checks,
compare desktop/phone screenshots, and merge main if needed instead of rebasing/force-pushing.
Issue #122 stays open. Add the all-new-UI-text rule to CLAUDE.md only after all areas are done;
its “Where things stand” section is unchanged in this PR.

## Claude review follow-up

Addressed all six points from PR #235 comment 6448220704: compound settings keys use camelCase;
counted-word messages have explicit one/other families (lookingThroughFolders already had them);
React components use plain t, with Activity locals renamed task; Intl.DisplayNames instances
are cached by locale. Documentation explicitly requires each area to replace its fmt.plural
callers and the final phase to remove the helper/catalog, and records en-GB for all English
browser variants as an intentional existing default pending the owner's later #80 decision.

The build audit now rejects long lowercase compound segments (notifications is a genuine-word
exception) and counted-word keys without plural pairs. Three new checks cover these rules and
actual DisplayNames construction counts across locale switches. English plural entries retain
existing wording, even where the displayed English is invariant. No dependency/API/layout change.

Fresh local review validation: 14/14 i18n/audit tests, production/e2e TypeScript and build, all-package
Go vet and 444/444 Go tests/subtests, full 113/113 browser journeys in 670.437 seconds with no failures, skips, flaky
results or retries. All 18 repeated desktop/phone shell/auth screenshots are byte-identical to
clean main. Screenshot results and exact run duration are in the review evidence folder:
/workspace/arc-cloud-evidence/i18n-review/. Earlier evidence above remains historical.
