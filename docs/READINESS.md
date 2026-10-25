# Stability and release readiness




## Library work, safe Stop, clock and title-action reviews (#183/#184/#196/#187/#188, 25 October 2026)

Clean starting commit f8261f7 matched GitHub main read-only; work branch
codex/library-maintenance-controls. Saved activation selected Go 1.27.0, with prepared
Node/npm, compiler, FFmpeg, pinned Chromium/dependencies/caches present. No installation,
owner settings/media/keys, Unraid or persistent server. Earlier hosted results remain history.

Review decisions: implement #183/#184/#196/#187; defer #188 in its existing Later milestone.
#183's current item/show/bulk refresh and due-only Run now do not provide an explicit
library-wide force refresh or basic-file/optional-format repair. New confirmed library actions
reuse provider/field-lock rules and drain ID cursors. Reanalysis preserves identity, links,
watch state, edited markers, presence/last-seen and existing probed-at timestamps; it does not
reclassify existing files as newly added work. Before/after physical stats and a transactional
saved-revision guard reject changed/offline/removed sources without losing previous details.
#184 adds owned cancellable runs to the existing library lease, dropping scanner follow-ups
and waking cancelled retry timers. Stops are distinct from errors, cannot target later runs
or survive a restart as a valid control for a new run, and keep unvisited files present.
Library/Activity controls expose stopping/stopped and Retry; bulk metadata refresh is also
cancellable. #196 reports actual server clock/zone/offset and open/next maintenance policy,
including overnight/all-day and DST skips/repeats; no scheduling-hour bug or policy change is
claimed. #187 moves Refresh into More and distinguishes Playback queue from saved playlists
with labels/icons, preserving the approved hero/poster/background/Cast layout classes.
Contract: [LIBRARY-WORK-CONTROLS.md](LIBRARY-WORK-CONTROLS.md); additive API docs regenerated.

#188 current code still provides search, shared filters, reverse/saved sorting, 500-item paging
and Load more. Fresh existing browser checks cover reaching title 501 and saved sort direction;
earlier audit Back-scroll evidence remains historical. No owner-library navigation session was
available to establish whether alphabet jump, density or a list view actually helps. Adding a
control solely from synthetic card counts would not meet the issue's adoption criterion or
the project's simplicity principle. No navigation feature was added and no performance/parity
claim is made. Keep #188 open/deferred; revisit with a specific observed owner browsing task.

Actual local results:

- Complete ordinary Go run: 364 tests/subtests, 219 top-level; zero failures/skips. Vet passes.
  Affected activity/scanner/store/background/schedule/API race run: 250 tests/subtests,
  136 top-level; zero failures/skips. Seven new regression functions include 13 cases/subtests.
- Tests hold a local provider until cancellation during a bounded incomplete discovery,
  prove unvisited files remain present and queued work/retry timers are dropped, reject stale
  Stop IDs and cancel an actual held child executable used as FFprobe. Another force refresh
  visits 205 freshly synced provider records across the 200-row cursor while keeping locks.
  Analysis storage checks preserve presence/identity and original probe timestamps and reject
  late changed/missing revisions. Clock cases exercise real Europe/London DST transitions.
- Seven new Chromium journeys pass together (47.4 seconds), zero failures/skips/retries.
  Real generated MP4 reanalysis and provider refresh preserve edited titles, file IDs and
  resume; controls exercise conflicts, stopped/idempotent/stale requests, Activity Stop,
  errors/Retry, server-derived clocks, title refresh actions and ordinary-profile denial.
- Ten related existing journeys pass together (76.5 seconds): library scan guards/partial
  retry, Settings grouping/navigation, queue/progress, per-profile saved sort, source-format
  labels, reaching title 501, profile/library access, playlist recovery and preplay/optimized
  audio. Total distinct focused browser journeys: 17, not a sum of reruns.
- Final follow-ups pass: five phone/1440px/clock/access journeys, one library-row failure
  journey using deliberately disabled fixture providers, and one real disposable-server
  restart/stale-Stop journey. The last run preserves saved state across restart and proves
  a pre-restart ID cannot affect a new run. A final 13-case targeted race run covers the
  timestamp-preservation refinement, and the embedded-server build passes. Clock/API-contract
  checks, production/app/e2e TypeScript and production build pass; diff checks are clean.
- Desktop/phone screenshots were inspected. Stop/Retry stays reachable at 390px; repair
  confirmation spells out cost and preserved data; clock text wraps inside the Settings card;
  movie/series Refresh remains available in More without changing the approved layout geometry.

Raw development failures remain retained, not counted as successful acceptance: the first
browser pass had 3 passes/4 fixture failures (alertdialog role, rejected delay outside the
fixture limit, series refresh route). The deterministic provider hold initially lacked its
await in the actual provider-response branch, producing 5 passes/2 failures; corrected, all
seven pass. A wrong working-directory harness preparation and an obsolete interrupted run
(exit 130) are also retained. Production failures were not concealed by retries.

First hosted run 40176974946 at 8594200 passed Windows, Linux Go/vet/race, container
acceptance and all seven new journeys, but finished with 76/77 browser passes. Publication
was correctly skipped. The existing ratings/watchlist/refresh recovery journey still searched
for the former Refresh button. Its follow-up opens More and selects Refresh metadata; all
failure feedback, Retry and persisted-watchlist assertions remain. No runtime, gate or retry
policy change. The corrected journey passes locally; raw hosted failure is retained as
hosted-first-linux.log. Final hosted verification/publication is recorded on the individual
issues/#162 and alongside these artifacts after the follow-up run completes. Prior runs are not substituted. Evidence:
`/workspace/arc-cloud-evidence/review-183-184-196-187-188/` (START.json, go-final.jsonl,
race-final.jsonl, analysis-preservation-final.jsonl, passed.json, existing.json, mobile.json,
failure.json, restart.json and failure traces/screenshots). Physical NAS/device/provider quotas,
owner navigation benefit, long jobs under real load, power-loss/full-disk and sustained use
remain separate. No native-client, HDR/Atmos-output or owner-deployment acceptance is claimed.

## Preplay source/track choices (#186, 25 October 2026)

Clean starting commit becadf5 matched GitHub main read-only; work branch
codex/preplay-track-options. Saved activation selected Go 1.27.0 and the prepared Node/npm,
FFmpeg, dependencies/caches and pinned Chromium 156 revision 1248. No installation or owner
media/settings/keys/Unraid access. Only generated clips, disposable stores and local providers.

The finding remains current: tracks were selectable only after Play. Movies, series-next and
available episode rows now offer Playback options with named version/audio/subtitle controls
and Play/Resume. A new profile-authenticated options endpoint shares actual playback file,
probe and preferred/default-audio resolution. It creates no stream/conversion or playback
progress/preference write, though unprobed files can be inspected on demand. Access checks
and source ownership apply. Version changes discard file-specific choices; labels follow the
selected edition. Ordinary Play/Resume and optimized eligibility are unchanged. Source choices
bind to the initial queue item/file and survive initial admission retry; picture subtitles enter
the first request as burn-in. Off suppresses automatic selection, and a vanished text track
reports profile-default fallback. Manual subtitle/version recovery clears that notice.

Approved language learning applies after successful admission only, combining preplay audio/
subtitle preference changes in one PATCH. Full subtitles remember language, forced ones do
not overwrite it, and Off follows the existing subtitle-mode rule. Automatic defaults and
cancellation do not learn choices. No migration, dependencies, quality policy or casting changes.
Contract and Plex references: [PREPLAY-OPTIONS.md](PREPLAY-OPTIONS.md). The API reference was
regenerated; its contract checks pass.

Fresh validation:

- Affected API/API-documentation ordinary checks: 89 tests/subtests (30 top-level), zero
  failures/skips. Focused playback/options/optimized race checks: 50 tests/subtests (7
  top-level), zero failures/skips. API tests verify French/two-letter/default/fallback audio,
  source commentary choices, no playback URL/session or preference updates, malformed input,
  unrelated file IDs and restricted-library denial. Existing best-version/optimized tests
  retain their assertions. Vet and the embedded-server build pass.
- Four new browser journeys pass, then a combined 10-journey run passes without failures,
  skips or retries (103.3 seconds). Six existing journeys cover delayed-subtitle French/Off
  races, optimized audio/output and measured bitrate budgets, editable queues/progress and
  compact autoplay. These are ten distinct journeys, not the sum of repeated runs.
- Actual generated two-language audio and external French subtitles play. An injected first
  start failure performs no preference write; Retry preserves the exact file/audio in both
  requests. Successful French uses the original/remux path rather than an English optimized
  copy. Switching to English can use the matching optimized copy; Off remains Off and the
  existing language preference policy is checked through the real API.
- Generated theatrical/director files reverse their audio ordering. The chosen edition/file
  resolves correctly; version switches reset IDs and defaults, errors offer Retry, cancellation
  makes no playback/preference writes, and the actual selected file/audio reaches playback.
  Episode header/keyboard row controls target First Chapter and display actual subtitle cues.
- Picture tracks and removal-at-start are injected response fixtures: the first request has
  burn_subtitle, a converter-unavailable error is explicit, and a missing text track explains
  fallback. These prove request/recovery wiring, not physical PGS/DVD burn-in output. A focused
  final follow-up checks that choosing actual English subtitles clears the fallback notice.
- Production/app and e2e TypeScript, production UI build and diff checks pass. Desktop/390px
  screenshots were inspected. Screenshot review caught and corrected a stale original-edition
  description after changing versions; the selected-edition assertion now guards it.

Development failures are retained: initial Go compilation found an extracted local error
variable that needed declaring; the first browser pass had two fixture assertion failures
(language normalizes fre to fra; a status locator matched both page and player). The next four
journeys and combined ten pass; no failures are relabelled as successful runs. A final isolated
notice-clear check is retained separately. Full hosted validation/publication is tracked on
#186/#162 and saved alongside local results, rather than assumed from historical runs.

Evidence: `/workspace/arc-cloud-evidence/preplay-186/` (START.json, backend-final.jsonl,
race-final.jsonl, first.json/final.json/verified.json, fallback.json, logs, traces and screenshots).
Physical devices, actual HDR/Atmos output, cast receiver choices (#192), screen-reader acceptance
and owner-server/sustained-use checks remain unverified here. No persistent server is started.

## Library, backup and bulk-selection clarity (#181/#182/#185, 25 October 2026)

Starting checkout c8dc62c was clean and matched GitHub main read-only. Work branch:
codex/library-backup-selection-clarity. Prepared Go 1.27.0, Node 24.19.0/npm 11.9.0,
FFprobe 7.1.5, dependencies/caches and pinned Chromium 156 revision 1248 were present;
no tools or dependencies were installed. Only disposable stores, generated media and local
provider fixtures were used. No owner data/settings/media, production keys or Unraid access.

All three findings were confirmed in current code. #181 associates the name input and each
movie/show advanced select with a visible label using unique React IDs; descriptive text is
linked with aria-describedby, including the changing file-numbering explanation. Group labels
(Type/Folders) remain groups rather than labelling an arbitrary nested button. Layout and
save/validation behavior remain unchanged. #182 reads the existing settings query: On/Off,
server-local maintenance window (including overnight/equal-hours all-day), foreground-work
admission delay, manual/safety availability and last saved successful backup. Loading/error
never implies Off; policy errors offer local Retry without disabling manual backup. The link
opens existing Scheduled tasks. There is no startup-time guarantee or scheduling change.
The backup manager's legacy overdue-window exception is still gated by background.Ready
in cmd/ams; the UI describes the actual application gate, not the manager in isolation.
Last successful backup means the newest saved snapshot, not proof of a successful latest
attempt; no new persistent failure-history API or real recovery acceptance is claimed.
#185 says Select loaded (N), preserving the existing explicit-ID bulk mutation contract.
New pages are never selected implicitly; Select None and route/filter reset remain intact.
No all-matching selection, API, database, dependencies or scheduler behavior changes.

Four new Chromium journeys failed against the previous embedded build: both movie/show
name controls were unnamed, backup policy was absent and the bulk control implied all.
After rebuilding, all four pass with no failures/skips/retries (23.6 seconds):

- Movie/show Add and Edit expose named controls and real associated help; clicking Name
  focuses the input, keyboard language selection saves through the real API, reopening
  retains it, and changing show numbering updates its help. Phone screenshots inspected.
- Actual automatic-backup Off, real manual snapshot creation, actual Settings toggle to On,
  same-SPA query-cache update, overnight and all-day windows, unavailable-policy Retry and
  continued manual-action availability. This does not simulate a full day of scheduling.
- A synthetic filtered 501-title response loads 500. The explicitly captured mock bulk write
  contains exactly those 500 IDs; Load more retains the prior 500 selection until explicitly
  selecting the loaded 501. Clearing/Done and the 390px control layout are checked. Synthetic
  records never reach real mutations; this is scope evidence, not a scanner benchmark.

Production/app and e2e TypeScript, production UI build, embedded-server build and
`git diff --check` pass. Seven related existing journeys also pass without failures/skips/retries: invalid-folder
retry, reaching title 501, selection reset across libraries, real restore/settings reload,
removal safety-backup labelling, desktop/phone management navigation and labelled Settings
controls. Total distinct focused browser checks: 11. Hosted validation/publication is tracked
on the individual issues and #162; it remains separate until the new run completes.
Full local Go/race suites were not repeated for these UI-only corrections; hosted gates
retain their existing backend/race coverage. Previous hosted runs remain historical evidence.
Artifacts: `/workspace/arc-cloud-evidence/review-181-182-185/` (START.json, before.json,
after.json, retained failure traces and inspected screenshots). No persistent fixture server
is left running. Physical screen readers/devices, owner recovery and sustained-use acceptance
remain separate from Chromium accessibility-name/keyboard and temporary-store checks.

## Recoverable UI errors and episode keyboard playback (#176–#180, 24 October 2026)

The clean starting checkout 14094bb matched GitHub main read-only. Work branch:
codex/recoverable-ui-errors. Fresh pre-fix browser fixtures reproduced all five findings:
**seven journeys failed at the expected checks** (News, playlist picker, Space activation,
Devices/Optimized Versions/Notifications, and device sign-out feedback). Raw before.json and
its screenshots/traces are retained, not counted as passing acceptance.

- **#176:** New for You exposes local query errors/Retry, an accessible loading status and a
  distinct empty state. Cached entries remain visible with a stale notice. Closing after a
  failed refresh no longer marks unseen updates seen; fresh recovery retains the existing
  seen behavior. Discovery/news eligibility rules are unchanged.
- **#177:** playlist list-loading errors remain separate from add/create errors. Existing cached
  editable lists remain available with an honest stale notice; creation is disabled with a
  reason until the list is usable. Retry preserves the selected title and unsaved name.
  Competing add/create requests are disabled while a mutation is pending. Smart playlists
  remain excluded from manual additions.
- **#178:** a native button overlays only the episode artwork/details, with an explicit Play
  name and focus ring. Source/queue/playlist/watched/edit controls are siblings, not nested
  interactive descendants. Native Enter/Space and mouse behavior each start playback once;
  Space does not scroll, and playlist/watched Space actions do not start playback. The
  approved series/title page structure is retained and desktop/390px rows were inspected.
- **#179:** Devices, Optimized Versions and Notifications distinguish loading/empty/unavailable/
  loaded states, retain cached records visibly on refresh errors and provide local Retry.
  Add Webhook is disabled with an explanation until its event data can support the form;
  cached usable data still supports forms. Shared ErrorNotice can disable its retry button
  and say Retrying while a query request is pending.
- **#180:** each device row owns pending/error state, with visible failure and Retry. Rows are
  removed only after the server confirms revocation, updating the cache before refreshing
  so a failed subsequent list request does not resurrect a confirmed signed-out device.
  Errors for two different rows remain independent through retries/list refresh.

Local final verification passed: production/app TypeScript/Vite build, e2e TypeScript, final
embedded-server build and diff whitespace checks. Existing backend session/cast regressions
passed **eight tests/subtests (four top-level functions)**, including cookie transport, cast
revocation after logout/password/profile/home-policy changes and session bookkeeping. No Go
source/API/schema/dependency changes were needed, so the full unrelated Go/race suite was not
repeated locally; hosted verification/publication is separate evidence on the issues.

The **eight new Chromium journeys passed**, zero failures/skips/retries, 37 seconds. Checks
cover initial 503 then Retry, true empty lists, cached lists after failed refresh, playlist
name/title preservation and actual addition after recovery, native episode keyboard/mouse
activation and independent row actions. Settings stale-state fixtures use correctly typed
synthetic list responses and controlled client time. The device test uses two real temporary
app sessions from the actual API (Unix-second timestamps), injects DELETE failures, then proves
real successful revocation returns 401 for each token while the current admin stays signed in.
It also verifies that confirmed removal survives an injected subsequent list-refresh failure.
No webhook was sent to an external service.

Six existing related Chromium journeys passed (zero failures/skips/retries, 36 seconds): manual
editable-playlist selection, alternate episode order/unplaced files, Settings grouping/navigation,
local server-setting save/error/retry, mobile Settings/player controls and playlist keyboard
ordering/write recovery. The existing profile/library/admin boundary journey also passed (18 seconds, access.json),
for **15 distinct local browser journeys passed** overall. Desktop/phone episode focus, news/playlist and Settings/device error screenshots
were visually inspected; this is not a full screen-reader or physical-device audit.

An initial post-change run passed six new journeys but rejected two fixture assumptions: initial
library imports are intentionally excluded from News, and a closing player's exit animation can
briefly coexist with its replacement. The corrected cached-news test injects a typed news entry
for a real generated item; the keyboard test waits for actual player removal before reopening.
No discovery/player animation policy was changed to make those tests pass. A root-directory
compiler lookup was canceled without installing anything; root node_modules and a cached tsc
shim were absent, and the prepared local compiler completed the final check. Earlier attempts
remain retained. All stores/media/provider fixtures were temporary; no owner data/settings/media,
production keys, Unraid, new tools/dependencies or persistent server. Evidence:
`/workspace/arc-cloud-evidence/review-176-180/`.

Hosted follow-up: run 40137947443 at b0fc9dc passed Windows, container checks, Linux Go/race
and **60/62 browser journeys**, including all eight new recovery checks. Publication was skipped
because two existing queue/compact-player tests used the broad button selector /^Play S/; the
new valid Play Second Chapter button made it ambiguous with Play S1 E1. The correction targets
the main season/episode-code Play button explicitly in those two tests, retaining every queue,
playback/progress and countdown assertion. Production labels/behavior and CI gates are unchanged.
Both corrected local journeys passed (28 seconds, zero failures/skips/retries), as did e2e
TypeScript; this brings the distinct local browser coverage to 17 journeys. The first hosted
log is retained as hosted-first-linux.log; final hosted results are tracked in the same
evidence folder and on #176–#180.

## Audit corrections: bitrate, Search and phone filter recovery (#172/#174/#175, 24 October 2026)

Starting clean baseline d232230 matched GitHub main read-only. Work branch:
codex/bitrate-search-filter-recovery. The next three unresolved items in #162 were confirmed
against current code before implementing. Only isolated stores, generated clips and local
providers were used; no owner data/settings/media, production keys, Unraid or persistent server.

**#172:** the pre-fix API regression accepted an optimized copy under a 2 Mbps request despite
unknown output rate. Migration 0051 adds measured output duration/average total bitrate; the
configured FFprobe inspects the completed output, and attempt-owned completion persists the
larger of its format rate and actual bytes divided by measured duration. Source/preset rates are
not substituted. Selection enforces the effective manual, remote and per-stream upload-share
budget and independent client/quality height limits. Exact boundaries fit; even one bit/s over
budget does not. Unknown/nonpositive duration/rate falls back under limits. Existing copies are
preserved for unlimited playback without a backfill/rebuild. Failed output measurement has the
same conservative behavior. This is average-rate eligibility, not a VBR peak, traffic shaping
or real-network throughput guarantee. Existing live-conversion policy is unchanged. Contract:
[OPTIMIZED-COPIES.md](OPTIMIZED-COPIES.md).

Fifteen API budget cases cover manual/remote/upload-share tightening, unknown and exact/over
boundaries, height and home behavior. Real generated outputs are probed; schema-50 migration,
valid pre-upgrade backup, restart persistence and requeue measurement reset are covered. Three
worker measurement-failure cases preserve unlimited usability and reject limited selection.
A new full-app journey makes a noisy generated 720p-preset copy whose measured rate exceeds
2 Mbps, verifies unlimited selection and actual reported duration/rate, then requests 2 Mbps
and gets an actual HLS conversion rather than that copy. It does not emulate remote congestion.

**#174:** the held-response baseline reproduced the new query heading with an old actionable
movie. Search now keeps cached results within their query key, without previous-query placeholder
data. An accessible Searching status sits outside the busy results region so live announcements
are not deferred by aria-busy. Delayed/zero-result responses, an older response arriving after
returning to a newer query, error/retry, clearing and back/forward navigation are exercised.

**#175:** the phone baseline had no empty-state Clear filters action. Filter controls now wrap
instead of hiding recovery in a horizontal strip. Active values stay reachable even when absent
from current facets. The empty state and filter controls both offer Clear filters; removing
filters preserves sort, direction and unrelated URL state. At 390px the journey checks horizontal
bounds/no page overflow, removes an individual active genre, clears using keyboard activation
and restores content with sorting intact. The final check also explicitly scrolls the empty
state into view and confirms its button is within the viewport; screenshots were inspected.
Normal vertical scrolling remains supported.

Verification: all Go packages passed **343 tests/subtests (211 top-level functions), 28 test-bearing
packages and five no-test packages**, zero failed/skipped test cases. Full store/optimizer/API
race checks passed **144 tests/subtests (78 top-level functions)**; a final focused race run passed
**50 tests/subtests (13 top-level functions)** including the final measurement-failure/boundary
changes. Go vet, final embedded-server build, production/app TypeScript/Vite build, e2e TypeScript
and diff whitespace checks passed. Eight focused Chromium journeys passed (zero failed/skipped/
retries, 127 seconds), covering the three new regressions, search error/history, optimized audio,
and actual remux/transcode seek/resume. A final targeted replay passed all three new journeys (zero failures/skips/retries, 28 seconds),
checking accessible status placement, the empty-state viewport and retained actual output data.
The generated copy measured 4,653,398 bits/s over 6,041 ms; a 2 Mbps request rejected it and
started actual HLS conversion. Screenshots of the final pending/phone states were inspected. Hosted full-suite/publication
results are separate and recorded on #172/#174/#175.

Earlier results remain retained: pre-fix failing Go/browser regressions; a JSX typo initially
blocked the UI build, and that first browser pass used the prior embedded UI (six journeys passed,
two UI regressions failed). That attempt is not accepted as validation of the UI changes. The typo
was corrected, the production UI rebuilt successfully and all eight reran successfully. No test
was weakened to accept stale results or a hidden button. Final evidence and limitations:
`/workspace/arc-cloud-evidence/review-172-174-175/`.

## Optimized audio selection correctness (#171, 24 October 2026)

Starting clean baseline: 226d05d (GitHub main matched read-only); working branch
codex/optimized-audio. The new preferred-French API regression failed before the change:
playback reported source audio stream 2 while returning the English optimized MP4. The original
failing response is retained at `/workspace/arc-cloud-evidence/optimized-171/before.log`.

Migration 0050 persists the exact mapped source audio index, language and title on attempt-owned
completion. The optimizer continues encoding default/first-source audio as AAC stereo, records
silent copies explicitly and writes language/title/default metadata. Playback eligibility now
compares the effective profile-derived or explicit source choice, including same-language
commentary. Mismatches use the existing original/remux/transcode path; matching explicit choices
can reuse an artifact. Explicit no-audio choices cannot return audible original/cached media.
Source choices and codec labels remain unchanged; the optional playback `optimized` object and
player info identify the served quality, source audio mapping and actual AAC stereo output.

Older completed copies, including schema-49 artifacts, have unknown audio provenance and require
explicit Optimize again; files remain recorded/removable. There is no automatic bulk rebuild.
An actual schema-49 upgrade creates a valid pre-upgrade backup; regeneration and reopen retain
the new track metadata. Source identity, watch state and source bytes are not rewritten. Contract
and response semantics: [OPTIMIZED-COPIES.md](OPTIMIZED-COPIES.md). Bitrate eligibility stays #172.

Local verification passed: all Go packages, **321 tests/subtests (208 top-level functions),
28 test-bearing packages and five packages with no tests**, zero failed/skipped test cases.
Store/optimizer/API race checks passed **125 tests/subtests (76 top-level functions)**; a final
API race rerun covers the added explicit-mute decision case. Go vet, embedded-server build,
production/app TypeScript/Vite build, e2e TypeScript and git diff whitespace checks passed.

New regression coverage includes ten API preference/explicit-selection cases (language aliases,
missing preferred language, overriding preferences, alternate language, same-language commentary,
no audio and invalid index), original MP4 audio-disable decisions, schema-49 audio migration/
backup/restart, and real FFmpeg output for default English, default French and no-default first
track. FFprobe verifies the actual sole output track (AAC, stereo, MP4 index 1, language/default).
Decoded output samples measure approximately 439.9/879.9 Hz for distinct 440/880 Hz input tones;
this establishes actual mapping rather than trusting language tags. It is not a spoken-language
or physical-speaker listening test.

Three pinned Chromium journeys passed (zero failures/skips/retries, 84 seconds): existing real
remux and transcode play/seek/resume, plus new real multi-audio optimized playback. The new
journey sets the profile to French, confirms remux of source track 2, switches to English and
checks optimized track/output details and advancing playback, inspects the actual info-menu
screenshot, then switches back to French and confirms playback advances. It uses generated
multi-audio MKV and local metadata. The complete unrelated browser suite was not repeated locally;
hosted Linux/Windows/full-browser/container/publication results are recorded separately on #171.

Initial sandboxed broad Go/browser attempts were blocked by local socket and compiler-subprocess
permissions, not accepted as passing runs. They remain retained (go-all.jsonl, browser.json,
fixture-start.log); rerunning with the required execution permissions passed without tool or
dependency changes. All media, stores and provider fixtures were temporary; no owner media,
settings, production keys, Unraid or persistent server was used. Prepared Go 1.27.0, Node 24.19.0,
npm 11.9.0, FFmpeg/FFprobe 7.1.5, GCC 14.2 and pinned Chromium revision 1248 were present.
Evidence, versions and initial/final results: `/workspace/arc-cloud-evidence/optimized-171/`.

## Optimized-copy source revision correctness (#170)

Contract and compatibility behavior: [OPTIMIZED-COPIES.md](OPTIMIZED-COPIES.md). A fresh
regression at 6886f1f reproduced the audit finding: after SaveFile updated a source, the old
optimized copy still appeared playable. The initial failing log is preserved, not counted
as a pass.

Migration 0049 records source path/size/time, actual conversion-time filesystem precision and
attempt identity. SaveFile atomically invalidates changed-source copies without changing item
or file identity. Claim/completion/failure guards and unique filenames isolate superseded
workers. Playback and the final streaming endpoint validate saved/actual source revisions and
artifact existence/size; new URLs also bind the attempt. Missing artifacts can be explicitly
requeued. Restart resumes interrupted jobs under fresh attempt IDs and prunes only orphaned
Arc-owned output. Registered failed copies and unrelated files remain removable/preserved.

Legacy completed copies are deliberately not stamped with an inferred source: they may already
be stale. Their generated files remain recorded but require Optimize again; there is no bulk
background rebuild. Actual schema-48 upgrade/pre-upgrade backup, restart persistence, metadata
edits/locks and resume-point retention are tested. Original media is only read by Arc.

Local final verification passed: all Go packages (304 tests/subtests, 205 top-level functions,
28 test-bearing packages and five with no tests), Go vet, production/app and e2e TypeScript,
and embedded-server build. Four affected packages passed race checks (store/optimize/API/scanner,
190 tests/subtests, 113 top-level); after preserving the existing startup-error fallback,
final store/optimize/API race checks also passed (109 tests/subtests, 73 top-level). No test
failures or skipped test cases in these final runs; no-test packages are counted separately.

Nine new Go functions produce 17 focused test/subtest passes: source size/time changes versus
unchanged saves; queued-source invalidation; late completion/failure after replacement,
deletion/requeue and restart; legacy upgrade/reopen; actual generated-media regeneration;
controlled subprocess replacement/deletion; interrupted subprocess recovery/owned cleanup;
and API selection/range/URL/missing-artifact repair. The real worker test verifies a six-second
copy is replaced by a nine-second copy and optimization leaves its input bytes unchanged.

Seven full-app HTTP observations pass with generated media/local providers: initial optimized
playback, rejection/fallback before a scan, post-scan identity/title preservation and updated
duration, explicit nine-second regeneration with new generation/bytes, missing-artifact
fallback, explicit repair and valid-copy reuse across restart. The first HTTP script mixed
Fetch's ok property with Playwright's ok method at its restart assertion; corrected in the
external harness. A second replay reached all seven checks but reported a nonzero runner
status, so was not accepted as a passing run. The final instrumented replay records child
exit 0, runner exit 0 and all seven checks; earlier attempts remain retained. Neither harness
result was treated as a new product defect.

Four focused pinned-Chromium journeys passed (zero failures/skips/retries, 50 seconds): real
remux/transcode playback with seeking/resume, direct video progress/reload resume and backup
restore with live settings/access. The entire unrelated browser suite was not repeated locally;
hosted Linux/Windows/full-browser/container/publication results are separately recorded on #170.
Evidence: `/workspace/arc-cloud-evidence/optimized-170/`.

Only isolated stores, generated clips, stand-in subprocesses and local providers were used.
No owner data/settings/media, production keys, Unraid, tool/dependency installation or persistent
Arc server. Filesystem revision checks are not whole-file hashing or guarantees about already
open streams. Device/HDR/Atmos/NAS/sustained-use acceptance remains separate; #171/#172 audio
and bitrate findings are outside this source-revision fix.

## Scanner worker CI regression (#173, 23 October 2026)

Arc/Plex review #162 now tracks all 36 audit findings through separate review issues or
existing dedicated issues. First review: failed publication run 39974633288 at a5f0a711.
Linux TestEach returned the expected boom error but counted 166 callbacks beyond index 100;
its arbitrary maximum of 50 assumed callback 10 had already run. Concurrent dispatch does
not guarantee that callback execution order. This failure does not establish that the
scanner continued after receiving a published error.

A controlled external copy of the unchanged helper held callback 10 while other workers
continued: 899 callbacks beyond index 100 finished before the error was released, then the
helper correctly returned boom. This demonstrates the invalid assertion; it is not a replay
of the hosted scheduler trace. The original test also passed 1,000 single-CPU repetitions,
so isolated passes alone were not accepted as an explanation or a fix.

The replacement TestEach holds the first worker wave using channels, proves the configured
concurrency is reached without sleeps, checks the upper limit and verifies every index runs
exactly once. TestEachStopsOnError uses one worker to assert exactly 11 calls through index
10 and the original error. TestEachDelayedError holds that error while all other work finishes
and verifies the eventual error is retained after dispatch has drained. Ten-second contexts
are deadlock watchdogs, not callback-order assertions. No scanner runtime, workflow gate,
dependency, media or database changes.

Local verification passed: all Go packages (287 tests/subtests, 196 top-level functions,
27 test-bearing packages and six packages without tests), Go vet, the embedded-server build
and the full scanner race package (81 tests/subtests, 40 top-level functions). The three worker
tests passed 1,000 repetitions each both at the default CPU setting and GOMAXPROCS=1 (6,000
test executions total, no failures/skips). Two deliberately invalid external helper variants
are rejected: continuing dispatch after reporting boom fails with 999 calls instead of 11;
discarding an error after dispatch drains returns nil and fails the delayed-error regression.
An initial worker-continue-only control still passed because its dispatcher correctly stops;
that control does not violate the contract and is retained as such in the evidence.

Evidence: `/workspace/arc-cloud-evidence/ci-173/`, including the original repeated run,
controlled baseline, final ordinary/race/repetition logs and external mutation checks.
The web app and browser suite were not rerun locally for this test-only correction. Hosted
Linux/Windows/browser/container verification and image publication are separately recorded
on #173; local success is not a claim those hosted gates have passed. No owner data/settings/
media, real providers, production keys, Unraid, tool installation or persistent server used.

## Routine rescan/maintenance separation (#169, 23 October 2026)

Owner-approved plan: [INCREMENTAL-RESCAN-PLAN.md](INCREMENTAL-RESCAN-PLAN.md).
Ordinary rescans no longer refresh old matched movies/shows or retry old unmatched titles
solely because time elapsed. Metadata maintenance owns those tasks, including incomplete movie
metadata and old unmatched movie/series recovery. It respects existing enabled/admission and
foreground cancellation; admitted Run now handles stale movies outside the clock window. Movie
refresh drains 200-row cursor pages; failures do not block later pages or get retried in the
same pass. Maintenance lookup errors no longer schedule discovery scans. Its Activity summary
retains deferred updates instead of implying a filesystem scan occurred.

New/pending content, unfinished initial series-import recovery and explicit language/provider/
numbering actions remain supported. A genuinely new unlinked episode can request fresh provider
data when cached numbering cannot place it. That lookup bypasses the preview response cache;
the first regression caught the stale-cache case. Existing linked unknown episodes retry in
maintenance after an hour, with file-only episode identity/history retained when matched later.
Explicit library-language changes mark unlocked known titles pending, preserving immediate
requested matching rather than waiting for their age to become due.

Identical file-link writes already no-opped before this change. Episode linking now compares
fresh, bounded batched snapshots and avoids repeated per-file link reads when mappings agree;
it still validates filename/order mappings and local assets. There is no persistent identity
cache or migration. Activity says Checking episode links for that work, and Linking episodes
when a mapping changes. Unchanged local artwork/theme/season-art calls return through the read
pool instead of joining the writer queue; changed/removed local assets and chosen art remain.

Verification passed: six affected Go packages (scanner/store/background/metadata/API/apidoc),
full scanner/store/background/metadata race packages, final added-maintenance focused race,
Go vet, embedded-server build, production/app/e2e TypeScript and diff checks. Seven new Go test
functions cover stale unchanged movie/show scans with zero age-driven provider calls and zero
probes, maintenance disabled/admitted outside-window behavior, explicit language refresh, new
unknown episode refresh without repeated rescan requests, later matching with preserved watch
history, old unmatched movie/show maintenance-only retry, >200 failed candidates drained once,
no discovery retry after maintenance failure, writer-held unchanged artwork operations, and
link snapshots across 256-row boundaries/duplicate IDs. Existing provider-switch, alternate-
order/import-retry, custom placement, local assets and missing/offline regressions also pass.

Eight focused Chromium journeys passed without retries (1.2 minutes): initial movie/TV import,
unchanged series scan, restored settings/access, individual/bulk scan controls, Settings/Activity
navigation, concurrent Activity detail and paused work. Actual evidence is in
`/workspace/arc-cloud-evidence/incremental-rescans/`. The initial new-episode test exposed preview
cache reuse and now passes; a later paging test fixture lacked required added_at/updated_at,
was corrected and rechecked. Initial failure logs are retained, not counted as passing.

Only temporary stores/generated clips/local providers were used. SQL aging and held-writer
tests target disposable fixture databases. No owner data/settings/media, production keys,
Unraid, installed tools or persistent server. Full rescans still walk the directory tree and
check local sidecars; no Plex-equivalent speed, real NAS throughput/device or sustained-use
claim is made. Full unrelated browser suite not repeated; hosted publication is separate.

## Manage libraries scan actions (#168, 23 October 2026)

Settings → Manage libraries now offers Scan all libraries beside Add Library and a Scan
action per library. Bulk requests use the existing admin endpoint for idle libraries only;
already-running scans continue. Request-pending and running buttons are disabled. Each request
settles independently, so one failure does not prevent other libraries starting; affected rows
show the error and Retry. Bulk feedback counts accepted/failed requests, not completed scans.
Activity remains the source of detailed progress/results. All-running and empty states are
handled; Edit/Remove/Add remain available and row actions wrap on phones. No backend or
matching/scheduling policy changes, migrations or new settings.

Production build/app TypeScript, e2e TypeScript and diff checks passed. Three focused Chromium
journeys passed without retries (27.2 seconds): individual request/pending guard, bulk idle-only
requests, 503 partial failure/retry without repeating accepted requests, all-running/empty state,
390px layout without overflow, existing Settings/Activity navigation and ordinary-profile
management protection. Real local scan endpoints were used; already-running status and one
503 were browser fixtures. Actual desktop/phone screenshots were inspected. Tests used only
temporary libraries, generated clips and local providers; no owner data/settings/media/Unraid.
Evidence: `/workspace/arc-cloud-evidence/manage-library-scans/`. No unrelated Go/race/full-browser
suite rerun for this UI-only change. Hosted image publication and owner deployment remain separate.

## Scan, matching and inspection Activity (#167, 23 October 2026)

Plan: [SCAN-ACTIVITY-PLAN.md](SCAN-ACTIVITY-PLAN.md). Each library task reports a bounded
set of concurrent operations with stage, subject and optional detail. Stages cover actual
file checking/inspection, movie/series matching on TMDB/TheTVDB, saving metadata and artwork
references, episode/order mapping/saving and linking files using saved episode metadata.
Independent format analysis identifies its library/title/filename and retains the subject in
finished history. No artwork-download stage is claimed for saved references. Reporting adds
no provider/probe calls or per-file title queries to unchanged discovery; title lookup occurs
only for actual file inspection. Filenames are basenames, not media-root paths.

Discovery has an unknown growing total, so Activity stays indeterminate. Known stage counts
start at zero. Finished summaries distinguish reused file details, successful reads, inspection
errors, deferred metadata updates and unrecognised files. Existing counters/matching/scheduling
remain; the added status fields are additive and need no migration. The UI wraps titles and
complete summaries (including failed scans), shows two operations and expands additional ones
and details. Activity stays admin-only and retains its Dashboard link.

Verification passed: production/app and e2e TypeScript, affected Go packages (activity, scanner,
background, store, API, API reference), Go vet and final embedded-server build. Complete affected
tracker/scanner/background race packages passed; subsequently added held-probe cancellation
checks and final analysis-history wording passed focused race checks. Backend fixtures hold
two movie provider calls concurrently, hold known movie/series refreshes, confirm cached rescans
reuse details with no repeated provider calls, observe an actual held probe child through
cancellation, count failed inspections, and verify analysis identity/cleanup and snapshot copies.

Six focused Chromium journeys passed without retries (42.6 seconds): real delayed movie/TV
imports expose matching subjects/stages through /api/activity; unchanged series rescans do not
probe again; Settings/Activity Dashboard navigation remains usable; desktop/390px concurrent
Activity/details/summaries wrap without overflow; paused scheduled work remains distinct. Final
copy/screenshot rerun: one journey passed (9.5 seconds). Actual desktop/phone screenshots were
inspected. Rich concurrent UI rows/counts are synthetic fixtures, separately supported by real
scanner/probe tests. No large-library/NAS/device/Unraid performance claim is made.

Actual evidence: `/workspace/arc-cloud-evidence/scan-activity/`. The initial sandbox attempt
could not open a loopback fixture listener; supported command review allowed it. An initial
test compilation error (unused loop variable) was corrected and rechecked. Both initial logs
remain. Only temporary stores/generated clips/local providers and a disposable probe helper
child were used; no owner data/settings/media, keys, persistent server or new tools. The full
unrelated suite was not repeated; hosted CI/image publication is separate evidence.

## Settings management navigation (#166, 23 October 2026)

Owner-approved plan: [SETTINGS-NAVIGATION-PLAN.md](SETTINGS-NAVIGATION-PLAN.md).
The desktop Manage group and mobile Manage menu are replaced with a Settings entry.
Settings and all six dedicated management pages share grouped navigation: personal
preferences, Server, Libraries, Playback, People & access, Maintenance. The health count
moves with Library Health. Existing management URLs, query parameters and settings-section
links remain valid; phones use a grouped section selector. Activity includes View Dashboard
even with no active tasks and closes when followed. Ordinary profiles see personal
preferences only; direct management URLs redirect before admin components/queries mount.
API authorization is unchanged.

Production build/app TypeScript and e2e TypeScript passed. Five focused Chromium journeys
passed without retries (37.3 seconds): all management destinations and active states, old
URLs/query state/reload/history, Activity keyboard/touch linking and popover dismissal,
desktop/390px navigation without overflow, section anchors, settings saving/error/retry,
mobile profiles/player controls and household-profile protection. The health badge test
injects a synthetic 900 count; it is not an owner-library measurement. Actual screenshots
were inspected. Initial checks caught an active-link aria-current bug (fixed by explicitly
marking the containing Settings link) and an old exact-name locator that omitted the health
badge's accessible count (updated). Initial failure logs/traces remain preserved.

Evidence: `/workspace/arc-cloud-evidence/settings-navigation/`. Tests used temporary
databases, generated clips and local metadata fixtures; no owner data/settings/media or
Unraid access. No backend changes, unrelated full Go/race suite or full browser suite rerun;
hosted checks/publication and owner deployment remain separate evidence. No persistent server.

## CI FFmpeg package download timeout (#165, 23 October 2026)

Run [39941884612](https://github.com/JonzaUK/arc-media-server/actions/runs/39941884612)
at `3a470b1` failed at the five-minute FFmpeg installation limit. Apt selected
`http://azure.archive.ubuntu.com/ubuntu`; downloading the 9 MB libcodec2 package
took over three minutes. Linux Go/race/browser checks never started. Windows checks,
container acceptance and multi-platform image preparation passed; release tagging was
skipped, leaving the prior published image in place. HDR/audio commit `e5f36e0` had
passed its separate run 39931694480; that does not validate the scheduling commit.

The shared GitHub-runner prerequisite helper now replaces official Ubuntu HTTP/Azure
archive entries with the official HTTPS archive, including the runner's apt mirror
list. It preserves suites, components, signing keys and third-party sources, omits
optional recommended packages, retains bounded retries/timeouts and verifies both
FFmpeg tools. Both workflows use the helper; the cloud preflight also triggers on helper
changes. All Go/race/browser/container tests and release dependencies are unchanged.

Local verification: eight isolated checks passed (zero failures/skips): source/mirror
rewrite and signature/component/vendor preservation, idempotence, update/install failure
propagation, unusable ffprobe rejection, non-GitHub rejection, and each workflow's retained
verification steps. Bash syntax, YAML parsing and diff checks passed. Tests stubbed package
commands against temporary source fixtures; no host packages or saved environment state
were changed. Full product tests were not repeated locally for this CI-only change.
Hosted download performance, tests and image publication require the new GitHub run.
Actual failed-job log and results: `/workspace/arc-cloud-evidence/ci-ffmpeg/`.

Target: an alpha release other people can test. Implemented features and verified deployments
are separate: a closed feature milestone does not establish that every device works.

Track this work in milestone **21. Stability and release readiness**, issues #130–#138.
Use isolated databases and copies of data for recovery tests. Never modify the owner's media.

Global scanning controls are implemented: see [SCANNING-CONTROLS.md](SCANNING-CONTROLS.md) and
[#147](https://github.com/JonzaUK/arc-media-server/issues/147). The owner superseded the earlier
per-library proposal with server-wide controls. Isolated verification is recorded below; real
network-share notification behaviour and sustained-use readiness remain separate acceptance work.

## Global scanning controls (#147, 22 October)

The admin settings now expose server-wide automatic change scanning and a full-discovery interval
(Off/hourly/6-hourly/12-hourly/24-hourly). Advanced contains partial scans and startup scans.
Defaults retain automatic/partial/startup On and an hourly fallback. Explicit false and Off survive
database reload. Runtime updates and restored settings use the same watcher configuration hook.
Manual scans, first-library imports and explicit restore reconciliation remain independent.

The watcher replaces its timer when the interval changes, removes it for Off, clears pending
notifications when disabled and requests one catch-up scan when re-enabled. Changes arriving
during existing scans retain the scanner's bounded/coalesced follow-up behaviour. Known-root
availability checks register returning folders independently of periodic discovery; they do not
repeatedly walk healthy roots. Library Health reports actual notification/registration errors
with the configured fallback, rather than promising an hourly scan when scheduling is Off.

Nightly maintenance now refreshes known database titles instead of requesting full discovery.
It shares each library's scan admission slot and has its own Activity entry. Movie refresh
failures in this maintenance path do not schedule a discovery retry. Backups and other maintenance
managers retain their own scheduling. Existing scan-driven provider retries are unchanged.

Isolated evidence:

- Settings/API regressions verify defaults, persisted Off, supported intervals, explicit false,
  partial patches, and rejection of incorrect types/null/unsupported intervals.
- Controlled-clock watcher tests cover timer replacement/removal, keeping a timer for unrelated
  setting changes, re-enabling without replaying missed ticks and a single change-scan catch-up.
  Direct event tests cover partial/full dispatch, disabled pending changes and error guidance.
  An actual temporary missing/returning local directory verifies reattachment with scheduling Off.
- Movie and TV local TLS fixtures verify known metadata updates, locked-title preservation,
  serial admission and absence of newly discovered titles during maintenance. Existing offline-root,
  episode identity and history-preservation regressions remain in the full Go suite.
- The real-app Chromium journey verifies the controls, reload persistence, disabled Advanced
  dependency, failed-save recovery, first-library/manual scans with triggers off, startup Off/On
  across real process restarts, no discovery after waiting beyond settling with changes disabled,
  and ordinary-profile UI/API protection. The restore journey checks all four restored values.
  Screenshots at 1440px and 390px were inspected.

Final local Windows verification passed: production UI and app/e2e TypeScript builds, all
23 Chromium journeys without retries (4.3 minutes), `go vet ./...`, `go test ./...`, regenerated
API reference and `git diff --check`. Subsequent watcher-only safeguards passed that package's
regressions. Local CGO is disabled, so Linux race checks and container acceptance/publication
are separate CI evidence. Nothing here establishes real NAS notifications, real-device playback,
hardware performance or sustained Unraid acceptance; owner data/settings/media were not used.

## Acceptance checklist

The broader UI assessment and approved implementation are recorded in [UI-REVIEW.md](UI-REVIEW.md)
and #148–#152. Desired-behaviour regressions in `web/e2e/ux.spec.ts` replace the original four
observed-gap diagnostics and run with the ordinary release suite. They cover error/retry states,
keyboard ordering, custom-row full results, profile-safe playback/queue edits, mobile geometry,
settings navigation, audio preference, order inheritance and remembered sorting. Source review
and generated-video viewport checks do not establish actual device or sustained-use acceptance.
| Area | Evidence required | Status |
| --- | --- | --- |
| Access controls (#130) | Regressions for forwarded sign-in, HTTPS cookies, cast revocation | Implemented; local regression tests pass |
| Conversion admission (#131) | Simultaneous starts respect limits; remux allowed; stopped/failed starts release capacity | Implemented; local regression tests pass |
| Build gates (#132) | Linux/Windows checks and Linux race/media tests pass before Docker publication | Passed for b5d8e39; see verification below |
| Browser journeys (#133) | Setup, profiles, library preview/import, fix match, order changes, playback/resume and denied access exercised in a browser | Six journeys pass locally and in Linux CI for 49ac1f5 |
| Player-close progress (#140) | Real video seeks beyond the resume threshold, closes, reloads and resumes at the saved point | Fixed; browser regression passes locally and in Linux CI |
| Library correctness (#134) | Representative real library; rescan/edit/provider/order history preserved; missing drive recovery; large-library measurements | Synthetic rescan/order/edition/local-asset regressions pass; broader and real-library validation pending |
| Playback matrix (#135) | Actual devices, codecs, seeking/audio/subtitles, HDR, encoders, casting | Automated Chromium direct/remux/software conversion coverage; actual device matrix pending |
| Recovery (#136) | Old-schema upgrade, backup restore, unavailable providers/drives, interrupted scan/conversion, unwritable/full storage | Isolated upgrade/restore/outage/storage/shutdown checks pass; full-disk, abrupt-exit and deployment recovery pending |
| Sustained use (#137) | 7–14 days on Unraid; start/end resource measurements, problems recorded | Pending |
| Packaging (#138) | Fresh Docker install, persistence/permissions, upgrade/rollback, attribution and distribution decisions | linux/amd64 install/persistence and pinned-image upgrade/snapshot rollback pass; hardware/ARM/terms pending |

## Recording results

For each run record the commit, date, OS/container, browser/device, FFmpeg version, relevant GPU,
fixture (without private paths or keys), expected outcome, actual outcome and logs/test result.
Mark unsupported and untested cases explicitly. Automated encoder-argument tests do not establish
real hardware support. A successful local build does not establish a successful CI run.

For playback, use rows for MP4 H.264/AAC, MKV remux, unsupported audio conversion, HEVC/HDR,
multiple audio tracks, text subtitles, image subtitles and multi-episode files. Record direct,
remux and video conversion separately, including seeking and resume. Add Chromecast/AirPlay rows
only for devices actually exercised.

Before the server trial, identify the baseline database size, CPU/memory at rest, temporary disk
use and expected background work. Sample them during ordinary use and after playback/maintenance
ends. File reproducible defects rather than treating a quiet log as proof of correctness.

## Initial changes (21 October 2026)

- Forwarding headers cannot grant home-network privileges. Any request carrying `Forwarded`,
  `X-Forwarded-For`, `X-Real-IP`, `X-Forwarded-Host` or `X-Forwarded-Proto` requires normal sign-in
  and is treated as remote for bandwidth limits. Direct home-network access keeps its existing behavior.
- HTTPS session and logout cookies are Secure. `X-Forwarded-Proto: https` may strengthen cookie
  protection, but cannot grant local privileges or downgrade a TLS connection's cookie.
- Cast stream requests validate the originating session and its current profile; deleting that
  session, changing its profile, or using a home-only session remotely rejects subsequent requests.
  This does not retract bytes a player has already buffered.
- Conversion capacity is checked and the process registered under the same lock. Startup failures
  release the lock; stopped sessions free their slot; video-copy remuxes do not use encoder slots.
- Pull requests and main pushes build the UI, analyse Go, run tests and build the embedded server
  on Linux/Windows. Linux additionally installs FFmpeg and runs race detection. Docker publishing
  depends on both check jobs passing.

The local checks use temporary stores; the owner's database is not opened or migrated by this work.
Real-device playback, proxy deployments and sustained use remain pending.

## Verification on 21 October 2026

Code commit: `b5d8e3915614b8f2c854164d9b43cf30099340cb`.
[GitHub run 39630770097](https://github.com/JonzaUK/arc-media-server/actions/runs/39630770097):

- Windows and Linux: production web build (including TypeScript), `go vet ./...`,
  `go test ./...`, and the server build with embedded UI passed.
- Linux: installed FFmpeg, ran media integration tests and `go test -race ./...`; passed.
- Local Windows checks also passed. With FFmpeg available, fresh tests exercised H.264/HEVC
  remuxing, generated HDR conversion (detected QSV encoder), subtitles, intro/credits detection
  and timeline thumbnails. This does not establish real-client compatibility.

The Docker publish job is gated on both check jobs. Image installation/upgrade testing remains
in #138; successful code checks do not close the Docker packaging issue #123.

## Alpha exit criteria

- No known critical authentication/access or data-loss defects.
- Core browser journeys and backend checks pass in CI.
- Agreed playback/device matrix has results and documented limitations.
- Fresh installation, supported upgrade, backup recovery and rollback are demonstrated.
- Sustained server trial has no unexplained resource growth or unresolved critical failures.
- Required attribution, distribution terms and installation/support documentation are ready.

## Browser regression suite (#133)

`web/e2e` uses Playwright Chromium against the production embedded app, a local TMDB stand-in,
generated H.264/AAC clips and a fresh temporary store for each test. Inherited `AMS_*` environment
variables are removed; the owner's settings, keys and media are not read. Browser internet requests
are blocked. Metadata, marker and ratings services point to local stand-ins.

Six journeys cover setup/password validation and sign-out/in; movie preview/import and fixing an
unmatched title; empty previews and service errors; alternate episode orders and unplaced files;
wrong/correct profile PINs and library/admin restrictions; real playback and persisted resume.

The player journey exposed #140: passive unmount cleanup ran after the video ref was cleared,
so the last position was not saved. Saving in layout cleanup fixes it. The test seeks beyond Arc's
intentional one-minute resume threshold, verifies a final `stopped` save, reloads, then plays from
the saved point. The local run passed all six journeys and captured the saved 1:10 resume point.

Linux CI runs the suite with no retries and uploads the HTML report and failure traces/screenshots/
video for seven days. These tests do not replace the device matrix or sustained server trial.

Verified code commit: `49ac1f5fdd079ebe7131cc7b8a98ccf2c9fc89db`.
[GitHub run 39635142227](https://github.com/JonzaUK/arc-media-server/actions/runs/39635142227)
passed both Windows and Linux check jobs, including the Linux browser suite and race checks.
The Docker publish job started after both check jobs succeeded. Local verification also passed
all six journeys with no retries; a screenshot records the saved 1:10 resume point after reload.

## API documentation follow-up

Issue #139 tracks an eventual `/docs` page: a maintained OpenAPI contract, searchable reference,
authentication/device-linking/playback/orderings guides and realistic examples. Establish the
contract while validating API behavior; an interactive documentation UI is separate follow-up
work, not a reason to postpone authentication or data-integrity fixes. `docs/API.md` remains the
current generated route reference.

## Recovery validation follow-up (21 October 2026)

Store regression tests construct a genuine schema-3 database from the embedded historical SQL,
restore it online into a current store, and check that its edited title and field lock survive,
all current migrations apply, and later session columns are usable. Invalid empty, gapped and
future migration histories must be rejected before the backup manager replaces live data.
This covers synthetic upgrade/restore behavior, not a real deployment or all of #136.
Local Windows validation passed: production web build (including TypeScript), `go vet ./...`
and `go test ./...` with Go 1.27.0 and FFmpeg/ffprobe 9.0.2. The restricted sandbox blocked an
existing backup test's temporary rename; the suite passed when rerun outside that sandbox.

Built-in `.db` backups contain database state only. They do **not** include uploaded artwork
in `data/uploads`, TLS material in `data/certs`, or optimized video copies in `data/optimized`.
For a complete server recovery, stop Arc and copy the entire data directory to separate storage;
retain media separately at its existing paths. Preserve certificates/private keys securely.
Provider artwork, subtitle caches, trickplay images and conversion output can be regenerated;
uploaded artwork cannot be recovered from SQLite alone. Restoring a built-in database backup
does not roll these separate files back. Packaging/full-directory recovery remains unverified.

### Isolated failure scenarios

Additional Windows regression checks (Go 1.27.0, FFmpeg 9.0.2) use temporary stores/files and a
local HTTP provider stand-in; they do not open the owner's database or media:

| Scenario | Verified result |
| --- | --- |
| Library root unavailable, then reattached | Existing file stays present; identity, edited title and resume point survive |
| Library root replaced by a regular file | Treated as unavailable, not an empty library; fixed missing-file sweep safety |
| Individual file removed from accessible root, then returned | Missing flag set and cleared; original file/item IDs, edit and resume point retained |
| Walk interrupted after first file; cancelled scan followed by fresh scan | Cancellation returned; fresh scan succeeds without losing existing state |
| TMDB returns HTTP 503, then recovers | Item stays pending; retry matches it while retaining locked title and resume point |
| Backup directory obstructed by a file | Backup fails without changing live data; succeeds once storage is repaired |
| Corrupt backup selected | Restore rejected before creating a safety copy or changing live data |
| Restore followed by undo using the safety copy | Settings, locked title and resume point return to each corresponding snapshot |
| Conversion directory obstructed by a file | Start fails without retaining encoder capacity; next start succeeds after repair |
| Server conversion-manager shutdown | Real FFmpeg process exits and session is removed |
| Stale conversion output after restart | New manager clears synthetic orphan output and a new real conversion succeeds |

The new tests pass along with the full Go test suite and `go vet ./...`. Filesystem rename and
local-provider tests run outside the restricted agent sandbox. These checks establish application
recovery behavior, not physical drive failure, an actual full filesystem, abrupt whole-server
termination during scanning, a complete data-directory restore, or a real Unraid deployment.
Those remaining cases keep #136 open. The broader library/provider/order matrix remains in #134.

### Synthetic library integrity (#134)

Scanner integration tests now exercise repeated scans of a synthetic TV library with a shared
S01E01E02 file, an S00 special and an unknown S01E20. Four episode identities are retained;
the two episodes keep their shared file; custom display order, locked manual placement and
resume progress survive three further scans. A movie fixture checks two editions group into
one title, same-name films from 1977/2018 stay separate, and local poster/NFO identity is read.
Every fixture's original bytes are checked afterwards to detect writes to source media/assets.
Previously read files are seeded, so these tests validate scanner/database integration rather
than media decoding. Both tests pass on Windows; #134 stays open for representative real-library
and broader provider validation.

Additional checks cover anime absolute numbering (`012v2`) and date-based files: three scans
attach them to the existing cached episode with no duplicate identity. An adoption regression
exercises the helper used by metadata sync to re-key a file-only episode to TVDB, then TMDB and
back. The same item/file IDs, locked title, resume point, recorded play-history row and locked
custom placement survive. Cached provider IDs still map when provider titles/air dates differ.
This is component-level provider-key preservation, not a full two-provider HTTP refresh/switch
or proof against the live providers. Local `go vet ./...` and `go test ./...` pass.

### Synthetic 1,800-file measurement

Run: `go test ./internal/scanner -run '^$' -bench BenchmarkLibrary1800 -benchtime=1x -benchmem`.
Windows amd64, AMD Ryzen 7 7700X, Go 1.27.0, 21 October 2026: **2.791 s**, **109,955,864 bytes
allocated (104.9 MiB total)** and **1,119,104 allocations** for one first grouping scan of 1,800
synthetic movies. Fixture/database creation is outside the measured section. Files are seeded as
already probed and online providers are disabled. This measures scanner grouping/database work,
not disk decoding, provider latency, peak resident memory, or end-to-end Unraid throughput; it
does not replace the representative-library timing/memory acceptance criterion.

## Upgrade and runtime restore safety

Startup now rejects unknown/newer schema versions before migration hooks or derived-data updates.
An older executable must be paired with a compatible pre-upgrade backup to roll back; simply
reusing a newer database is refused. Temporary historical-schema tests verify that a failed
safety backup aborts the upgrade and that a successful retry creates a restorable safety copy.

Online database restore now reloads runtime settings as well as libraries: access controls,
provider clients, region, maintenance, conversion limits/location, home networks and remote/TLS
settings. Provider keys are not validated online during recovery. The restored ffprobe path is
rediscovered, with automatic discovery as a fallback. If the restored port cannot be opened,
restored access controls still apply and the response explicitly says the database was restored
but Arc needs restarting. A real-app browser regression verifies region, conversion limit and
local sign-in bypass immediately reflect the backup, without restarting the server.

## Container acceptance gate (#138)

`node tools/docker-smoke.mjs <local-image>` creates only uniquely named disposable Docker volumes
and containers, with no host-directory mounts or network access. It checks fresh setup, packaged
FFmpeg/ffprobe, read-only root filesystem and media, root/non-root execution with pre-provisioned
volume ownership, database ownership, backup/restore, health, restart and replacement-container
persistence. GitHub builds a local linux/amd64 image for this test and image publication depends
on its success as well as the existing Linux/Windows checks. Initial linux/amd64 acceptance passed
for `2387a56` in [run 39654467547](https://github.com/JonzaUK/arc-media-server/actions/runs/39654467547),
including both root/non-root scenarios; Linux/Windows check jobs also passed.
Docker is not installed on this Windows PC; no local container result is claimed.

This gate does not yet prove ARM runtime behavior,
PUID/PGID convenience support, actual NAS permissions, GPU passthrough, hardware playback,
external networking, attribution or provider distribution terms. #123/#138 remain open.
Published provider requirements and the unresolved project-key/subscriber-PIN decision are
recorded with official sources in [DISTRIBUTION.md](DISTRIBUTION.md); #64/#65 remain release blockers.

The gate now also accepts a previous immutable image tag. On main pushes it seeds that image,
stops it, copies its complete data volume (including a sentinel in uploads), upgrades the
original volume to the new image, then runs the previous image against the separate snapshot.
The pinned baseline is `2a7029a1a0871a459424336146a9fdb73c107001`; cross-image CI acceptance passed
for `31597fa` in [run 39655440173](https://github.com/JonzaUK/arc-media-server/actions/runs/39655440173).
Both root/non-root scenarios passed, including the separately preserved uploads sentinel.
Pull requests run the fresh-install gate without requiring private-registry access.

## Additional process/storage/provider checks

- A separate scanner/store process is forcibly killed after reaching its file-probing phase
  (the probe is delayed so no child FFprobe remains). Reopening the on-disk temporary database
  and scanning again preserves file identity, edited title and resume, and passes integrity
  validation. This verifies a real process kill of the scanner/store subsystem, not every
  background task in a deployed server or physical power-loss durability.
- SQLite `max_page_count` limits a disposable database to simulate `SQLITE_FULL`. A failed large
  setting write retains its previous value, `integrity_check` passes, and the write succeeds
  after removing the limit. No real filesystem is filled; FFmpeg/full-filesystem behavior
  still needs its own validation.
- A full metadata-sync integration test routes every provider connection into a local TLS
  fixture, using real TMDB/TVDB clients. It refreshes a file-only episode, switches source
  numbering TVDB -> TMDB -> TVDB, rescans, and checks the original episode/file IDs, field lock,
  resume, custom display order and manual placement. This exercises HTTP parsing and sync as
  well as rekeying; live-provider variations and representative real libraries remain pending.

Local Windows full `go vet ./...` and `go test ./...` pass. The previous `2a7029a` and `b0bb836` GitHub runs completed
successfully, including Linux race/browser checks, Windows checks and image publication.

## Converted playback regression follow-up (#135)

Generated fixtures extend the Chromium journeys with H.264/AC-3 MKV (remux plus audio conversion)
and MPEG-2/AC-3 MKV (software H.264 conversion). They verify actual video progression, rapid
forward skips past one minute, final saved progress and resumed playback. Hardware encoding is
disabled and the conversion limit is one; these checks do not establish hardware or TV support.

The journeys exposed two player defects: overlapping restart responses could install an older
stream and lose accumulated skip presses; remux starts at an earlier measured keyframe were
not advanced to the requested point. Restarts now serialize admission, discard superseded plans,
release prior sessions, retain the latest requested position and ignore obsolete stream errors.
HLS playback seeks within its measured output offset. The production web build and all nine
local Windows Chromium journeys pass without retries, including remux and software conversion
with a one-conversion limit. Real TV/mobile/HDR/hardware/casting results are still pending.

An additional old-schema restore regression exposed stale derived parental ages. Restore now
recomputes those ages after migration, as startup already does; the historical rating label
retains its correct age without requiring a server restart. Local full Go checks pass.

The owner reports Arc is already running successfully in an Unraid Docker container, with no
installation problems. The current priority is bugs inside Arc; further packaging is not the
focus. A timed/resource-measured trial has not been documented. [SERVER-TRIAL.md](SERVER-TRIAL.md)
remains an optional blank record for #137, not a request to reinstall or start a separate server.

## Everyday bug investigation

Reproduced with isolated fixtures:

- A delayed English subtitle response overwrote French subtitles after switching language, and
  made subtitles reappear after choosing Off. Subtitle selections now invalidate older cue/error
  responses, including after a file/version change or player close. Browser regressions deliver
  the earlier real subtitle response after the new selection.
- Continue Watching returned nothing when its thirty most recent resumes were dismissed or
  inaccessible, even with an older visible resume available. Dismissal and profile library/age/
  label filtering now happen before the result limit. API regressions cover all four cases.
- Safety backups made before removing library data were incorrectly labelled as manually created
  backups. The UI now recognizes `before-delete` and shows “Saved before a removal”. A real-app
  journey removes an isolated library and verifies the backup's label.

The previous `ba031d4` passed all GitHub jobs in run 39657158172, including Linux race/browser,
Windows and container upgrade/rollback checks. Current fixes pass the production web build,
TypeScript/e2e type checks, all twelve local Chromium journeys without retries, `go vet ./...`
and `go test ./...`. Continue Watching tests cover movies and episodes with show-level controls.

## UI flow review (#144)

The owner requested a review pass first, suspected defects logged in GitHub next, and then
reproduction before fixes. [Issue #144](https://github.com/JonzaUK/arc-media-server/issues/144)
records the initial six unconfirmed candidates and the before-fix reproduction results.
Source review covered navigation/authentication, home/library/title/series pages, search,
watch state, collections/playlists, player entry, settings/profiles/devices and administrative
pages. A separate Chromium walkthrough opened fourteen main pages at 1440px and 390px widths;
it checks page rendering, not every control or successful external-service operation.

All six candidates reproduced before product code changed:

| Candidate | Observed behavior | Change |
| --- | --- | --- |
| Large-library browse | Reports 501 titles but renders 500 with no way to reach the last | Load more uses the existing offset API and retains loaded titles |
| Search navigation | Clearing the textbox leaves the previous URL/results | Clearing updates the URL; Back/Forward updates the input |
| Search failure | Local HTTP 503 leaves loading UI instead of the error | Display query errors explicitly |
| Playlist picker | Offers a smart playlist whose API forbids manual additions | Only offer editable playlists |
| Player entry | Delivering an earlier real pre-roll response replaces the latest Play choice | Ignore superseded lookups, including after player close |
| Bulk selection | Changing libraries retains selection mode | Reset selection and shift-click anchor when the library/filter/view changes |

SelectionBar already limits edited IDs to the displayed items; cross-library edits were not
reproduced or claimed. Pagination uses a synthetic 501-title browser API listing to isolate UI
behavior; a separate check exercises actual API offsets against the two real generated titles.
This is not a 501-file scanner benchmark. Search failure is deliberately injected locally.
Playlist and player checks use real isolated server state/responses. No owner data is accessed.
Seven new browser checks bring the suite to nineteen journeys; screenshot evidence includes
the paging control, search failure and desktop/narrow layouts. This bounded review does not
establish that every UI action, real device, provider or sustained Unraid run is defect-free.

The previous `f3de41d` passed all GitHub jobs in run 39672301946. This pass completed the
production web build, TypeScript/e2e checks, all nineteen Chromium journeys without retries,
`go vet ./...`, `go test ./...`, the embedded-server build and `git diff --check` locally.
The current commit's CI outcome must be checked separately; local checks do not establish it.

## Scanning investigation (#145)

The owner reaffirmed Plex-like feature coverage and simple automatic defaults, with a scanning
investigation before the broader UI/feature gap review. No production scanner/UI behavior was
changed in this investigation. [Issue #145](https://github.com/JonzaUK/arc-media-server/issues/145)
tracks proposed work and [the scanbench notes](../tools/scanbench/README.md) record commands,
measurement boundaries and results.

Against product baseline `c22b671`, a fresh real-server import of 120 generated MP4s with a
local provider delayed by 200ms and real ffprobe delayed by 300ms finished in 21.97s for direct
API addition, or 30.06s including a completed Preview. Requests were 240 vs 360: movie Preview
fetches full details but only its match candidate is reused. The UI allows adding before the
preview completes, so this is not an enforced minimum wait. The old milestone-20 preview
timings started after Preview, which excluded that work from the comparison.

Synthetic provider-free/probe-seeded 1,800-movie measurements repeated three times gave
2.848–2.875s unchanged and 2.872–2.889s with one added file. The watcher discards changed paths,
debounces the entire library for ten seconds, and requests a full walk. The walk finishes
before matching begins; unchanged films still check local assets and replace file links.
For a usual own-folder film without local assets, the current search loops perform 68 artwork
existence checks plus theme/NFO checks. This is a source-level count, not measured NAS I/O.

TV sync fetches available TVDB metadata/orderings first when configured, then TMDB and its
orders, before linking files. Fetching essential source numbering/details first and deferring
secondary orderings could shorten readiness, but requires explicit tests preserving episode
identity, custom/manual ordering and watch history. Never change canonical numbering silently
because another provider is faster. Matchers' fallback to the first non-exact provider result
also needs an ambiguity corpus; no live-library false-match rate was measured.

Recommended work is full preview-metadata reuse, targeted folder scanning with independent
settling and safe full reconciliation, less unchanged filesystem/DB work, incremental matching,
prioritized essential TV data and confidence/retry improvements. Existing worker pooling,
provider pacing, season batching and on-demand probing should remain. Library grids already
refresh during scans; absent progress updates were not established as a defect.

Plex documents [partial scans and OS/network notification limits](https://support.plex.tv/articles/200289526-library/)
and [separate maintenance work](https://support.plex.tv/articles/201553286-scheduled-tasks/).
TMDB documents [changing rate limits and respecting 429s](https://developer.themoviedb.org/docs/rate-limiting).
These describe reference behavior, not a measured Plex benchmark. All local checks use generated
or synthetic files and temporary stores; none establish Unraid throughput, physical-disk
performance, provider accuracy, peak RAM, hardware/device support or sustained-use readiness.

### Direct background library addition (#146)

The owner approved removing Preview from the ordinary add flow. Movie, TV and other-video
libraries now use Add Library directly; the existing background scan starts on creation.
Advanced options are collapsed for additions, with Preview folders available there when wanted.
Existing-library editing opens its settings. Folder existence, overlap and a lightweight read
check run before saving, without enumerating the entire library or querying metadata providers.

Isolated browser regressions delay the real fixture provider and verify movie/TV creation closes
the dialog while scanning is still running, registers the library immediately, makes no Preview
request, and eventually matches the generated title. Invalid-folder creation fails without saving
a library and succeeds on retry. Optional Preview still handles empty folders and service errors;
a failed optional preview does not prevent direct addition of the valid folder. Shared helpers
now use direct addition, exercising it throughout the existing movie/TV/playback/profile journeys.
Go regression coverage checks readable, missing, non-directory and absent folders; Unix access
denial is checked only on non-root Unix runs (not claimed from Windows permission bits).

This removes a step and preview work from the default path. Targeted scanning, unchanged-file
optimizations, complete preview-metadata caching and deferred TV order fetching remain open on
#145. This change does not establish new Unraid throughput or real-device readiness results.

Final local validation passed the production UI build and app/e2e TypeScript checks, all
22 Chromium journeys without retries (2.1 minutes), `go vet ./...`, `go test ./...`, the
embedded-server build, fixture-server syntax and diff checks. Desktop movie and 390px TV
addition screenshots were inspected. GitHub CI for the new commit is separate evidence.

### Scanning implementation (#145)

The owner clarified that approval covers all five scanning recommendations. Implemented:

- Preview keeps successful raw provider responses for up to two hours, bounded to 64MiB/4096
  entries per provider client. Keys include request/language/API-key identity; region-specific
  rating selection is recalculated from the raw data. Errors are not cached. Explicit refresh
  fetches fresh data and invalidates its old response. Add still works without Preview.
- Watch notifications retain paths and settle independently. Scans normalize changes to their
  movie/show folders, keep original library roots, queue changes arriving during a scan, and
  fall back to full reconciliation for root events/large batches. Hourly/manual full scans
  remain. Missing-file sweeps are confined to completely read scopes; unavailable roots,
  walk/record failures and untraversed directory symlinks suppress destructive absence claims.
- Unchanged file presence updates use short batches; unchanged file associations avoid rewrite
  transactions. Artwork checks enumerate each directory rather than issuing dozens of absent
  path stats; episode linking shares season-folder enumerations. Sidecar detection and Windows
  filename case behavior are retained.
- Discovery feeds six matching workers through a bounded twelve-job queue. Movies match as
  files arrive; TV matches as show folders finish. Split folders for a show serialize their
  updates. Workers join before missing-file reconciliation or scan completion, including errors.
- New TV shows load selected numbering/details and link files before secondary orders. Existing
  episodes always take the complete mapping path, including after invalidation/provider changes.
  Secondary failures leave essential data usable and unsynced state durable for restart/retry.
  The series UI refreshes orders/seasons during scanning and once more when it completes.
- Ambiguous searches require a strong title score and separation from the next distinct ID;
  conflicting sequel numbers are rejected. Explicit IDs remain authoritative. Uncertain titles
  stay visible in Fix Match with an explanation. Transient HTTP/transport failures retry with
  backoff/Retry-After and cancellation; exhausted scans schedule one retry per library, starting
  at one minute and capped at fifteen. Old unmatched movies retry during daily reconciliation.

Isolated regression coverage includes both providers, ten ambiguity cases, preview language/key
separation, fresh explicit refresh, failed-response exclusion, safe Unicode scoped sweeps,
targeted TV identity, progressive discovery, split show folders, large change-batch fallback and
changes queued during completion. A blocked TMDB alternate-order fixture checks essential files
are linked before secondary completion; four 503 responses then a successful delayed retry keep
episode/file IDs, resume and a custom placement. TVDB→TMDB→TVDB transitions (including invalidated
metadata during a background rescan) retain locked edits, episode identity, files and watch state.

Sequential local measurements on Windows amd64/Ryzen 7 7700X: synthetic 1,800-movie unchanged
rescans 0.835–0.853s versus 2.848–2.875s baseline; scoped addition 1.69–2.42ms. TV unchanged
0.135–0.265s; a scoped 61-file show addition 8.47–10.00ms. A single cold grouping trial took
1.27s for movies/1.36s for TV (baseline 3.38s/1.61s). The generated 120-movie import with
200ms local-provider delay/300ms probe delay: direct first match 0.50s, complete 21.66s; including
Preview first match 12.53s, complete 26.08s. Both now use 240 requests (previous Preview 360).
Full direct-import completion is essentially unchanged; no universal first-scan speedup claimed.
See [scanbench](../tools/scanbench/README.md) for commands and detailed measurement limits.

`go vet ./...` and `go test ./...` pass locally against isolated stores and localhost providers.
The production UI/app/e2e TypeScript build and all 22 Chromium journeys pass (2.1 minutes);
later thumbnail and reconciliation safeguards are covered by the complete Go regressions.
Local Go has CGO disabled, so Linux race detection is a separate CI gate. Generated/synthetic
fixtures do not establish real-library acceptance (#134), real-device playback, physical NAS I/O,
live-provider match accuracy, peak RAM or sustained-use readiness. No owner data or deployment
was used or modified.

## Approved UI follow-up (#148–#152, 22 October)

Implementation covers all eight reviewed areas: Home feedback/keyboard arrangement, shared
browse filters/full results, display-order scope/inheritance, grouped Settings, explicit audio
preference, mobile controls/profiles, editable queue/compact playback, and visible recovery
paths. See UI-REVIEW.md for the feature-by-feature record and scope boundaries.

Local Windows verification: production UI build and app/e2e TypeScript checks pass. All 23
existing Chromium journeys passed in the complete run; its custom-row regression exposed an
empty editor when preferences arrived late. After fixing initialization and the friend-row
layout found in screenshot review, all 12 new regressions passed together without retries
(1.9 minutes). The suite now contains 35 journeys; the final combined Linux run is a separate
CI gate. Screenshots of long household/friend names, grouped Settings and 44px player controls
at 390px, plus the desktop editable queue with compact playback, were inspected.

`go vet ./...` and `go test ./...` passed; affected store/API/watcher and generated-reference
checks were repeated after the final test/doc changes. Browse regression checks reverse order
with runtime/rating filtering and pagination. Linux CI from the earlier scanner commit exposed
a data race in the controlled-clock test callback counter; asynchronous counters now use atomics.
Local CGO is disabled, so this does not substitute for Linux race verification.

Browser scenarios cover failed Home dependencies, failed preference/order writes with Retry,
audio persistence, keyboard Home/playlist ordering, 60 synthetic filtered results, comparison
input preservation and inherited order, rating/watchlist/refresh recovery, device-local/profile
sort separation, playback Retry, queue edits without restarting the current video, saved progress,
and switching profiles while minimized. Tests use isolated generated media/local providers;
requests to outside services are blocked. Queue state is in memory, ending on refresh/close;
sort memory is device-local. No owner data, settings, media or Unraid deployment was changed.
This completes the approved review follow-up, not actual device, hardware, NAS or soak acceptance.

Compact-player follow-up: keep the next-episode countdown active while minimized and show its
count/Cancel control. An additional generated-video regression seeks to the end and verifies
the second episode actually starts while still compact (passed locally, 22.1s including setup).
The final ordinary suite has 36 journeys (23 existing + 13 UI); production/app and e2e
TypeScript checks pass after this change. The final combined CI result is recorded on #148.
## CI-discovered split-show scanner synchronization (#153)

Final combined local UI verification passed all 36 Chromium journeys without retries (6.4m).
CI for 32e819c then failed Windows tests and Linux race tests in the existing split-show scan
regression; the container installation/persistence/upgrade/rollback job passed. The failure was
a SQLite FOREIGN KEY error, not a Go race report. Unlike the earlier watcher test-counter race,
this exposed a production scanner synchronization defect.

Directory grouping used raw directory spellings while EnsureItem identifies series by the
normalized series match key. Two workers could use different locks for the same series, letting
orphan cleanup delete an episode created but not yet linked by the other worker. Locks now use
seriesMatchKey, matching the row identity; unrelated series retain parallel matching.

The isolated regression now combines 18 independently named folders into one show, checks all
episodes/file links after both scans, and retains source-byte checks. It reproduced the same
failure in 7/10 local repetitions before the fix; 30 consecutive repetitions pass after the fix.
No real library/data was used. Final full server and CI outcomes are recorded on #153/#148.
Full local Go vet/tests pass after this correction; Linux race verification remains a CI gate.

Final verification for 4f09795 subsequently passed all Linux/Windows checks, Linux race and
browser checks, container installation/persistence/restore/upgrade/rollback and image publication
([run 39785261123](https://github.com/JonzaUK/arc-media-server/actions/runs/39785261123)).

## Queued database writes (#154)

The owner reported repeated `database is locked (5) (SQLITE_BUSY)` errors during library scanning;
logs were requested, and the runtime has not yet been specified. The former unrestricted pool
allowed multiple Arc writers to compete
until the ten-second busy timeout expired. An isolated regression reproduces that exact error
by holding a write transaction and requesting another setting write. Concurrent reads correctly
return the last committed value; a second write previously failed after about ten seconds.

The store now has one write connection and independent query-only readers. Exec, transactions,
INSERT RETURNING and raw online restore share the writer queue. Waiting requests respect
context cancellation. Restore releases its raw writer before migration/derived-data updates,
avoiding self-deadlock. WAL, immediate transactions and the existing busy timeout remain; no
schema change, reset or media modification is required. Read-only enforcement also catches
future mutations accidentally sent through a read-query path.

The contention regression now waits beyond the actual production timeout (11 seconds), verifies
reads remain available, releases the active writer and checks the queued value is saved. A
second regression checks cancellation prevents a queued change from applying later and that
subsequent writes recover. Full local Go vet/tests pass, including scanning, constrained-storage
atomicity/integrity, old-schema upgrade, backup and online restore regressions. All checks use
temporary stores/generated media. No owner database, settings or deployment was opened.

This fixes writer contention inside one Arc store; it does not coordinate two servers sharing
a database, repair filesystem locking, or establish that the owner's reported deployment is
fixed. SQLite documents [one concurrent WAL writer and network-filesystem limitations](https://www.sqlite.org/wal.html)
and [busy timeout behaviour](https://www.sqlite.org/c3ref/busy_timeout.html). Keep configuration
on suitable local storage even when media lives on network shares. Final browser/CI evidence
is recorded on #154.

Final local browser verification: all 36 Chromium journeys passed without retries (5.8 minutes),
including settings/restore, scanning, ordering, direct/remux/transcode playback and saved progress.
CI for the fix is separate evidence tracked on #154; no deployment confirmation is claimed.

The owner then supplied container logs showing four libraries starting together; three finish,
and library ID 3 subsequently fails with SQLITE_BUSY. This supports the concurrency diagnosis
but does not reveal the exact SQL operation. Scan failure logs now include the cached library
name, ID, scan phase and files seen, without querying the database again on the error path.
A real failed-background-scan regression and the full scanner package/vet checks pass. The owner
has not yet confirmed the fix on their deployment; the updated image must finish CI first.
## Diagnostics and poster-save responsiveness (#155/#156)

The owner approved diagnostic controls after reporting a series poster save that eventually
completed but took a long time. Admin Settings > Maintenance & recovery now exposes a single
Detailed logging switch (live, automatically off after 30 minutes) and Download diagnostics.
`-debug` also starts this temporary mode. No stored setting or restart is needed to toggle it.
Normal-mode logs retain >=2-second mutation timings and 5xx request context, using route templates
instead of raw URLs/query strings. Versioned API routes retain the same safe request context.

Recent records persist under the data directory's logs folder, bounded to three 2MiB JSONL files.
Normal restart reloads recent history and sequence IDs. Storage failures are visible, while the
recent in-memory records remain exportable. A ZIP contains redacted retained logs plus a manifest
of build revision/version, OS/architecture/Go version and aggregate writer queue/read-connection
statistics. CI container builds inject their actual commit SHA. No database, settings dump or
media is attached; logs can include profile/media names and paths. Known configured credentials,
sensitive attributes, bearer/query tokens, URL user-info and common webhook/link secrets are
masked in console/buffer/files and again during export. This is not a promise to anonymize all
user-entered text. API access, toggle and download are admin-only.

The Edit form previously PATCHed unchanged details before an artwork save. It now skips unchanged
metadata/locks, shows the current save step, and disables form input while saving. EditItem only
recomputes derived ages if the age-rating value changes. An explicit poster choice performs its
artwork save without needing metadata refresh or synchronous image download; automatic artwork
reset and intentional field unlock retain their existing refresh behavior. These remove duplicate
work, but the exact cause/duration on the owner's running server is not yet captured. New normal
request timing and database queue metrics provide evidence for any remaining deployment delay.

Validation: full local Go vet/tests pass. Controlled-clock tests cover expiry/renewal; filesystem
fixtures verify rotation, bounded size, restart IDs, unavailable retention and failed-write export;
redaction tests cover console/file/export, configured/short credentials and common token formats.
API regressions check validation/ZIP contents/build identity and normal-mode slow request logging
without raw query tokens. Production/app and e2e TypeScript checks pass. All 38 Chromium journeys
pass together without retries (7.4m), including live toggle/save retry/download failure+retry,
restart retention/default detail-off, ordinary-profile denial and 390px layout. A delayed real
artwork PUT verifies visible save progress, no unchanged details PATCH/refresh and a persisted
poster lock. Mobile diagnostics and desktop save screenshots were inspected. The poster/listing
image choices are local/synthetic; tests never use owner data or live provider credentials.

Plex provides debug logging and downloadable server logs in its
[troubleshooting workflow](https://support.plex.tv/articles/200250417-plex-media-server-log-files/).
Arc deliberately uses one temporary switch, bounded default retention and a small export without
extra verbosity/retention knobs. This choice is recorded on #80. Linux race/container publication
and actual server/device/sustained-use results remain separate acceptance evidence.

Final production build and both focused UI regressions pass again (28.1s) after the last save/
privacy safeguards. Container smoke now asserts persistent logging for root/non-root users,
reset of temporary detail mode plus retained history across recreation, and the CI-injected
revision. Its script syntax check passes locally; Docker is unavailable here, so those new
container assertions require CI and are not recorded as local acceptance.

## Cloud development environment (#157, 22 October 2026)

The owner approved Codex Cloud for cross-PC development with GitHub retaining completed work
and handoffs. The private `arc-media-server` environment is published with only Arc's repo,
the package-manager network preset and three owner-approved public Go/Playwright download hosts.
No production secrets, owner database/settings/media or Unraid connection were used.

Setup required a checksum-verified Go 1.27.0 compiler ahead of the base's unrelated `go` board
game, Node 24, FFmpeg/ffprobe, GCC and pinned Chromium 156. The base lacks sudo, but already
has the required browser libraries. The repo setup helper now supports this case and requires
real launch/content checks of both pinned Chromium and its headless shell; missing libraries
still fail, without browser substitution, suppressed validation or privilege bypass.

The setup task passed the complete verifier at fc3ace2, then fast-forwarded to e8e315d and
passed the exact corrected setup command. No Go/web product or test source changed between
those commits. Publication was followed by a distinct task restoring the compiler, npm deps,
browsers and caches at e8e315d, matching GitHub main. Its exact verifier passed UI/app/e2e type
checks, Go vet/tests/race, embedded-server build and all 38 browser journeys with zero browser
failures/skips/retries. Because most Go results were initially cached, it also ran uncached
ordinary Go and race suites; both exited zero. Each suite recorded 231 tests/subtests,
zero failures/skips, 26 test-bearing packages and six packages without tests. Results are
retained at /workspace/arc-cloud-evidence/FRESH-TASK.md in the cloud chat and on #157.

The normal cloud command-permission review was required for browser/proxy access; environment
network policy and global security protections were retained. GitHub independently passed the
exact Linux setup/verification recipe for e8e315d in run 39847970913 and its full release/image
gates in run 39847970870. See [CODEX-CLOUD](CODEX-CLOUD.md) for the private chat and rebuild steps.
Physical confirmation from the owner's second PC remains unperformed. These are developer-
environment results, not real-library, device, GPU, casting, Unraid or sustained-use acceptance.


## Rescan feedback and request responsiveness (#158, 22 October 2026)

An owner-supplied diagnostic export from e8e315d showed a repeated Series scan checking 14,627
files with zero probe attempts and zero new matches. Its general matching phase did not mean
all files were probed again. An earlier scan did probe 1,320 files, but the export does not
explain why those needed reading. The repeated scan also coincided with authenticated Activity,
library and item requests taking about 31 seconds. The export does not identify the exact SQL
operation/session age causing that stall; no production database or media was opened.

The discovery phase now says "Checking files and titles". Finished Activity summaries separate
files checked, file details actually read and titles identified. Real probing retains its own
phase and is still restricted to pending/new/changed files by the scanner. A generated-series
browser regression imports three files, rescans, verifies zero probing/new matching, and reads
the zero-read summary from the actual Activity popover.

Authentication previously waited synchronously to update last_seen_at after five minutes. A
held temporary writer reproduced the resulting Activity stall. Authorization still reads and
checks the current session/profile on every request; only last-used bookkeeping is asynchronous.
One store-owned worker coalesces duplicate sessions, caps pending work at 128, and bounds each
write wait to two seconds. Captured request time cannot overwrite a newer timestamp. Updates
are best-effort: full/failed queues retry on a later request while stored activity is old;
shutdown cancels pending work rather than delaying browsing or accumulating request goroutines.
Deleted sessions cannot be recreated by activity updates. This adds no user-facing setting.

TV scans run cached-episode orphan cleanup for each series. Its ordering_entries.episode_id and
user_item_data.item_id lookups lacked indexes beginning with those columns. Migration 0046 adds
both, without changing cleanup's rules. Before implementation, a temporary actual-Arc-schema/
modernc-driver fixture of 300 series, 15,000 cached episodes without files and 60,000 ordering
entries took 38.024 seconds to clean up; candidate indexes reduced that query workload to
99.066 milliseconds, retaining all episodes. Setup time is excluded. This is a synthetic query
measurement, not an end-to-end scan, real-library or Unraid speed claim.

Targeted regressions pass for old-session Activity during a held writer, persisted bookkeeping,
invalid/revoked session denial, queue coalescing/capacity/shutdown, queued-update revocation,
indexed query plans, reference-safe cleanup and genuine schema-45 upgrade/safety-backup/online
restore with preserved edited title. Older-schema and restoration checks apply migration 0046
through the normal migration path. The unchanged cloud verifier exited zero: production/app
TypeScript, e2e TypeScript, Go vet/tests/race, embedded-server build and all 39 Chromium journeys
passed without browser failures/skips/retries (4.2 minutes). Final queue hardening and request-
cancellation regressions passed race checks separately. Final Go vet/e2e TypeScript and both
focused rescan/profile-access browser journeys pass (20.1 seconds) after the last safeguards.
Desktop and 390px Activity screenshots were inspected.
The private diagnostic export is not committed or attached to GitHub.

Actual deployment responsiveness and real-library/device/sustained-use acceptance (#134–#138)
remain open. Rollback across the new schema needs its compatible pre-upgrade backup, as usual.


## Title-page proportions and long descriptions (#160, 22 October 2026)

The owner approved rebalancing movie and series headers after an isolated desktop review.
Previously, 208px movie and 176px series posters aligned with the bottom of tall information
blocks. Long movie descriptions could push Play below the first screen; series descriptions
were permanently clamped. Both detail pages now use a shared top-aligned poster/info grid:
240px posters at the smaller desktop breakpoint, 256px on wider desktops, smaller detail-title
typography, and a wider information column. At wide desktop sizes, synopsis and credits use
separate columns while the synopsis retains a comfortable maximum line length. Landscape
home-video artwork retains its aspect; Home hero presentation is independent.

Play/Resume and existing edition/action controls precede optional text. Movie and series
synopses share a four-line preview and keyboard-operable Show more/Show less, linked to their
controlled paragraph with aria-controls/aria-expanded. The expansion control appears only
when the content overflows; width/font changes are observed. Profiles, playback behavior,
episode-order controls and the owner's chosen cast-before-episodes layout are retained.

Focused real-app regressions import generated clips through local metadata fixtures, persist
long titles/descriptions into temporary stores, and verify viewport-visible Play, consistent
top-aligned poster frames, no horizontal document overflow and keyboard expansion/collapse at
1366x768, 1920x900 and 390x900. Both pass (15.8 seconds). Additional visual review uses explicitly
synthetic artwork/cast/metadata at 1366, 1440 and 1920 widths; desktop and phone screenshots were
inspected. These are isolated layout/interaction checks, not owner-library, physical-device,
provider-artwork-quality or sustained-use acceptance. No owner settings, database or media used.

The unchanged cloud verifier exited zero: production/app/e2e TypeScript, full Go vet/tests/race,
embedded-server build and all 41 Chromium journeys passed (4.3 minutes, zero browser failures,
skips or retries). That change was scoped to the title-header adjustment. The later approved Settings work is
recorded under #161 below.


## Release pipeline scheduling (22 October 2026)

The owner reported approximately 12 minutes before an updated private image was available
for server testing. Completed run 39859383042 took 12m34s: Linux checks took 7m41s,
container acceptance ran alongside them (4m19s), then release preparation/publication added
4m46s. The workflow now prepares the amd64/arm64 image concurrently with verification,
uploading it by digest without moving release tags. A final job promotes that exact digest
to `latest` and the commit tag only after Linux/Windows checks, Linux race/browser checks,
and container installation/persistence/upgrade/rollback checks all succeed. Failed checks
leave the existing release tags untouched. The separate container acceptance image and all
checks remain in place; this change does not claim artifact identity between that smoke-test
image and the separately prepared multi-platform image.

The older PR run 39859035196 stalled in FFmpeg installation for over 30 minutes, before
Go/browser verification; it was not a Docker build or a publishing run. New runs bound
FFmpeg and browser installation to five minutes, add apt network timeouts/retries, and bound
the check job to 30 minutes. Installation failures remain failures, not skipped acceptance.
Local validation passed YAML parsing, dependency/PR gating assertions, unchanged verification
coverage comparison, shell syntax checks and `git diff --check`. Hosted Buildx/GHCR behavior,
full CI results and actual timing improvement require the new GitHub run; no local registry
publication or owner-server access was performed. Digest-only uploads from failed runs may
remain untagged in the private registry; they are never selected by `latest`.


## Settings usability and lower title headers (#161, 22 October 2026)

The owner approved the full Settings review and the further local title-spacing revision.
Five stable query-addressable categories remain. Libraries & metadata now includes imports
and notifications; Playback & transcoding distinguishes intro/credit detection and subtitle
download services from personal auto-skip/language/style preferences, with optimized versions
last. Maintenance & diagnostics contains scheduling, diagnostics and direct recovery links.
Settings defaults, stored field names and admin/profile access rules are unchanged.

Persistent desktop links include subsection anchors; smaller viewports use a sticky native
section selector. Links support normal keyboard/history behavior, group changes return to
the top, and existing section query URLs remain valid. Form content keeps a readable width.
Dropdowns have descriptive accessible names, maintenance start/end are distinct, and phone
switches remain beside their labels. Copy explains instant-save controls versus explicit Save.

Automatic profile/server saves show saving/saved/error feedback beside the edited row, with
local retry using the failed patch. Existing specialized text/key editors retain local error
handling, now announced as alerts; upload-speed, pre-roll and remote controls have local
mutation feedback rather than relying on a distant page-wide error. Rejected pre-roll/upload
promises are handled. No credential values are included in feedback routing (only field names).

Movie/series headers start at 224px on desktop (160px below the previous committed position),
with a smaller shift at tablet sizes and 96px at phone sizes. The gap before Cast is tightened
while preserving the large aligned posters, prominent playback and expandable descriptions.

The unchanged cloud verifier exited 0: production/app/e2e TypeScript, Go vet/tests/race,
embedded build and all 43 Chromium journeys pass (4.5m; zero browser failures/skips/retries).
Unchanged Go packages used the prepared caches. Two new real-app journeys cover category
placement, accessible dropdown names, keyboard links/history, subsection/sticky navigation,
phone switch alignment, persisted selection and recovery links; delayed/failed server saves,
local feedback/retry/persistence and rejected pre-roll/upload writes without page errors.
Existing profile-save and mobile-navigation regressions were updated to the approved UI.
Both movie/series journeys retain desktop/phone visible-Play and description expansion checks.

Final visual captures use fresh temporary stores, generated clips, local provider fixtures and
explicitly synthetic title artwork/metadata. Desktop and phone Settings, light/dark spot checks,
and title pages at 1366/1440/1920 were inspected. Reports/logs/screenshots are retained under
/workspace/arc-cloud-evidence/settings-polish in the cloud task. Historical fresh-task evidence
is intact. No owner database/settings/media, production keys, Unraid or persistent server used.
Actual owner deployment, physical devices and sustained-use acceptance remain open on #134–#138;
GitHub container/publication checks are separate from these local results.


## Source HDR and audio format labels (#163, 23 October 2026)

The owner approved evidence-based HDR and audio labels after a read-only investigation.
Movie headers/file details, on-demand series episode file details and player source-track/
version menus now display Dolby Vision, HDR10+, HDR10/PQ, HLG and codec/profile-aware audio,
including TrueHD, Dolby Digital+, DTS-HD and reported Atmos/DTS:X. File/version isolation
prevents combining different source formats; multi-version editions do not claim one source
format in the header. No series-wide HDR claim is inferred from an episode. Source labels
remain separate from actual playback output and unchanged compatibility/tone-mapping logic.

Migration 0047 adds source evidence/status, channel layout and format-probe version. New/
changed candidate videos receive a bounded frame sample alongside structured stream metadata.
Existing successful candidates enrich at most 32 per library scan, only for files seen during
that scan; compact stream evidence and sampling share a four-second budget. Failed optional
attempts are remembered without losing duration, chapters or basic metadata. Completed attempts
are skipped on unchanged scans. Revision/concurrent-probe checks reject stale enrichment.
[Source format contract](SOURCE-FORMATS.md) records packet/time/output limits, gradual backfill,
unknown/negative sampling semantics and remaining device/profile limits.

Portable fake-subprocess regressions cover structured DV flags/profile, independent hybrid
flags, stream-index isolation, preserved audio profiles/layouts, failed/cancelled optional
probes and output limits (including io.Copy). Actual schema-46 records upgrade without losing
basic probes, saved audio profiles or chapters. Store/scanner checks cover positive evidence
round trips, capped/seen-file batches, one-time attempts and protection from concurrent normal
probes/changed files. API regression retains the original tone-mapping HDR contract while
exposing richer source labels. Existing scanner/watch-state/recovery coverage remains in place.

Thirty final actual-file metadata assertions passed: fifteen cases through both the normal
Arc Prober and the existing-file enrichment path. Small public regression HEVC bitstreams
from quietvoid/dovi_tool and quietvoid/hdr10plus_tool (Git blob hashes verified) provide encoded
DV/HDR10+ positives in raw/MP4/MKV forms; stripped/static-only and delayed-metadata controls,
plus generated PQ/HLG/SDR/misleading-name files, provide the other cases. These files were not
committed or drawn from owner media. Cloud evidence under arc-cloud-evidence/hdr-validation
includes provenance.json and ARC-FINAL-FIXTURES.log; reproduction scripts remain outside Git.

Two new browser journeys use generated imports and explicitly injected source metadata to
check movie edition isolation, multi-version headers, generic TrueHD versus TrueHD Atmos,
hybrid DV+HDR10+ labels at 1440/390px, no document overflow, and episode file details with
HLG/Dolby Digital+ Atmos/DTS:X and a misleading-title negative control. Desktop/phone captures
were inspected. Browser injection proves presentation; actual-file probing and store tests
supply separate detection/persistence evidence, not a physical HDR/Atmos playback claim.

The first full verifier passed Go vet/tests/race and the embedded build, but finished with
44 browser passes and one failure in the pre-existing Settings inline-save viewport check.
The screenshot put Save at the viewport bottom and its local error immediately below it.
The test now centres the upload control before saving; its error-content, viewport and retry
assertions remain intact. Settings product code is unchanged. Original failure log, screenshot,
trace and exit status are retained in hdr-validation/first-verification. The final unchanged cloud
verifier exited 0: production/app/e2e TypeScript, Go vet/tests/race, embedded-server build
and all 45 browser journeys passed (4.6 minutes for the browser phase; zero browser failures,
skips or retries). Final actual-file probe/enrichment checks passed 30/30. GitHub CI/image
publication is separate evidence and was not assumed complete from these cloud checks.

No owner database/settings/media, production keys, Unraid or persistent server was used.
These results do not establish all DV profiles/authoring variants, actual Atmos-positive
bitstream detection, HDR display appearance, receiver pass-through, network-drive throughput,
production-container binary behaviour or sustained-use acceptance. #134–#138 remain open.


## Background analysis and scheduling (#164, 23 October 2026)

The owner approved the written [background-work plan](BACKGROUND-WORK-PLAN.md) and its
implementation after comparing Arc with Plex's Library/Scheduled Tasks controls. Baseline
e5f36e0. Discovery remains independent, with 15/30-minute and 2-hour choices added to the
existing intervals. Canonical minute settings retain legacy hour API requests, including
fractional hours for 15/30-minute round trips, and preserve explicit Off/restored values.

Existing source-format enrichment now drains independently of scans during maintenance or
Run now. A 32-row cursor page is no longer a run limit. Previews and local intro/credits
detection share Off / During maintenance / When added and during maintenance. The persisted
policy epoch distinguishes existing from newly probed media across restarts. Existing
per-library preview/marker scopes remain. Maintenance & diagnostics provides task switches,
status and per-task Run now; Activity displays pending/paused work without treating it as
running foreground activity. Basic new/changed-file probing remains automatic.

Migration 0048 stores revision-specific queued/running/paused/completed/failed work, cascades
jobs when their media records are removed, and versions explicit marker resets. Missing
preview artifacts can reopen completed work. Optional format/preview/local-detection jobs
share fair admission, yield to playback/discovery/refresh, and cancel subprocess leases when
policy closes or restores invalidate them. Failures back off for an hour; Run now can retry
them. The override coalesces and applies to the requested backlog rather than permanently
changing schedules. Metadata refresh checks live policy, including discovery queued behind
the same library slot. Backups finish an admitted database copy safely; manual/safety backups
remain independent. Unrelated settings saves do not start backups or metadata refresh.

Eleven scheduler regressions exercise controlled windows/overnight/added policies, serial/fair
admission, foreground/Off/window/restore cancellation, completed/interrupted restart, changed
revisions, retry delay, marker reset, missing preview artifacts, obsolete waiters and waking an
idle server when maintenance opens. A saved-catalog fixture contains 70 pending files (one
intentionally unavailable): one pass attempts all 70 across cursor pages, completes 69 and
retains the failure; an immediate second pass performs zero repeats. Its analysis callback
uses isolated store updates, not a real NAS probe throughput measurement.

Actual HTTP-router progress requests cancel an optional lease on playing and permit a new
lease on paused, tested under race detection. A blocked local TLS provider regression proves
a queued discovery request cancels maintenance holding the same library slot and then finds
new synthetic media. This added regression initially caught a context-interface capture race
in the maintenance monitor; the monitor now captures an immutable context. The final complete
scanner race suite passes (52.4s), as do final task-access/playback/legacy-hour API regressions
under race detection (7.5s) and the final scheduler race suite (27.9s). The original race log
is retained separately in cloud evidence; it was not accepted as a passing run.

The unchanged cloud verifier exited 0: production/app/e2e TypeScript, Go vet/tests/race,
embedded-server build and all 47 browser journeys passed (browser phase 5.3m, zero failures,
skips or retries). The first attempt stopped at the generated API reference check; docs/API.md
was regenerated for the new admin endpoints before the complete rerun. Final compatibility,
path/revision, admission and same-library priority hardening received additional targeted/race
checks; all-package ordinary Go tests, Go vet and the embedded build passed again before
publication. These final checks supplement the full verifier rather than replacing any gate.

Four focused browser journeys passed for task modes/actions/retry, 15-minute interval
persistence, Activity paused status, existing global scan controls and restored settings.
The new task-status Activity presentation uses explicitly injected status data; scheduler and
router/scanner tests provide separate backend evidence. A 390px Maintenance page capture was
inspected, and its no-overflow check passes. Full browser checks retain existing desktop/phone
Settings, playback, scanning, backup/restore and access regressions. Cloud evidence is retained
under /workspace/arc-cloud-evidence/scheduling, including first failures and final logs.

No owner database/settings/media, production keys, Unraid or persistent server used. Prepared
Go 1.27.0, npm dependencies and pinned Chromium were used without new tool installation.
Real server-local time/TZ deployment, NAS CPU/I/O impact, physical devices/hardware, actual
production-container operation and sustained-use acceptance remain open on #134–#138. Hosted
GitHub checks/image publication remain separate from this cloud verification.
