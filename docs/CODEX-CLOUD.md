# Arc development in Codex Cloud

The owner approved using Codex Cloud for cross-PC development on 22 October 2026,
with GitHub remaining the source of truth. The private `arc-media-server` environment
is now published, and a separate task restored it and passed verification. The
instructions below are retained for rebuilding or updating the environment.
Setup and cross-device acceptance: [GitHub #157](https://github.com/JonzaUK/arc-media-server/issues/157).

Library-work handoff (#183/#184/#196/#187, 25 October): confirmed library repair actions
and safe Stop now live in Manage libraries/Activity, sharing scan admission and preserving
identity/presence/history. Stop cancels scanner-owned follow-ups/retry timers and uses run IDs
independent across server lifetimes. Server clock/window values are server-derived, including
DST and all-day/overnight cases. Title Refresh moved into More; queue/playlist labels/icons
are distinct. #188 navigation controls are reviewed/deferred pending real owner task evidence.
First hosted 8594200 passed 76/77 journeys and blocked publication: the older refresh-recovery
journey still targeted a Refresh button. It now opens More → Refresh metadata, retaining
all failure/Retry/watchlist assertions and runtime/CI behavior. Final hosted outcome is
recorded on the issues and in the task evidence folder. Contract:
[LIBRARY-WORK-CONTROLS.md](LIBRARY-WORK-CONTROLS.md); actual results and limitations
in READINESS and the individual GitHub issues/#162. No migration or dependency installation.

Preplay handoff (#186, 25 October): movie/series-next/episode Playback options offers
version/audio/subtitle choices before Play/Resume. Shared server resolution prevents drift
from normal playback; no stream/conversion begins in the dialog. Choices bind to the source,
reset on version change, survive initial retry and preserve approved language learning only
after successful start. Ordinary Play and optimized eligibility remain. Contract:
[PREPLAY-OPTIONS.md](PREPLAY-OPTIONS.md); READINESS and #186/#162 record actual checks,
injected-fixture boundaries, hosted publication and remaining physical-device limits.

Clarity handoff (#181/#182/#185, 25 October): movie/show library controls have explicit
labels and associated help without a layout redesign. Backups shows live automatic-backup
policy, server-local maintenance hours, last saved successful backup, manual/safety availability
and a link to existing settings; unavailable policy offers Retry. Bulk selection explicitly
names loaded titles and does not select newly loaded pages implicitly. No new API, schema,
scheduling policy or server-wide selection. READINESS records isolated results; the individual
issues and #162 hold hosted validation/publication and the next review agenda.

Recovery handoff (#176–#180): query failures show local Retry instead of endless/empty states;
cached news/playlists/devices/optimized copies/notifications remain visible with a stale notice.
Dependent playlist/webhook actions explain unavailable prerequisites. Failed news refresh is not
marked seen; per-device errors stay independent and successful revocation is retained if list
refresh fails. Episode playback uses a native button separate from row actions. No backend or
schema changes. First hosted b0fc9dc passed 60/62 browser journeys and blocked publication on
ambiguous older /^Play S/ queue-test selectors. The follow-up narrows those two selectors to
season/episode codes without changing assertions or runtime labels. READINESS and individual
issues record actual checks and hosted outcome.

Audit handoff (#172/#174/#175): optimized selection respects measured average total bitrate
and effective quality/remote/upload-share limits; migration 0051 preserves existing copies with
unknown measurements for unlimited playback. Limited playback falls back conservatively. Search
keeps each query's actionable results separate and exposes pending feedback. Phone filters wrap,
active values stay reachable and empty states offer Clear filters without losing sorting.
See OPTIMIZED-COPIES, READINESS and each issue for actual results and hosted publication.

Optimized audio handoff (#171): completed copies record exact mapped source tracks; playback
checks profile and explicit selections and falls back for missing alternate tracks. The player
identifies the optimized copy and its AAC stereo output. Migration 0050 requires explicit
regeneration for older completed copies with unknown audio provenance, retaining removable files
and avoiding bulk work. See OPTIMIZED-COPIES, READINESS and #171 for validation. #172 follow-up is recorded above.

Optimized-copy handoff (#170): migration 0049 binds copies to source revisions and attempt IDs;
playback/streaming reject stale or missing artifacts, including before the next scan. Old
workers cannot overwrite a newer request. Restart/cleanup and missing-copy regeneration are
covered. Older completed copies need Optimize again because provenance cannot be inferred;
there is no automatic bulk regeneration or forced source reprobe. See OPTIMIZED-COPIES.md,
READINESS and #170 for evidence/limits. Audio follow-up is recorded above; #172 bitrate follow-up is recorded above.

Audit review queue: [#162](https://github.com/JonzaUK/arc-media-server/issues/162) indexes all
36 Arc/Plex findings, with independent review/implementation-or-close decisions. Start with
CI #173, then optimized-copy correctness #170–#172. The #173 TestEach correction uses controlled
worker gates instead of assuming a failing callback runs before later indices. It preserves
the runtime helper and CI gates. Commit 6886f1f passed run 40023241460, all 50 browser journeys,
container acceptance and publication; #173 is closed. See READINESS and the issue for evidence.

Navigation handoff (#166, 23 October): server management lives within Settings, including
the existing Dashboard/Insights/Health/Libraries/Profiles/Backups URLs. Activity links to
Dashboard. Plan: [SETTINGS-NAVIGATION-PLAN.md](SETTINGS-NAVIGATION-PLAN.md); focused
desktop/phone/profile-boundary verification and limits are recorded in READINESS.

Activity handoff (#167): scans expose concurrent title/file operations and real stages;
background format analysis is separate, and completed scans distinguish reused details and
inspection/metadata problems. See [SCAN-ACTIVITY-PLAN.md](SCAN-ACTIVITY-PLAN.md) and READINESS
for the implementation, actual fixture results and limits. No owner data was used.

Library management (#168): individual Scan and Scan all libraries now live in Settings →
Manage libraries. Running scans are left alone, request failures show per-row Retry, and
Activity carries detailed progress. Focused desktop/phone results are in READINESS.

Rescan follow-up (#169): normal scans reuse established metadata/links; age refresh and old
unmatched retries run in maintenance. New/pending and explicit changes still match; local
sidecars still update. Plan: [INCREMENTAL-RESCAN-PLAN.md](INCREMENTAL-RESCAN-PLAN.md). See READINESS
for provider-count, writer-queue, history/recovery, race and browser evidence. No migration.

## Create the environment

In ChatGPT/Codex choose **Work in > Cloud > Select environment > Create environment**,
or **Settings > Codex Cloud > Environments**. Select only
`JonzaUK/arc-media-server` and keep the environment private (**Only me**).
Sign in and authorize the repository connection yourself if prompted.

Give the environment setup conversation this request:

> Prepare this private repository for Arc development. Read AGENTS.md, CLAUDE.md,
> ROADMAP.md, docs/READINESS.md and docs/CODEX-CLOUD.md. Install the Go version in
> go.mod (currently 1.27.0), Node 24, npm, FFmpeg/ffprobe and a C compiler for Go's
> race detector. Run `bash tools/codex-cloud/setup.sh`, then
> `bash tools/codex-cloud/verify.sh`. Fix environment setup failures without changing
> product behaviour or weakening checks. Use only temporary stores, generated clips
> and local fixture providers. Record actual results and any unsupported checks.
> No real provider keys, owner database/settings/media, Unraid access or persistent
> server is needed. Report the configuration and verification before publication.

The setup command installs locked npm/Go dependencies and pinned Chromium, then builds
the embedded UI. When root or noninteractive sudo is available it installs Chromium's
OS dependencies too. A non-root cloud base can instead use its existing libraries;
both pinned Chromium and its headless shell must pass real launch/content checks.
Missing libraries remain a setup failure requiring supported environment provisioning;
never substitute the system browser or skip checks. Language runtimes, FFmpeg and a
compiler must already be installed by the environment preparation. The verification command
runs Go vet/tests/race, a disposable server build and all browser journeys. It does
not start the ordinary `go run ./cmd/ams` server or open the owner's `data` folder.

The **Codex Cloud preflight** GitHub workflow runs these exact commands on a fresh
Ubuntu runner whenever the scripts change, or when manually dispatched. Its result
validates this Linux recipe; it does not create or verify a Codex account environment.
The existing Docker release gates remain independent.

Both GitHub workflows install media/compiler prerequisites with
`bash tools/ci/install-media-deps.sh`, bounded to five minutes. This GitHub-runner-only
helper selects official Ubuntu HTTPS archives instead of the slow Azure HTTP mirror,
retains signed package checks and omits optional recommended packages (#165).
Published cloud tasks continue to use their saved compiler/media prerequisites; do not
run this helper to repair missing restored state.

Allow package installation during setup. Required sources include npm, the Go
module proxy/checksum database and toolchain downloads, Linux package mirrors and
Playwright's browser download hosts. Let setup report missing destinations rather
than allowing unrestricted networking. GitHub access is needed to fetch/push the
selected repository. No TMDB/TVDB or production secrets are needed for these tests.

Review actual setup results, save, and **Publish**. Publication is an account action;
a Git push alone does not create or publish a Codex environment. Start a fresh task
from the published environment and confirm the same checks work there. Record its
environment name, starting commit and results in the handoff or GitHub issue.

Do not create a broad reusable service that automatically runs every time this
environment starts: the test harness manages its own isolated servers on ports
18199/18200, temp databases, generated videos and cleanup. Docker installation and
actual GPU/TV/Unraid acceptance stay with GitHub CI or real deployment checks.

## Continue from either PC

Reopen the **same cloud task** to continue its conversation and saved work. A new
task starts separate work; it does not inherit every prior chat. Task state is not
a replacement for source control. Keep completed work and a concise handoff in Git.

Start a task with:

> Continue Arc's milestone 21 stability and readiness work. Read the repository
> instructions and current notes first; fetch GitHub and inspect the task checkout
> before updating it. Do not discard uncommitted work. The owner prioritizes
> Plex-like simplicity and fixes to actual Arc behaviour. Scanning controls, the
> everyday UI/queue review, series-folder serialization, queued SQLite writers,
> retained diagnostics and poster-only saves are implemented; inspect newer commits
> before assuming they are still pending. Real-library/device/sustained-use evidence
> remains open on #134–#138. Confirm the present baseline and outstanding issue evidence
> before choosing more work. Use isolated test data. Commit and push completed work,
> update shared notes and GitHub, and report checks with their limitations.

The owner updated the workflow on 22 October: authorized changes can be committed
and pushed directly to main after the relevant checks pass; a PR is no longer required.
Use a separate working branch when useful, then push the verified commit without force.
Main publishes Docker images after its CI gates. Do not run a local and a cloud
agent on the same branch at the same time. Local PCs can pull completed changes with `git pull --ff-only` when
their checkout is clean; never force/reset/stash unknown local work automatically.

## Saved context and present baseline

- `AGENTS.md` points agents to shared instructions; `CLAUDE.md` records architecture
  and current state; ROADMAP/READINESS and GitHub hold plan and verification evidence.
- Before cloud preparation, local and GitHub main matched `a0de3eb`. All release
  gates and image publication passed in Actions run 39816525377. Local production UI,
  Go vet and Go tests also passed after syncing nine laptop commits.
- Browser coverage at cloud acceptance had 38 journeys; subsequent fixes add coverage.
  Read the current test count and run the checks rather than reusing historical results.
- The owner completed account sign-in and GitHub authorization. The environment draft
  `arc-media-server` contains only this repository and is private (Only me). The owner
  explicitly approved three additional public dependency hosts: storage.googleapis.com,
  cdn.playwright.dev and playwright.download.prss.microsoft.com. The package-manager
  preset stays enabled; there are no production secrets. These changes were saved.
- In the setup container, Node 24/FFmpeg/GCC were present. Go 1.27.0 was installed from
  a checksum-verified official archive under /workspace; the saved activation must
  put that compiler ahead of the unrelated system `go` board-game executable.
  Both pinned Chromium 156 binaries launched with existing libraries and no sudo.
- Publication completed and a new task restored the prepared Go compiler, npm
  dependencies, pinned browsers and caches at `e8e315d`, matching live main. The exact
  verifier passed there, including all 38 browser journeys without failures/skips/retries.
  Supplemental uncached ordinary Go and race runs both passed; each recorded
  231 tests/subtests with no failures or skips across 26 test-bearing packages (six
  packages have no tests). Exact results are in that task's
  `/workspace/arc-cloud-evidence/FRESH-TASK.md` and the shared GitHub issue.
- The ongoing chat is **Arc — stability and readiness (Cloud)**:
  [open the private cloud chat](https://chatgpt.com/local/01a16551-79a5-7234-9524-02d06e165079).
  Sign into the same account/workspace on either PC and reopen this chat for continuity.
  A physical check from the owner's other PC remains separate from the fresh-task test.
- The cloud command sandbox initially prevented the browser and GitHub proxy from
  running. The platform's normal per-command permission review allowed those operations;
  no global protection or network restriction was disabled. Use that supported review
  when needed; do not replace the pinned browser or bypass access controls.
- Independent GitHub preflight for `e8e315d` passed in
  [run 39847970913](https://github.com/JonzaUK/arc-media-server/actions/runs/39847970913).
  Linux/Windows/race/browser/container and image-publication gates also passed in
  [run 39847970870](https://github.com/JonzaUK/arc-media-server/actions/runs/39847970870).
  Neither result substitutes for actual Unraid/device/sustained-use acceptance.

Official setup and saved-state behaviour:
[Codex Cloud environments](https://learn.chatgpt.com/docs/environments/cloud-environments).
