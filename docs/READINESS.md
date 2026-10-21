# Stability and release readiness

Target: an alpha release other people can test. Implemented features and verified deployments
are separate: a closed feature milestone does not establish that every device works.

Track this work in milestone **21. Stability and release readiness**, issues #130–#138.
Use isolated databases and copies of data for recovery tests. Never modify the owner's media.

## Acceptance checklist

| Area | Evidence required | Status |
| --- | --- | --- |
| Access controls (#130) | Regressions for forwarded sign-in, HTTPS cookies, cast revocation | Implemented; local regression tests pass |
| Conversion admission (#131) | Simultaneous starts respect limits; remux allowed; stopped/failed starts release capacity | Implemented; local regression tests pass |
| Build gates (#132) | Linux/Windows checks and Linux race/media tests pass before Docker publication | Passed for b5d8e39; see verification below |
| Browser journeys (#133) | Setup, profiles, library preview/import, fix match, order changes, playback/resume and denied access exercised in a browser | Six journeys pass locally and in Linux CI for 49ac1f5 |
| Player-close progress (#140) | Real video seeks beyond the resume threshold, closes, reloads and resumes at the saved point | Fixed; browser regression passes locally and in Linux CI |
| Library correctness (#134) | Representative real library; rescan/edit/provider/order history preserved; missing drive recovery; large-library measurements | Synthetic rescan/order/edition/local-asset regressions pass; broader and real-library validation pending |
| Playback matrix (#135) | Actual devices, codecs, seeking/audio/subtitles, HDR, encoders, casting | Pending |
| Recovery (#136) | Old-schema upgrade, backup restore, unavailable providers/drives, interrupted scan/conversion, unwritable/full storage | Isolated upgrade/restore/outage/storage/shutdown checks pass; full-disk, abrupt-exit and deployment recovery pending |
| Sustained use (#137) | 7–14 days on Unraid; start/end resource measurements, problems recorded | Pending |
| Packaging (#138) | Fresh Docker install, persistence/permissions, upgrade/rollback, attribution and distribution decisions | Pending |

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
than media decoding. Both tests pass on Windows. Real-library and large-library measurements,
anime/date naming and provider-switch preservation still need broader validation; #134 stays open.
