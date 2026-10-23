# Arc Media Server (AMS): notes for AI agents

An open source media server (Plex/Jellyfin alternative) written in Go with an embedded React web
app. The owner (GitHub: JonzaUK) drives product vision; the agent does most implementation.
Read `ROADMAP.md` for the plan and `README.md` for features and the API.

## Setting up a new machine

The owner works from more than one PC (Windows). On a fresh clone:

1. Install the tools (then open a new terminal so PATH updates):
   ```
   winget install GoLang.Go Gyan.FFmpeg OpenJS.NodeJS.LTS GitHub.cli
   ```
2. Log in to GitHub and clone:
   ```
   gh auth login --web
   gh repo clone JonzaUK/arc-media-server
   cd arc-media-server
   ```
3. Start Claude Code in the folder and say "let's continue".

**Agent: when starting on a fresh clone, do this for the owner:**
- Check `go version`, `node --version`, `ffprobe -version` and `gh auth status`. If any are missing,
  give the owner the exact command above (installs need their go-ahead). If `go` or `ffprobe` aren't
  found right after installing, the shell's PATH is stale: use `C:\Program Files\Go\bin\go.exe`, and
  find ffprobe under `%LOCALAPPDATA%\Microsoft\WinGet\Packages\Gyan.FFmpeg*\...\bin`.
- Create the test library outside the repo with `go run ./tools/testmedia -out ../ams-test-media`,
  unless the owner gives real media paths.
- Build the UI (`cd web && npm install && npm run build`), then `go run ./cmd/ams`, and tell the
  owner to open http://127.0.0.1:8099. There are no config files: first-run setup in the browser
  creates their admin profile and takes the TMDB/TVDB keys (they have both; keys never go in git),
  then libraries are added in the app (the test library's `Movies` and `TV` folders). ffprobe is
  found automatically (PATH, winget folder, next to the binary). An old `ams.yaml` is imported once.
- Check `ROADMAP.md` and the open GitHub milestones (`gh issue list -R JonzaUK/arc-media-server`) to
  see what's next, then carry on (see "Where things stand").

## Where things stand

- Manage libraries scans (#168, 23 October): Settings → Manage libraries offers Scan all
  libraries and a Scan button per row. Bulk requests only idle libraries, keeps running scans
  untouched and reports partial failures beside the affected library with Retry. Pending/running
  state prevents repeated requests; Add/Edit/Remove remain available and phone actions wrap.
  Existing admin scan endpoints/policies are unchanged. Production/app/e2e TypeScript and three
  focused browser journeys pass; results and deployment limits are recorded in READINESS.
- Scan Activity (#167, 23 October): each library task now exposes bounded concurrent
  movie/series/file operations with actual matching/provider, metadata/episode-saving,
  cached-linking and inspection stages. Independent format analysis names its library/file.
  Unknown discovery totals stay indeterminate; summaries separate reused details, successful
  reads and inspection/deferred-metadata/unrecognised-file problems. Activity wraps titles and
  summaries, with extra operations/filenames expandable. No matching/probing policy change or
  migration. Affected Go/API tests, tracker/scanner/background race checks, production/app/e2e
  TypeScript and six focused browser journeys pass; plan/evidence in docs/SCAN-ACTIVITY-PLAN.md
  and READINESS. Real owner-library/device acceptance remains separate.
- Settings navigation (#166, 23 October): management pages now share a Settings rail
  grouped by Server, Libraries, Playback, People & access and Maintenance, alongside personal
  preferences. The desktop Manage group and phone Manage menu are replaced by Settings;
  Activity offers an admin-only View Dashboard link that closes the popover. Existing URLs,
  settings section links and health counts remain. Ordinary profiles retain preferences only
  and are redirected before admin pages mount. Production/app/e2e TypeScript and five focused
  browser journeys pass; see docs/SETTINGS-NAVIGATION-PLAN.md and READINESS for scope/evidence.
- CI media prerequisites (#165, 23 October): run 39941884612 timed out downloading FFmpeg
  dependencies from the runner's Azure HTTP mirror before Linux tests started. Both workflows
  now use tools/ci/install-media-deps.sh to select official Ubuntu HTTPS archives, omit optional
  recommended packages and verify ffmpeg/ffprobe. Signed apt verification and all release gates
  remain. Eight isolated helper/workflow checks pass; hosted verification is separate evidence.
- Background scheduling (#164, 23 October): discovery now offers 15/30-minute and 2-hour
  intervals alongside the existing choices. A separate revision-aware worker drains existing
  format analysis during maintenance or Run now; database pages are not a run limit. Previews
  and local intro/credits detection use Off / During maintenance / When added and during
  maintenance. Existing-file work waits for the shared server-local window. Task switches,
  status and Run now live in Maintenance & diagnostics, with pending work also in Activity.
  Optional analysis/preview/detection jobs share fair admission and yield to playback/scans;
  unfinished jobs resume, completed revisions persist and failures back off. Metadata refresh
  respects live policy; manual/safety backups remain independent. Migration 0048 stores jobs
  and the policy epoch. Legacy settings, restores and missing artifacts are covered by isolated
  checks; see docs/BACKGROUND-WORK-PLAN.md and READINESS.md for the exact contract and evidence.

- Source format labels (#163, 23 October): Movie/file details and series episode file details
  show Dolby Vision, HDR10+, HDR10/PQ, HLG and codec/profile-aware audio (TrueHD, Dolby Digital+,
  DTS-HD, Atmos/DTS:X when actually reported). Player source track/version menus share these
  labels. Multiple versions never contribute a mixed HDR/audio badge; a whole series is not
  assigned one episode's formats. These describe source metadata, not current device output.
  Migration 0047 adds evidence/status and a one-time format-probe version. New/changed candidate
  videos get a bounded FFprobe frame sample; existing-file enrichment now follows #164
  background scheduling rather than the original 32-files-per-scan backfill. Optional failure is remembered without losing basic metadata.
  Unchanged scans skip completed attempts. See docs/SOURCE-FORMATS.md for limits and backfill.

- Settings and title spacing (#161, 22 October): Settings retains five groups, with imports/
  notifications in Libraries & metadata, renamed Playback & transcoding and Maintenance &
  diagnostics, clearer subtitle/intro scopes and optimized versions last in playback.
  Sticky desktop section/subsection links and a compact sticky selector replace wrapping tabs.
  Save feedback/retry is local to changed controls; dropdowns are named and phone toggles stay
  beside their labels. The approved movie/series headers sit lower with a tighter Cast gap.
  Isolated verification evidence and deployment limits are recorded in docs/READINESS.md.

- Release scheduling (22 October): Docker image preparation now runs alongside verification.
  Only the final gated job tags the prepared digest as latest/commit; all existing checks remain.
  FFmpeg/browser installation and jobs have time limits to prevent indefinite runner stalls.
  Local workflow structure/shell checks pass; hosted validation/timing is separate evidence in
  docs/READINESS.md. Container acceptance still builds its own isolated image.

- Done: milestone 1 (movie core), milestone 2 (TV shows + display orderings), and the first version
  of the web app (pulled forward from milestone 5). The owner loves the design and the orderings feature.
- Milestone 3 done (Oct 2026): artwork cache, backups + restore, field locks + Edit Details, Fix
  Match with explanations, Library Health page, folder watching, and libraries managed in the app
  with a preview before adding (the owner chose in-app libraries over ams.yaml).
- Milestone 4 done (Oct 2026): sign-in, Plex-style profiles, Settings (no config file), watch state,
  Continue Watching/Up Next, richer metadata (age ratings by region), parental controls, empty trash.
- Milestone 5 done (Oct 2026): browse filters and sorts, local artwork/NFO, extras and trailers,
  Choose Artwork, admin Dashboard (now playing, stop, server log), Compare Orders, per-profile
  preferences (Edit Home with custom and seasonal rows, Hide Spoilers, theme), shared form styles.
- Milestone 7 done (Oct 2026): playback decision (direct/remux/transcode), HLS conversion, new player,
  subtitles (incl. OpenSubtitles), versions/editions, scrubbing previews, hardware encoding, tone mapping,
  compatibility report.
- Milestone 9 done (Oct 2026): collections (automatic from TMDB, manual, smart), playlists with a player
  queue, and search across titles, episodes, people, collections and genres with typo forgiveness,
  plus person pages.
- Milestone 10 done (Oct 2026): watch history (play_history, recorded from player progress after a
  minute of playing), admin Insights page (activity, history, storage/never watched), Year in Review
  per profile, plus clear logos on title pages and the cast moved up on series pages (#81, #82).
- After 10 (Oct 2026): home rows got Plex-style options (Continue Watching merged with Up Next,
  posters/thumbnails, show or season posters, Recently Added TV grouped by season, season posters).
- Plan re-ordered with the owner (Oct 2026), from a Plex gap review: 12 Everyday playback, 13 Profiles
  and home, 14 Library management, 15 Moving in and extras, 16 Intro and credits skipping, 17 Remote
  access and devices, 18 Arc apps and client API. Old milestones 8 and 11 became 16 and 17.
  No Jellyfin-compatible layer, ever (owner, Oct 2026): Arc has its own clients and an open API;
  #30–#33 are closed as not planned.
  Deleting media from the app was left out on purpose (principle 1).
- Since then (Oct 2026): TMDB numbering by default with per-show override (episodes re-keyed safely
  on a provider switch), editable genres, artwork and season posters in the Edit dialog, season and
  episode editing, TMDB season summaries.
- Milestone 12 done (Oct 2026): Auto-Play, preferred audio language, quality menu, remove from
  Continue Watching, chapters, Picture-in-Picture.
- Milestone 13 done (Oct 2026): library access per profile, collection/playlist home rows, admin-set
  and default Home, ratings and watchlist, More Like This and Recommended.
- Milestone 14 done (Oct 2026): show status and original language, search by other names and keywords,
  labels, Activity menu, edit many titles at once, merge and Split Apart, metadata language per library,
  nightly maintenance window, Other videos libraries, IMDb/Rotten Tomatoes/Metacritic scores. The owner
  put cross-provider metadata details on the back burner (#104-#106, #109 in Later). 
- Milestone 15 done (Oct 2026): import from Plex/Jellyfin, webhooks + New for You, theme music, pre-roll.
- Milestone 16 done (Oct 2026): skip markers from chapters, IntroDB/TheIntroDB (checked against file
  length) and Chromaprint detection across a season; Skip Intro/Recap/Credits in the player; manual editing.
- Milestone 17 done (Oct 2026): friends with invite links (#58), HTTPS on the same port + Let's Encrypt + UPnP
  remote access (#57), remote bandwidth limits, optimized versions, casting (Chromecast/AirPlay; untested on
  real devices). Milestone 18: client API done (#102: Bearer device tokens, TV link codes at /link, /api/v1,
  docs/API.md generated by tools/apidoc), web app installable; native apps (#63) still open: they can't be
  built here (need a Mac / Android Studio). 15-18 were done in one go at the owner's request.
- After 18 (Oct 2026): profile settings (Auto-Play, Skip intros, Theme music, Hide spoilers, Appearance)
  moved from the profile menu to Settings, which every profile can open (server sections admin-only).
  Each show's details come from ONE provider: TMDB, or TheTVDB if set in Edit > Details From
  (items.details_provider); never backfill (owner). Skip Intro/Credits are per library (as Plex);
  movies get credits too (chapters, TheIntroDB, overnight picture-based detection in markers/credits.go).
- Milestone 19 done (Oct 2026), from a Plex gap review: home networks (e.g. Tailscale), transcoder temp
  folder + conversions at once, Shuffle, subtitle mode/style per profile, collection mode, smart
  playlists, Dashboard resource graphs. The owner is thinking about: release packaging, native apps,
  downloads, and translating the UI (recommended react-i18next + Weblate, #122).
- Milestone 20 done (Oct 2026), Faster scanning: titles matched from names first (6 at a time), files read
  afterwards (3 at a time; Play reads an unread file on the spot), preview matches reused, progress shown.
  600 films: all matched in 43 s instead of 449 s (tools/scanbench). Prompted by the owner's Unraid.
- Milestone 21 started (21 Oct 2026): stability and alpha release readiness (#130–#138).
  Owner clarification: Arc already runs successfully in their Unraid Docker container; installation
  is not the problem. Prioritize reproducible bugs in Arc's everyday behavior over additional
  packaging/installation work. Features remain paused while stability is the focus. This report
  is real deployment evidence, not a claim of completed timed resource/device measurements.
  First changes fix forwarded home-network bypass, HTTPS cookie protection, revoked cast sessions
  and concurrent conversion admission, with regression tests. Linux/Windows checks, Linux race
  detection and media tests passed for b5d8e39 and now gate Docker publishing. See docs/READINESS.md.
  Browser journeys (#133) implemented: six Playwright Chromium tests pass locally against the real
  embedded app, fresh temporary databases, generated clips and local metadata. Linux CI passed for
  49ac1f5 (run 39635142227), including the browser suite and race checks; Windows checks also passed.
  They found #140, final resume progress lost on player close; fixed with layout cleanup.
  Run from web: `npx playwright install chromium`, then `npm run test:e2e` (Go/FFmpeg required).
  Ports 18199/18200 must be free; never point the harness at owner data. Failure reports/traces stay
  ignored. Next: library/recovery tests and real-device/server validation.
  The owner wants an eventual /docs page backed by an OpenAPI contract; docs/API.md is the current reference.
  Never mark hardware, deployment or sustained-use checks complete without actual evidence.
  Recovery follow-up: backup validation now rejects empty/gapped migration histories; regression
  coverage restores a genuine schema-3 database and checks edited title/field-lock preservation.
  Built-in backups contain SQLite only; uploads/certificates need separate preservation (see
  docs/READINESS.md). #136 remains open for the remaining failure/deployment scenarios.
  More #136 regressions now cover missing/restored synthetic media roots/files, scan cancellation,
  provider 503/retry, blocked backup/conversion storage, corrupt restore rejection, restore undo,
  FFmpeg shutdown and stale conversion output cleanup. A root replaced by a regular file is now
  treated as unavailable, and every walk error disables missing-file sweeping. Local Windows
  checks pass; physical/full-disk, abrupt-server-exit and complete deployment recovery remain open.
  #134 synthetic scanner integration checks pass for multi-episode files, specials, unknown
  episodes, repeated scans, custom display/manual placements and resume, movie editions/remakes,
  local poster/NFO and unchanged source bytes. Representative real-library/large-library and
  broader anime/provider-switch validation remain open; do not close #134 from these fixtures.
  #134 follow-up: cached anime absolute/date file linking and file-only -> TVDB -> TMDB -> TVDB
  rekey component regressions preserve IDs, field locks, resume, play history and custom placement.
  Synthetic 1,800-film grouping benchmark: 2.791 s / 104.9 MiB total allocations (Windows Ryzen
  7700X; probing pre-seeded, providers off). Full provider-switch HTTP and real-library/Unraid
  throughput/peak-memory validation are still pending; see docs/READINESS.md for exact limits.
  Further #136/#138 work: startup refuses newer/unknown database schemas; failed pre-upgrade
  safety backups abort upgrades. Online restore now reloads runtime settings (including access
  controls) without online key validation; a real-app browser regression covers the previous
  stale-settings bug. Container acceptance gate added for isolated root/non-root, read-only media,
  restore/restart/recreation persistence; Docker unavailable locally. First Linux container smoke
  and Linux/Windows checks passed for 2387a56 (run 39654467547). Cross-image upgrade/snapshot
  rollback gate passed for 31597fa (run 39655440173), pinned to previous 2a7029a image.
  More isolated readiness coverage: a killed scanner/store subprocess recovers stable IDs/edits/
  resume; SQLite capacity-limited writes are atomic and recover; full HTTP metadata sync against
  local TLS TMDB/TVDB fixtures preserves identity/locks/resume/custom order through provider
  source switches. Physical full-disk/power-loss and live-provider/real-library checks remain open.
  Provider distribution review: docs/DISTRIBUTION.md records official attribution and TVDB project
  key/PIN guidance. #64/#65 still block public release; the existing private key is not assumed
  approved for distribution. No provider agreement/registration/contact was made.
  Further restore/playback checks: old-backup restore now rebuilds derived parental ages. New
  real Chromium remux/transcode journeys exposed overlapping HLS restart/seek accumulation and
  remux keyframe-position bugs; fixed with serialized/superseded request handling and relative
  start positioning. Production web build and all nine local Chromium journeys pass without
  retries, including converted seeks with a one-conversion limit; full Go checks also pass.
  docs/SERVER-TRIAL.md is a blank operator record, not evidence that #137 has begun.
  Everyday bug fixes (#141–#143): late subtitle responses no longer override a new language/Off;
  Continue Watching filters dismissals and profile restrictions before limiting results (older
  visible resumes previously disappeared); removal safety backups are labelled correctly.
  Regressions use delayed real subtitle responses, movie/episode API fixtures and actual isolated
  library removal. Twelve browser journeys and Go regression checks cover the changes.
  UI review #144 follows the owner's requested sequence: source/flow review, GitHub candidates,
  then isolated reproduction before fixes. Six candidates reproduced: library browse stopped
  at 500 with no paging; clearing Search/history left its input/results out of sync; search
  failures kept loading; manual additions offered smart playlists; a late pre-roll lookup
  replaced the latest Play choice; bulk selection survived library navigation. Fixed with
  paged browsing, URL-aware search/error display, editable-only playlist choices, superseded
  Play handling and selection reset on route/filter changes. Added seven browser checks,
  including fourteen main pages at desktop/narrow widths (page smoke, not every dialog/action).
  See docs/READINESS.md for evidence and limits; no real-device or sustained-use claim.
  Scanning investigation #145: owner reiterates Plex-like features with simple automatic
  defaults, few choices and no tuning needed. Broader UI/feature gap review follows scanning.
  No production scanner behavior changed: added reproducible isolated measurement tools.
  A 120-film local delayed-provider/probe run took 21.97s direct vs 30.06s including completed
  Preview; 240 vs 360 provider requests (Preview's full details are fetched again on Add).
  Synthetic 1,800-movie unchanged and one-add scans both take about 2.85–2.89s in repeated runs.
  Priorities proposed on #145: retain full preview metadata, targeted/debounced folder scans,
  skip unchanged work, incremental matching, essential TV data before secondary orders and
  match-confidence checks. These are proposals, not implemented improvements or Unraid results.
  Owner approved direct background library addition (#146): choose a type/folders and Add Library;
  the existing background scanner handles discovery/matching/probing. Preview folders is optional
  under Advanced, collapsed when adding; editing still opens existing settings. A lightweight
  directory-read check complements existence/overlap validation before saving. Browser tests cover
  movie/TV addition with delayed metadata (dialog closes while scanning), invalid-folder retry and
  optional-preview errors. That UI change alone did not establish a throughput gain.
  The owner subsequently approved the full #145 recommendations, now implemented: successful
  preview response reuse, independently settled folder-scoped scans, batched presence updates
  and unchanged file-link avoidance, progressive bounded matching, essential TV data before
  secondary orders, conservative ambiguous matching and automatic provider/background retries.
  Explicit refresh bypasses preview data; language/key identities remain separate. Periodic full
  reconciliation and missing-drive safeguards remain. Only new shows use the essential fast path;
  existing episodes take complete provider mapping to retain identity/history/custom placements.
  Synthetic unchanged movie scans fell from 2.85–2.89s to 0.835–0.853s; scoped movie addition
  1.69–2.42ms; after sharing thumbnail folder reads, TV unchanged 0.135–0.265s and scoped
  61-file TV show 8.47–10.00ms. Full generated-video import remains probe-bound:
  120 movies direct 21.66s (first match 0.50s); completed-preview flow 26.08s, now 240 total
  requests instead of 360. See scanbench/READINESS for boundaries; no Plex/Unraid speed claim.
  Handoff after dd2291e (21 October): all Linux/Windows checks, Linux race detection, 22 Linux
  browser journeys, container installation/persistence/restore/upgrade/rollback and Docker
  publication passed (Actions run 39682197648). #145 is closed; the local checkout was clean
  and matched origin/main before this notes update. Owner's Unraid/data/media were untouched.
  Global scanning controls (#147, 22 October) implemented with the owner's go-ahead: server-wide
  Settings > Library scanning, automatic changes On, full scans hourly (Off/hourly/6h/12h/24h),
  Advanced partial scans On and startup scans On. This supersedes the per-library proposal.
  Values persist in existing DB settings and apply live, including restore. Manual/first-library
  scans remain independent; turning triggers off leaves current scans running. Off removes the
  periodic timer; enabling automatic changes requests one catch-up scan. No tuning knobs.
  Nightly work now refreshes known movie/show metadata through the scanner's library admission
  slot, without a media discovery walk or failure-triggered discovery retries. Activity tracks it.
  Library Health reports actual watcher failures with configured fallback advice; known roots
  are checked for return independently of the full-scan schedule. Offline-root protections remain.
  docs/SCANNING-CONTROLS.md records the contract; #80 records the Plex simplifications. Isolated
  Go checks, controlled-clock watcher regressions and a real-app browser journey cover settings,
  save errors, restart/startup Off, manual/add scans, profile protection and restore values.
  Desktop/narrow screenshots were inspected. No owner data/media or Unraid deployment was used;
  real network notification behaviour and sustained-use readiness remain unverified.
  Final local Windows checks: production UI/app/e2e TypeScript build, all 23 Chromium journeys
  without retries (4.3 minutes), go vet ./..., go test ./..., generated API and diff checks pass.
  Final watcher-only safeguards were rechecked separately. Local CGO is off; Linux race and
  container/publication checks remain separate CI evidence for this commit.
  Broader eight-area UI review #148 and its approved implementation are complete (22 October).
  Owner explicitly approved all findings/proposals, including the editable playback queue.
  #149–#152: Home distinguishes failed requests from emptiness, exposes row retries, and profile/
  display-order saves explain errors. Profiles wrap household/friend identity and actions on phones.
  Home and playlists have keyboard move buttons; reset keeps custom/collection/playlist rows.
  Home custom rows link to complete results with shared added-date/runtime/rating browse filters.
  Sort direction is reversible and remembered per profile/library on this device, with explicit
  filter URLs taking precedence. Settings groups profile/library/playback/access/maintenance;
  preferred audio language is explicit. Display-order menus explain admin/server vs individual
  scope, expose inheritance reset, and comparison retains input after a failed save.
  Player controls have 44px targets and wrap at narrow widths. Retry restores a failed video.
  Play next/Add to queue plus a visible editable queue preserve current playback while arranging
  upcoming entries. Minimize allows browsing; profile switch/sign-out flushes original-profile
  progress before leaving. The queue is client-side and ends on refresh/close, not server-persisted.
  Ratings/watchlist/watched/refresh/Continue Watching and relevant load failures expose feedback.
  A regression also caught direct ?edit=1 mounting an empty Home editor before prefs loaded;
  editor initialization now waits for preferences. Retired diagnostic checks were replaced by
  normal desired-behaviour regressions in web/e2e/ux.spec.ts (web/review/README.md explains).
  Verification and exact limits are in docs/UI-REVIEW.md and docs/READINESS.md. No owner database,
  settings/media or Unraid deployment was used. #134/#135/#137 remain open for real acceptance.
  Prior scanner CI exposed a race in the fake-clock test's shared callback counter; replaced
  asynchronous counters with atomics. This was a test race, not evidence of a production race.
  Latest UI follow-up checks: production build/app and e2e TypeScript pass; 23 existing journeys passed in the full run and all 12 final UI regressions pass together (35 total). Full Go vet/tests pass; affected package/API reference checks pass after final changes. Linux/Windows CI remains separate evidence recorded on #148.
  Compact playback also keeps the next-episode countdown mounted; its own generated-video
  regression verifies automatic advancement after minimizing. The normal suite now has 36
  journeys (23 existing + 13 UI). See #148 for the final CI result.
  Final combined local Chromium run passes all 36 journeys without retries (6.4 minutes).
  CI then exposed #153: directory-key locks did not serialize differently named folders that
  EnsureItem maps to the same series. Lock by seriesMatchKey instead; unrelated shows remain
  concurrent. The expanded 18-folder regression failed 7/10 before the fix and passes 30/30
  after it. See READINESS/#153 for final server/CI evidence. This is a production synchronization
  bug, separate from the watcher test-counter race; owner data/media was untouched.
  Final CI for 4f09795 passed Linux/Windows, Linux race/browser checks, container acceptance and
  image publication (run 39785261123); #153 is closed. The owner subsequently reported repeated
  SQLITE_BUSY errors (#154). A disposable concurrency regression reproduced the exact error
  after the ten-second writer timeout. Store writes now share one queued connection, with
  independent query-only readers. Context cancellation prevents abandoned writes applying later;
  online restore releases its raw writer before migration/age updates. No timeout increase,
  schema change, database reset, media change or owner deployment was performed. Full local Go
  vet/tests pass; final browser/CI evidence is recorded in READINESS and #154. The owner's runtime
  logs show four library scans starting together in the container, followed by SQLITE_BUSY for
  library ID 3. This fits contention but does not identify the failed SQL operation. Failed scans
  now log the cached library name/ID, phase and files seen without a new database read. An
  isolated failed-background-scan regression verifies the diagnostic fields. Deployment confirmation
  still requires the updated build on the owner's server.
  Follow-up #155/#156 (22 October): owner approved logging controls after a slow series-poster
  save. Admin Settings > Maintenance & recovery > Diagnostics now has one live Detailed logging
  switch (auto-off after 30 minutes) and a redacted ZIP download. Logs persist under data/logs
  (/config/logs), three files of at most 2MiB each, with a recent in-memory fallback on storage
  failure. Startup -debug is now temporary too. The archive contains logs and a build/system/
  aggregate DB-queue manifest, never a database/settings dump/media. Names and paths can remain.
  Known keys, credential attributes, bearer/query tokens, URL user-info and common webhook/link
  secrets are masked before normal sinks and again on export. Register secrets before jobs start.
  Slow mutations (>=2s) and HTTP 5xx responses log route/status/duration at normal level; raw URLs
  and query tokens are excluded. Container CI injects its SHA into startup/export build identity.
  Poster-only Save skips the unchanged metadata PATCH and derived-age recalculation when the
  content rating is unchanged. Save shows its current step and disables inputs while pending.
  This reduces duplicate work; the owner's exact deployed latency is not yet measured/proven.
  Full local Go vet/tests and production/app/e2e TypeScript pass. The 38-journey Chromium suite
  passes together without retries (7.4m); diagnostics retry/download/restart/admin/390px and
  poster-only request/progress/lock regressions are included. Screenshots inspected. Linux race,
  container/image and actual deployment acceptance remain separate evidence on #155/#156.
  Earlier database-lock fix 1348aed and logging 48cb242 passed all release gates and publication
  (run 39801816884); #154 closed. No owner data/settings/media or Unraid deployment was changed.
- Cloud handoff (#157, 22 October): owner approved moving cross-PC development to Codex Cloud while
  retaining GitHub. Read docs/CODEX-CLOUD.md for setup, verification and task handoff. Linux setup
  and verification scripts are in tools/codex-cloud; they require the project Go toolchain,
  Node 24, FFmpeg, a C compiler and package access. The original cloud workflow used codex/ branches and PRs;
  the owner later authorized direct verified main pushes (see current policy below). Do not
  overlap local/cloud agents on the same branch. Account
  sign-in/publication/fresh-task verification are separate from this repository preparation;
  never claim Cloud is active merely because these files were pushed.
  Owner completed sign-in and GitHub authorization; the private arc-media-server draft contains
  only Arc. The owner explicitly approved and saved the three public Go/Playwright download
  hosts listed in CODEX-CLOUD. The cloud base lacks sudo but both pinned Chromium binaries
  launch with existing libraries; setup now supports this path with mandatory launch checks.
  At that draft stage, full verification, publication and fresh-task acceptance were pending.
  Completed follow-up: arc-media-server is Published and Only me. A separate restored task at
  e8e315d confirmed the saved toolchain/dependencies/browsers/caches and matched live GitHub main.
  Its exact verifier passed all 38 browser journeys with no failures/skips/retries; supplemental
  uncached ordinary Go and race runs also passed. Each run counted 231 tests/subtests,
  no failures/skips, 26 test-bearing packages plus six with no tests. Platform per-command
  permission review was needed for browser/proxy operations; no global restrictions were disabled.
  Continue in the private chat "Arc — stability and readiness (Cloud)" linked from CODEX-CLOUD.
  Full evidence lives in /workspace/arc-cloud-evidence/FRESH-TASK.md in that chat and GitHub #157.
  e8e315d also passed cloud preflight (39847970913) and all release/publication gates (39847970870).
  Actual reopening from the owner's other PC has not been physically exercised. GitHub stays
  authoritative; keep handoffs current, follow the current push policy below, and never infer
  deployment/device readiness from this developer-environment acceptance.
- Scan responsiveness follow-up (#158): the owner saw matching/reading phases on an existing
  Series library. Investigation showed a repeat scan reused every file's probe details, while
  authenticated UI requests stalled. Activity now says Checking files and titles, with finished
  summaries separating files checked, details read and titles identified. Session last-used
  bookkeeping uses one bounded/coalescing store worker instead of blocking authentication on
  the SQLite writer; session/profile/revocation checks remain immediate reads. Migration 0046
  adds reverse lookup indexes for cached episode cleanup (orders and per-profile state).
  Isolated cleanup/upgrade/restore and busy-writer authentication regressions pass. The cloud
  verifier passes production/app/e2e TypeScript, Go vet/tests/race, embedded build and all 39
  browser journeys without retries; final queue/cancellation race checks pass separately.
  Results and limits are recorded in READINESS. PR #159 was merged as 2440fbe; actual
  owner-deployment latency confirmation remains separate. Owner data/media untouched.
- Current push policy (owner, 22 October): commit and push authorized work directly to main
  after relevant checks pass; PRs are no longer required by default. This supersedes the earlier
  PR-only cloud guidance. Preserve unrelated work, never force push, and keep GitHub handoffs current.
- Title-header balance (#160): movie and series pages now share top-aligned 240–256px desktop
  posters, wider metadata/control space, smaller detail headings and expandable synopses with
  Play/Resume above the description. Wide desktops split synopsis and credits into columns;
  phones retain the compact layout. Home hero presentation and landscape home-video artwork
  are retained. Production/app/e2e TypeScript, Go vet/tests/race, embedded build and all 41
  browser journeys pass without retries; isolated desktop/phone/keyboard evidence is in READINESS.
  Settings-review recommendations remain proposals, outside this authorized header adjustment.
- The owner has said to keep working through milestones without check-ins. For product decisions,
  pick a sensible default, build it to be easy to change, and log it as a comment on GitHub issue #80
  ("Decisions waiting for the owner"); they review those in a batch. Only stop for risky or
  irreversible things.
- Necessary user configuration belongs in the web UI; no config files (issue #66). The owner's
  priority is Plex-like simplicity: choose good automatic defaults, keep advanced choices out
  of ordinary flows, and don't expose implementation details such as scanner worker/cache knobs.
- Keep this section current at the end of each working session.

## Principles (non-negotiable)

1. **Never modify media files.** AMS only reads: no renaming, moving, tagging or writing beside them.
2. **Ease over options.** Apple-level polish and simplicity; no setting without a reason. For every new
   feature, first check how Plex does it and match its behaviour, defaults and where its settings live
   (per library, server or profile) unless there's a clear reason to differ; log any difference on #80.
3. **No plugin system.** Features are built into AMS.
4. **Storage order ≠ display order.** Episode identity is separate from how a series is displayed.
5. **Nothing silently disappears.** Unmatched or unplaced items are shown and listed, never hidden.

## Working with the owner

- Explain trade-offs in plain language with a recommendation; they're a Plex/Jellyfin power user,
  not a Go/React expert.
- Work milestone by milestone (`ROADMAP.md`). The web app was pulled forward so progress is visible:
  add UI for new features as you build them.
- Track work in GitHub: each roadmap item is an issue under a milestone in
  `JonzaUK/arc-media-server` (private). Close issues with the commit hash when done, tick the box in
  `ROADMAP.md`, and close the milestone when it's finished.
- Commit and push finished work. Commit messages: a summary line, a body describing what changed,
  and the Co-Authored-By trailer.
- Silo (github.com/Silo-Server/silo-server, a Go + React media server) is a design reference only:
  it's AGPL-3.0, so never copy its code or exact designs.

## Layout

```
cmd/ams/             entrypoint (flags: -data, -debug, -reset-password; -config only imports an old ams.yaml)
internal/settings/   server settings in the DB (keys, ffprobe, port, network, sign-in at home); env overrides
internal/auth/       password/PIN hashing (bcrypt), session tokens, home-network check, rate limiting
internal/config/     reads an old ams.yaml once, to import it
internal/store/      SQLite (modernc, pure Go) + embedded migrations in store/migrations/*.sql
internal/probe/      ffprobe wrapper
internal/scanner/    folder walking, filename parsing (parse.go movies, parse_tv.go TV), shows.go TV sync
internal/metadata/   TMDB (movies + TV + episode groups) and TheTVDB v4 clients
internal/api/        REST API (net/http patterns); serves the web app for non-/api paths
internal/artwork/    local artwork cache (data/artwork, keyed by source URL hash; safe to delete)
internal/backup/     daily DB backups in data/backups (VACUUM INTO), retention, online restore
internal/watcher/    fsnotify on library folders -> scanner.Request after 10s quiet; hourly rescan
internal/transcode/  FFmpeg sessions writing fMP4 HLS into data/transcode; remux (copyts, measured start) or H.264 encode
internal/logbuf/     the last log lines in memory for the Dashboard
internal/schedule/   the maintenance window (hours of the night heavy work waits for)
internal/markers/    intro/recap/credits markers: chapters, IntroDB, TheIntroDB, Chromaprint detection (ffmpeg)
internal/optimize/   optimized versions (H.264/AAC MP4 copies in data/optimized), used by playback when needed
internal/sysstats/   CPU, memory and network samples for the Dashboard (Windows and Linux; Arc's own elsewhere)
internal/remote/     remote access: HTTP and HTTPS on one port, certificates (files or Let's Encrypt, data/certs), UPnP port forwarding; AMS_UPNP_URL / AMS_PUBLIC_IP_URL / AMS_ACME_URL for stand-ins
internal/notify/     webhook events (library.new, playback.start/watched, server.problem); Discord/Slack formats
internal/importer/   imports watch state from Plex (plex.tv PIN sign-in) and Jellyfin; AMS_PLEX_TV_URL for a stand-in
internal/ratings/    IMDb ratings file (weekly) and OMDb critics' scores; AMS_OMDB_URL / AMS_IMDB_RATINGS_URL for stand-ins
internal/activity/   running and recent background tasks (scans, refreshes, previews, backups) for the Activity menu
internal/trickplay/  timeline thumbnails per library (keyframes only, nearest per 10s slot), data/trickplay
internal/opensubtitles/ OpenSubtitles.com client (search, download, file hash); AMS_OPENSUBTITLES_URL for a stand-in
internal/subtitles/  subtitle tracks (in the file and beside it), extracted to cues with FFmpeg, cached in data/subtitles
internal/insights/   watch-history stats: activity overview and a profile's year in review
internal/search/     search normalising and fuzzy scoring (accents, case, small typos)
internal/webui/      embeds web/ build output (internal/webui/dist)
web/                 React 19 + TypeScript + Tailwind 4 + Radix UI + Motion + TanStack Query
tools/testmedia/     generates a fake test library
tools/apidoc/        regenerates docs/API.md from internal/api (internal/apidoc; a test fails when it's stale)
tools/scanbench/     times first scans against a stand-in TMDB with slow disks simulated (AMS_TMDB_URL, AMS_PROBE_DELAY_MS)
```

## Build, run, test

```sh
cd web && npm install && npm run build && cd ..   # MUST run before go build: the UI is embedded
go vet ./... && go test ./...
go run ./cmd/ams                                    # data in ./data, serves http://127.0.0.1:8099
cd web && npm run dev                               # UI hot reload; proxies /api to :8099
cd web && npx tsc -b                                # type-check the UI
go run ./tools/apidoc                               # after changing API routes or handler comments
```

- Settings (incl. API keys) live in the database; this PC's `data/ams.db` already has the owner's keys.
- To test first-run setup or anything needing a clean slate without touching the owner's data, run a
  second copy: `AMS_LISTEN=127.0.0.1:8199 ams -data <scratch folder> -config none`.
- API calls from scripts need a session: `POST /api/auth/setup` or sign in, keeping the cookie.
- One-line `useEffect(() => fn())` is a bug if fn returns anything (e.g. `window.scrollTo` returns a
  promise in new Chromium): React treats it as the cleanup and crashes on unmount. Use braces.
- Test library: `go run ./tools/testmedia -out ../ams-test-media` (needs ffmpeg); add its `Movies`
  and `TV` folders as libraries. It includes Firefly (TVDB 78874: aired vs DVD vs TMDB "Intended
  Order"), Mushoku Tensei (TVDB 371310: TMDB groups Cours/Crunchyroll/Seasons), a fake show (unmatched)
  and an S01E20 that doesn't exist (file-only episode).
- Verify UI changes visually: run the server and screenshot with headless Chrome, e.g.
  `chrome --headless=new --hide-scrollbars --force-dark-mode --window-size=1440,1000 --virtual-time-budget=8000 --screenshot=out.png http://localhost:8099/`.
  Headless Chrome won't render narrower than ~500px.
- The data dir (`data/ams.db`) can be deleted to start fresh; migrations recreate it.
- A new migration triggers a `before-upgrade` backup automatically when an existing database is opened.
  Restores run online (SQLite backup API) and then re-apply migrations, so old backups stay restorable.

## How TV orderings work (the headline feature)

- A TV library's `source_ordering` (tvdb_aired default, or tmdb_aired) says how files are numbered.
  That provider is "canonical": its episodes become items, keyed `ep:<series>:<provider>:<id>`.
- All orderings from both providers are stored (`orderings`, `ordering_entries`, `ordering_seasons`).
  The other provider's episodes are mapped onto canonical ones: cached ids → air date + title (free).
  The expensive per-episode TMDB external-id lookups ("deep" mapping) run only when a series is
  displayed in the other provider's ordering (`store.DisplaysProvider`) or `refresh?deep=1`.
- Files whose SxxEyy isn't in the source ordering become file-only episodes (`ep:<series>:file:s1e20`),
  later adopted (re-keyed, same id) when a provider learns them. Watch history must survive this.
- Display ordering: `series_display` with `user_id ''` = server default; per-user rows come with users
  (milestone 4). Custom orderings are copies with locked entries; manual placements are `locked=1` and
  survive refreshes.

## Gotchas

- Windows: Go handles paths over 260 chars, and so does ffprobe 9; scripts creating test files may need
  the `\\?\` prefix.
- TVDB returns relative image paths (`/banners/...`); `tvdbImage()` prefixes artworks.thetvdb.com.
- TVDB and TMDB both require visible attribution in the UI (tracked in "Before public release").
- SQLite runs with WAL and `_txlock=immediate`; keep write transactions short. The store's
  database wrapper queues Exec/BeginTx/raw restore through one write connection and keeps
  independent query-only read connections. INSERT/UPDATE RETURNING must use WriteRowContext,
  not the read QueryRowContext. Release any raw write connection before calling store methods.
- Other videos libraries are stored as kind 'movies' with `libraries.videos = 1` (the original
  CHECK constraint only allows movies/shows) and read back as kind "videos". Their videos are movie
  items with metadata_state 'locked' (never looked up online); folders are collections of kind 'folder'.
- Items are hidden from listings while none of their files are present (`missing_since`), but never
  deleted, so history survives drives going offline.
- Match explanations: provider clients `record()` each search/lookup into a `metadata.Trace` carried
  in the context; `scanner.traced()` saves it to `items.match_info`. Nested calls share the outer trace.
- Provider data is written only through `store.UpdateMetadata`, which skips fields in the item's
  `locked_fields` (set by user edits). Don't write metadata columns any other way, or locks break.
- UI checks that need clicks: drive headless Edge over CDP (a small Node script; Node 24 has
  WebSocket built in). In Git Bash set `MSYS_NO_PATHCONV=1` or `/movie/...` args become Windows paths.
- Playback: `POST /api/items/{id}/playback` decides direct/remux/transcode from the caps the player
  reports (`web/src/lib/playback.ts`). Converted streams start at the seek point and the player adds
  `offset_ms` to its clock; seeking past what's converted starts a new session. Copied (remuxed) video
  uses `-ss -noaccurate_seek -copyts` and `Manager.Ready` measures the real start with ffprobe;
  predicting keyframes failed for MKV (sparse cues) and x265's open GOPs. HEVC copies need `-tag:v hvc1`.
- Headless Chrome/Edge marks pages hidden after a few seconds and stops loading media; UI scripts
  driving playback must keep the page active (CDP `Emulation.setFocusEmulationEnabled`).
