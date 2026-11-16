# Complete web translation preparation — combined review, 28 October 2026

Arc's reviewed web pages, dialogs, Player, Settings and management tools now resolve app-owned
text from 22 bundled English namespaces (2,042 entries including plural forms). English behavior
and wording remain the contract. This completes the remaining extraction/integration phases of
#122 for review; no additional language, language picker, Weblate service or RTL layout is added.
Server-generated errors/tasks/logs, provider metadata, user values and technical IDs stay unchanged.

## Review and merge

The area PRs are deliberately separate review units. This combined PR integrates their changes
and the final cross-area corrections. **Review the area PRs, then merge the combined PR for #122**;
there is no need to separately merge the overlapping snapshots. Its commit retains all 13 area
heads as parents; use a merge commit to retain that ancestry. Everything remains unmerged until
the owner reviews. Keep #122 open pending review and first-language acceptance. Other agents'
branches/PRs are untouched. GitHub main remains 7b1e4d3b7078780f63efce387ebc4c92d600c212.

| Area | Review PR | Validated area head |
| --- | --- | --- |
| Browsing | [#238](https://github.com/JonzaUK/arc-media-server/pull/238) | `b738ff3ea945` |
| Titles | [#240](https://github.com/JonzaUK/arc-media-server/pull/240) | `ae5259e57100` |
| Artwork and matching | [#241](https://github.com/JonzaUK/arc-media-server/pull/241) | `34bdc1057fe3` |
| Details and orders | [#242](https://github.com/JonzaUK/arc-media-server/pull/242) | `c03d1f84b5f3` |
| Title actions | [#243](https://github.com/JonzaUK/arc-media-server/pull/243) | `4342d61969d3` |
| Player | [#244](https://github.com/JonzaUK/arc-media-server/pull/244) | `21cb30fb5454` |
| Profile Settings | [#245](https://github.com/JonzaUK/arc-media-server/pull/245) | `4ee1f2603f76` |
| Server Settings | [#246](https://github.com/JonzaUK/arc-media-server/pull/246) | `543a626de221` |
| Access Settings | [#247](https://github.com/JonzaUK/arc-media-server/pull/247) | `ca33d8256469` |
| Library editor | [#248](https://github.com/JonzaUK/arc-media-server/pull/248) | `0481fb2865dd` |
| Library management | [#249](https://github.com/JonzaUK/arc-media-server/pull/249) | `b2750b252d70` |
| Reports | [#250](https://github.com/JonzaUK/arc-media-server/pull/250) | `3001bbdf2d89` |
| Profiles and backups | [#251](https://github.com/JonzaUK/arc-media-server/pull/251) | `10e2257047f2` |

## Final integration changes

- Resolve shared Settings imports and collection-card/dialog changes while retaining all areas.
  Settings grouping and anchors use stable internal section keys, independent of translated titles.
- Complete queue notices/errors/aria, subtitle-search and season-artwork empty sentences; use full
  detection/file-order help variants. Remove three abbreviated catalog keys without changing values.
- Restore the literal `Home` keyboard token in RowShell. Phase 1 had compared a keyboard event
  with the translated navigation label; add an audit regression guard against that pattern.
- Use active-locale numbers, calendar names and fixed decimals. Existing English rounding and
  native browser date/time presentation are preserved in the views that already used them.
- Remove every legacy `fmt.plural` caller, the helper/catalog bridge, raw compatibility artwork/
  collection labels and unused source-order hints. Replace legacy helper assertions with relevant
  locale/date/rounding checks; retain unchanged existing browser selectors.
- Update TRANSLATION coverage, boundaries and first-language acceptance. Add the agreed new-UI-text
  rule only to CLAUDE's Principles. Its entire “Where things stand” section remains byte-identical
  to main. No Go source, API schema, dependencies, owner data or layout/CSS changes.

## Actual combined validation

Prepared Go1.27.0, Node24.19.0, npm11.9.0, FFprobe7.1.5 and pinned Chromium rev1248 were used.
No tools/dependencies/browser substitutes were installed. Actual results, not historical setup
passes, establish the following:

| Check | Passed | Failed | Skipped | Result |
| --- | ---: | ---: | ---: | --- |
| Production/e2e TypeScript, production and embedded-server builds | all | 0 | 0 | pass |
| i18n/catalog/fallback/locale checks | 19 | 0 | 0 | pass |
| Adaptive playback unit checks | 6 | 0 | 0 | pass |
| All-package Go vet | all | 0 | 0 | pass |
| Regular Go tests/subtests | 449 | 0 | 0 | pass; 275 top-level tests |
| Unique browser journeys | 118 | 1 | 0 | baseline fixture failure below |
| Optional all-package Go race tests/subtests | 448 | 1 | 0 | baseline test race below |
| Final Settings journey after key clarification | 1 | 0 | 0 | pass |
| Final desktop/phone visual journeys | 2 | 0 | 0 | comparison below |

The browser check was deliberately paused during CPU-heavy Go race checks: two completed adaptive
journeys were retained; the interrupted third and 116 unstarted journeys were resumed as 117 tests.
The resumed run passed 116 and failed one. The rollup asserts 119 unique journeys with no remaining
skips/interruption; neither passed adaptive journey was repeated. Raw initial exit130 and resumed
exit1 are preserved. No retries or selector changes concealed failures.

Three later catalog-name clarifications (`notifications`, `openSubtitles`, `administrator`) retain
identical English values. After them, production/e2e TypeScript, catalog checks and both builds
were rerun; the final Settings journey and two visual journeys passed. No other product-source
change followed the combined browser run. The area checks are recorded in their dated notes;
counts overlap across areas and are not additional unique journeys.

Final combined Settings comparison: six desktop/phone PNG pairs against separately built clean
main. Four byte-identical; the two maintenance-clock captures differ at 62 text-rendering pixels
each, with equal dimensions and no wording/layout change found. The earlier server area note
called these icon pixels; the comparison is in the clock text region. Other area differences and
fixture path/video presentation normalization are documented in their notes. No blanket pixel-
identical claim. Twenty Year intro template/DOM/spacing/plural combinations matched original markup.

## Failures reproduced on clean main

1. `webserver.spec.ts:284`, Chromecast text subtitles/AirPlay: UI assertions pass, then Node fetches
   the cast WebVTT URL at the container LAN address while the fixture server listens on loopback,
   producing ECONNREFUSED. The final failure is at 172.26.91.246:18199; a focused clean-main run
   reproduces it. Real casting is unverified. Do not substitute a URL merely to mark this passed.
2. `TestConversionWaitsForThePlayer`: the race detector sees writes to test timing globals in
   `internal/transcode/throttle_test.go:29` against a previous conversion's throttle read at
   `throttle.go:53`. The unchanged clean-main transcode package reproduces the same race (10 passed,
   one failed, exit1). Only the tests assign those timing globals after declaration. No Go files
   differ from main; this task neither repairs that separate test race nor claims race acceptance.

## Evidence and limits

Evidence root: `/workspace/arc-cloud-evidence/i18n-remaining/`:
- `integration-browser-rollup.json`, raw `integration-browser*` logs/JSON/exits/traces.
- `integration-go.jsonl`, `integration-race.jsonl`, `integration-race-baseline*`,
  `integration-cast-baseline*`, `integration-vet.log`, build/adaptive/i18n logs.
- `integration-settings-final*`, `integration-visual-final*`, `integration-screenshots-final.json`,
  `integration-key-clarity.json`, `integration-tools.json`, `integration-scope-check.json`.
- Per-area notes/evidence, and `reports-intro-dom-check.mjs/log`.

Temporary stores, generated media, local provider fixtures and fixture-only keys throughout.
No owner media/settings, production keys, Unraid, persistent server, container publication or
production/device acceptance. The API `/docs` app is outside scope. The first real translation
still needs plural/language review, document `lang`, dynamic language/fallback behavior, wrapping,
accessibility and device checks. Current English validation does not establish those outcomes.
No GitHub Actions completion was awaited; this PR lists the actual local results and limitations.
