# Stability and release readiness

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
skips or retries). The other Settings-review recommendations (near-control errors, accessible labels and navigation polish) remain proposals;
this change is scoped to the title-header adjustment the owner approved.


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
