# Library editor and folder picker translation preparation — 28 October 2026

Part of #122, separate codex/i18n-library-editor branch from main 7b1e4d3.
Add/edit library controls, source ordering, metadata language, preview/progress and folder selection use a dedicated bundled English namespace, static keys and noun-specific plural
families. Technical IDs, matching tokens, paths, metadata, user edits and server results stay raw.
English wording, labels, defaults and layout are preserved. No dependencies, translated language,
picker, API/schema or CSS changes. This is translation preparation, not another-language acceptance.

Actual isolated validation:
- Production/e2e TypeScript, production build and embedded-server build passed.
- 16/16 existing i18n/catalog/fallback checks; Go vet and 449/449 Go tests/subtests passed
  (275 top-level tests; 30 test-bearing packages, six without tests). No Go failures/skips.
- 8/8 unchanged focused browser journeys passed; zero skips/flaky tests/retries.
- Two additional desktop/phone visual journeys before and after passed. Four desktop/phone pairs differ at 214–679 pixels, within fixture path/text rendering regions.
  Compared with separately built clean main 7b1e4d3. Source/visual review found no wording or
  layout change; this is not an all-pixel-identical claim. Random item file roots are normalized
  only in fixture GET presentation; library folder paths remain random and explain differences.

Prepared Go1.27.0, Node24.19.0, npm11.9.0 and pinned Chromium rev1248; no tools installed.
Temporary stores, generated clips, local provider fixtures only. Existing browser tests/selectors
unchanged. No owner data/settings/media, production keys, Unraid, persistent server or device/
deployment/full-suite acceptance claim. Evidence: /workspace/arc-cloud-evidence/i18n-remaining/library-editor-*.

Keep unmerged for the owner's batch review. Final integration removes unused raw compatibility
labels/hints after every caller has converted, audits phrase cohesion and documents all areas.
Do not close #122 from this PR.
