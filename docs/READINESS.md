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

The broader UI assessment is recorded in [UI-REVIEW.md](UI-REVIEW.md) and #148. Four focused
diagnostic journeys reproduce #149–#152 and the custom-row full-results gap; they are separate
from the normal regression suite. A diagnostic pass establishes the observed defect/gap,
not a fix. No product code changed in this assessment. Desktop/narrow screenshots and source
review cover the requested areas with the limits documented there; real devices and sustained
use remain unverified. Follow `web/review/README.md` to reproduce with isolated data.

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
