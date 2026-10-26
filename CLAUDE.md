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

- Desktop rows and profile Home hero (owner request, 26 October): shared RowShell gives poster/
  thumbnail, cast and Extras rows visible Previous/Next controls whenever actual content
  overflows, including narrow widths and short cast lists. Bounds update on scroll/resize/data
  changes; keyboard Left/Right/Home/End, focus reveal and native horizontal/vertical input remain.
  Card sizes/actions are preserved. Your profile → Home → Show home hero defaults On; an optional
  per-profile JSON preference hides the banner without a large gap, keeps Edit Home accessible,
  and disables banner-only queries/timers. Existing profiles retain their visible default.
  No migration/dependencies. Contract: docs/HOME-NAVIGATION.md; actual local full-browser/API
  evidence and physical-browser limitations in READINESS. PR only; no PR Actions under current
  policy, and the owner asks agents to stop after opening the PR.


- Logging pass (#219, 26 October): owner said logs showed almost nothing and they couldn't tell whether
  intro/credit detection works. Normal (Info) logs now record sign-ins and failures (address/device),
  settings changes (keys/passwords shown only as "(changed)"), playback decisions with reasons,
  movie matches/no-match, metadata refresh/library work/previews/media analysis summaries, and an
  intro/credit lookup summary per pass (chapters/TheIntroDB/IntroDB/none). Detailed logging adds
  one line per file with the source found or the reason nothing was (no IMDb ID, no aired number,
  no TheIntroDB key for movies, rate limited, failed) and why local detection was skipped.
  Media analysis now shows one Activity task per library instead of one per file. Library Health
  has a Skip Intro & Credits section (GET /api/libraries/{id}/markers): per library counts and a
  By title list. No schema/migration changes.

- Maintenance upkeep (#203, 26 October): Optimize database and Clean up cached files default On
  in the unified task list, run weekly in the quiet maintenance window and offer coalesced Run now
  plus persisted last results. Database work joins the writer queue, checks integrity, updates
  query statistics and conditionally VACUUMs only substantial free pages with verified disk
  headroom. Cache cleanup recognizes generated formats, retains missing/offline references,
  recent files and unknown legacy subtitle hashes, and excludes media/backups/uploaded originals.
  Cue caches now carry file-ID ownership, adopting identifiable legacy entries on use. No schema
  migration or dependency changes. Contract: docs/MAINTENANCE-UPKEEP.md; actual local checks and
  remaining physical-storage/Windows-runtime limits in READINESS. Work submitted via PR only;
  owner requests stopping at PR creation rather than waiting for hosted verification.


- Empty trash (#204, 26 October): library More and Health share one destructive confirmation.
  Cleanup holds the scanner lease through a mandatory safety backup and transactional record
  removal. Maintenance's EmptyTrashControl is Off by default with enable confirmation, integrated
  into #202's merged task list with a scoped Choose library action. Automatic work requires a fresh complete
  scan since startup, unchanged readable roots with known-file anchors, absent missing files,
  and quiet maintenance hours, checked again after backup. Empty/offline roots require review;
  media bytes are never deleted. Contract: docs/EMPTY-TRASH.md; actual checks in READINESS.
  Merged via PR #207; no direct main push or deployment acceptance claim.


- Settings copy review (#210, 26 October): every Settings description rewritten as one short,
  plain sentence; implementation detail and cross-references removed. Maintenance tasks are named
  the same everywhere (visible, aria, Run now, server titles): Scrubbing previews, Intro & credit
  detection, Media analysis (Plex's "Upgrade media analysis": brings older files' analysis up to
  date, currently DV/HDR10+/Atmos), Metadata refresh, Database backups, Empty trash automatically. Tasks show status
  only when Running/Paused/failed. The clock is one "Server time: HH:MM zone" line under
  Maintenance time (owner asked to drop next-window detail; hours follow the server's clock).

- Simple Activity menu (#208, 26 October): owner asked for Plex-like simplicity. The Activity
  menu shows each running task with one plain line (current title, or N of M), a progress bar
  and a small Stop, then up to four past-tense Recent entries with time/errors. Steps, file
  names, scan statistics and scheduled backlogs moved to Dashboard → Activity (unchanged detail).
  activity.Handle.Discard drops runs that did nothing: the intro/credits pass and the scheduled
  metadata refresh (now "Updating details in …") no longer list empty entries.
  Scans show one line per step under way (Scanning / Matching / Analysing), each with one item
  (#214, refined after owner feedback). Intro/credit detection and previews stay separate tasks.

- Library editor layout (#201, 26 October): Add/Edit Library uses a Plex-style section sidebar
  (General: type/name; Folders; Advanced), shown as tabs on phones. Advanced groups settings
  under Metadata, Collections, Player and (adding only) Before adding, with label/help left and
  consistent drop-downs/switches right. Adding starts on General; the primary button reads Next
  until a folder is chosen, then Add Library. Labels/aria descriptions and Preview inside
  Advanced (#146/#181) are unchanged; UI only. Follow-ups: Maintenance task list (#202, done);
  Optimize database/Clean cached files (#203), Empty trash in library management/Maintenance
  with an optional off-by-default automatic mode (#204).

- Maintenance task list (#202, 26 October): Settings › Maintenance & diagnostics › Scheduled
  tasks shows the maintenance window/clock, then each task once (Scrubbing previews, Intro &
  credit detection, Media analysis, Metadata refresh, Database backups) with its Off / During
  maintenance / When added choice or switch, a readable live status and Run now in one row.
  Intro detection timing moved there from Playback, which keeps marker sources and links to it.
  Accessible control/Run-now names, settings, API and defaults are unchanged (detection still
  defaults to During maintenance; changing it is an owner decision on #80). UI only.

- Fast development images and ordering form (26 October): owner explicitly removed full
  verification from the main image feedback loop. Main publishes latest/SHA after compilation
  and image building; tests/race/browser/container jobs are intentionally skipped, not passed.
  Full verification remains the manual default and gates v-prefixed tags/PR validation.
  Manual/version publication uses SHA/version tags, leaving development latest to main pushes.
  New main pushes cancel superseded image runs. Policy: docs/IMAGE-PUBLISHING.md.
  Display order/Specials now use full-width native form fields beside File episode order;
  Compare orders is a separate button. Saved choices are distinguished from inherited effective
  order without changing files/metadata or Save/Cancel/profile behavior. Actual focused checks
  and hosted publication timing: READINESS and GitHub handoff. No full-suite claim for this UI/
  workflow change; prior 93/93 acceptance at 2ba97df is historical.


- Metadata/order simplification and Recently Added (26 October): library Metadata provider
  selects show information; File episode order selects filename interpretation and episode
  information, with per-show override in Edit → Metadata & ordering. Display order/specials
  live in that dialog too, with a season-area shortcut and profile-only access for viewers.
  Per-show Details From is removed; legacy values remain for rollback but no longer apply.
  Migration 0053 defaults existing libraries to TMDB without changing stored metadata or
  starting work. Scoped show refresh/relink uses the library lease and known present files,
  with busy/Stop/visible errors/Retry; no folder discovery or probing. Library provider/order
  changes refresh known titles; folder edits discover files. Recently Added selects candidate
  shows before complete episode groups, eliminating the 400-episode starvation cutoff.
  Contract: docs/METADATA-ORDERING.md; actual evidence/limits: READINESS and GitHub handoff.


- Automatic browser quality (#197, 26 October): owner accepted implementation after the prior
  deferral. Automatic defaults without a saved choice; manual/explicit Original persist.
  Sustained buffer/delivery pressure lowers quality, fresh buffered headroom raises it, with
  grace/cooldowns and bounded switches through existing serialized single-rendition admission.
  Exact file/audio/burn, position, captions/timing, speed and pause intent are preserved; server
  ceilings remain authoritative. Replans can briefly rebuffer. No managed relay/discovery.
  Contract: docs/ADAPTIVE-QUALITY.md; actual validation/limitations: READINESS and #197/#162.
  Episode-row actions also stay together rather than stretching/wrapping across the description;
  narrow layouts place the action group below the text. No hero geometry changes.


- Episode details/specials (#106/#109, 25 October): episode rows now open stored description,
  guest cast and director/writer credits independently of Play, with spoiler protection/Retry.
  Series cast cards show provider-reported appearance counts. Credits originally followed Details From (superseded by the current file-provider credit contract);
  canonical episode text/numbering stays separate and unsupported/unmapped sources remain empty.
  Episode Order offers opt-in specials in seasons, with profile/server-default inheritance.
  Stable mapped TVDB aired hints feed one traversal for seasons/Next Up/autoplay/queue/season
  actions, retaining Special labels, unknown fallback, manual/custom placements and history.
  Migration 0052 preserves old credits/defaults and starts no batch work. Existing libraries
  need normal Refresh metadata/maintenance to populate the new fields. Contract:
  docs/EPISODE-DETAILS-SPECIALS.md; actual results/limits: READINESS and #106/#109/#162.


- Web/server audit pass (#189/#190/#199/#192, 25 October): personal settings now save library
  navigation order (all available libraries remain visible) and future watched percentage,
  retaining the 90% default. Persisted progress and active history share the boundary; no
  earlier plays/markers/autoplay policy is rewritten. Player text captions offer session/file/
  track-local ±10s timing and Reset, including native PiP track wiring. Cast handoff/seek/retry
  keep actual file/audio and compensate remux keyframes; owned conversions are released before
  browser handback. App Chromecast/AirPlay buttons explain text-caption limits rather than
  silently dropping them. Receiver compatibility/full transport remains on #192/#135.
  WEB-SERVER-REVIEW.md records all nine decisions: #106/#109 have concrete metadata/order plans;
  #193/#198 remain deferred pending resource/listening tasks; #197 was subsequently accepted; #138 remains a release
  gate with ALPHA-CHECKLIST.md. These planned/deferred features are not implemented. Native and
  physical HDR/Atmos findings remain outside this pass. Actual checks/limits: READINESS and #162.

- Recovery/metadata review (#136/#191/#194/#104/#195, 25 October): Backups now states the
  database-only boundary and warns that files such as uploaded artwork are not restored.
  RECOVERY.md documents full stopped-folder copies; a fresh copied-store/API rehearsal
  verifies actual uploaded image bytes, locks/settings/history/resume and DB-only limits.
  #136 retains physical storage/power-loss/external-config acceptance. LOCAL-METADATA.md
  documents IDs/artwork only, including no episode-sidecar import. Full NFO import is not
  accepted. Approved language learning, single-provider metadata scopes, global schedules,
  shared edition watch identity and no source-media deletion remain. Older #104 backfill is
  superseded; provider-gap/switch assertions preserve locks/identity. Review decisions and
  fresh results: docs/REVIEW-RECOVERY-METADATA.md, READINESS and GitHub #162/the five issues.

- Library letter navigation (#188, 25 October): owner selected A–Z jumps after the initial
  deferral. Title-sorted grids use an access/filter-aware server offset beyond loaded pages;
  reverse, Load more, All titles, existing default-sort persistence and loaded-only selection
  remain. Letter URLs and bounded tab-local history positions support returning from a title.
  Numeric/non-Latin titles remain in normal browsing; no new collation/density/list settings.
  Contract: docs/LIBRARY-NAVIGATION.md; actual checks and limits in READINESS/#188/#162.

- Explicit library repairs/Stop/clock (#183/#184/#196): Manage libraries has separate confirmed
  Refresh all metadata and Reanalyze files actions, draining cursor pages rather than limiting
  operation scope to one page. They share the scanner library lease. Reanalysis preserves
  presence, original probe timestamps, identity/history/markers and rejects stale/unavailable
  revisions; manual repairs do not broaden scheduled policy. Library/Activity Stop cancels
  owned contexts and pending scanner retries, with stopping/stopped states and run IDs safe
  across restarts. Partial discovery never sweeps unvisited files. Existing bulk refresh is
  cancellable too. A server-derived maintenance clock handles overnight/all-day/DST policy.
  Contract: LIBRARY-WORK-CONTROLS.md; actual local/hosted evidence and limits: READINESS/issues.
  First hosted 8594200 passed 76/77 browser journeys but blocked publication on an older
  Refresh-button selector. The existing recovery test now opens More → Refresh metadata;
  its failure/Retry/persisted-watchlist assertions and all runtime/CI behavior remain.
- Title actions (#187): Refresh metadata lives in More, with existing failure Retry retained.
  Playback queue has an explicit label and a distinct icon from saved playlists. Approved
  hero/poster/background/Cast geometry and queue behavior are preserved. #188 was reviewed:
  existing search/filter/sort/paging and Back navigation are present; density/alphabet controls
  were initially deferred. The owner subsequently selected jump-to-letter (#188); see the
  current navigation contract below.

- Preplay track choices (#186): Playback options on movies/series-next/episode rows opens a
  compact version/audio/subtitle dialog with Play/Resume. A profile-authenticated source-options
  endpoint shares actual playback file/audio resolution and starts no stream or conversion.
  Version changes reset track IDs; explicit choices bind to the selected file and survive
  initial retry. Subtitle Off is explicit, picture burn-in enters the first request, and a
  disappeared text track reports fallback. The selected edition label follows the chosen file.
  Ordinary Play, approved profile learning and optimized eligibility remain; cancelling/failed
  starts do not learn preferences. No migration/dependencies. Contract: PREPLAY-OPTIONS.md;
  actual isolated checks/limits in READINESS; hosted publication and completion on #186/#162.

- Library/backup/bulk clarity (#181/#182/#185): library name and movie/show advanced selects
  now have explicit labels with unique IDs; existing explanatory text is associated with the
  corresponding select. Backups reads live settings, reports On/Off and the server-local
  maintenance window (including overnight/all-day), keeps manual/safety backups available,
  reports the last saved successful backup and links to the existing maintenance controls.
  Unavailable policy has local Retry rather than assuming Off. Bulk selection says Select
  loaded (N); loading another page never silently selects it. No API/schema/scheduling or
  all-matching selection change. Actual isolated checks and limits are in READINESS; hosted
  publication and completion evidence are tracked on the individual issues and #162.

- Recoverable UI failures (#176–#180): New for You, playlist selection, Devices, Optimized
  Versions and Notifications distinguish unavailable/loading/empty/loaded states, retain
  cached data with an explicit stale notice and offer local Retry. Playlist creation waits
  for a usable list while retaining drafts; webhook forms wait for available event data.
  Failed news refreshes are not marked seen on close. Device revocation owns pending/error
  state per row and removes only after server confirmation, including failed subsequent
  list refresh. Episode artwork/details have a native playback button with Enter/Space,
  a visible focus ring and separate sibling actions. No API/schema/scanning policy change.
  Isolated actual results and fixture limitations are in READINESS and #176–#180. First
  hosted b0fc9dc passed 60/62 journeys but blocked publication: two older /^Play S/ selectors
  also matched the new Play Second Chapter button. They now target the season/episode-code
  button explicitly; all queue/progress/countdown assertions and runtime labels stay intact.

- Audit corrections #172/#174/#175: completed optimized copies record measured output duration/
  average total bitrate (migration 0051), with height and effective manual/remote/upload-budget
  eligibility. Unknown legacy measurements remain usable without a limit and otherwise fall
  back; no batch rebuild. Search no longer substitutes a previous query's results and exposes
  pending status. Library filters wrap on phones, retain reachable active values, and offer
  Clear filters in the empty state while preserving sort/direction. Contract: OPTIMIZED-COPIES;
  actual validation and limits: READINESS and the individual issues.

- Optimized audio correctness (#171): migration 0050 saves the exact source audio index,
  language and title. Eligibility checks the effective profile/explicit choice, including
  same-language commentary; mismatches use normal original/remux/conversion playback.
  Matching explicit choices can reuse a copy. Playback responses identify optimized quality
  and actual AAC stereo audio separately from source choices; player info no longer calls
  an optimized copy the original. Legacy completed copies need explicit Optimize again;
  there is no automatic reconversion. Contract: docs/OPTIMIZED-COPIES.md; actual verification
  and limitations in READINESS and #171. #172 bitrate follow-up is recorded above.

- Optimized-copy source revisions (#170): migration 0049 records saved source snapshots and
  opaque attempt IDs. Changed files invalidate old copies; playback and stream requests also
  check real source metadata/artifact availability before a scan. Attempt-owned completion and
  unique filenames prevent late conversions overwriting newer results; restart/cleanup and
  explicit missing-copy repair are covered. Older completed copies lack provenance and ask
  for Optimize again, without an automatic batch rebuild. Originals/history/edits are retained.
  Contract: docs/OPTIMIZED-COPIES.md; local Go/race, real-media HTTP and playback browser evidence
  in READINESS. Hosted publication is separate on #170; audio follow-up is recorded above; #172 follow-up is recorded above.
- Arc/Plex audit review queue: #162 indexes all 36 findings (new review issues #170–#199,
  plus existing dedicated issues). Review each before implementing, deferring or closing with
  a reason. Work starts with CI #173, then optimized playback #170–#172; optional/intentional
  differences are not automatically accepted changes. #173 replaces a scheduler-dependent
  TestEach assertion with gated concurrency, exact single-worker stop and delayed-error tests.
  Scanner runtime and CI gates are unchanged. Commit 6886f1f passed hosted run 40023241460,
  including all 50 browser journeys, container checks and image publication; #173 is closed.
- Incremental rescans (#169, 23 October): ordinary scans no longer refresh matched titles
  because of age or retry old unmatched titles. Maintenance owns overdue/incomplete movie
  refresh (draining pages), due/airing shows and old unmatched recovery under existing admission.
  Discovery still handles new/pending content, unfinished series imports, genuinely new unknown
  episodes and explicit language/source/provider changes. New episode lookup bypasses stale
  preview responses. Fresh batch comparisons preserve unchanged episode links; unchanged local
  artwork/theme/season art avoid the writer queue. Activity distinguishes checking from changing
  links. No migration or identity cache. Plan: docs/INCREMENTAL-RESCAN-PLAN.md. Affected Go/API,
  race/vet/build and eight focused browser journeys pass; actual results/limits in READINESS.
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
  That original all-push gate is superseded by the owner-approved development-image policy
  above. Full verification remains for manual/version/PR runs.
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
  Show details now follow ONE library metadata provider; episode information follows file
  numbering. Per-show Details From was superseded on 26 October; never backfill gaps (owner). Skip Intro/Credits are per library (as Plex);
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
  direct main pushes were later allowed, then replaced by PRs for everything (see current policy below). Do not
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
  authoritative; keep handoffs current, follow the current workflow policy below, and never infer
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
- Current workflow policy (owner, 26 October): ALL additions and fixes go through pull requests,
  by Claude and Codex alike. This supersedes the 22 October direct-to-main policy. Never push to
  main. Work on a branch (Claude: the branch the session assigns, `claude/...`; Codex: `codex/...`),
  open a PR into main. PRs start no GitHub Actions runs (owner, 26 October, to save Actions
  minutes): before opening a PR the agent runs the relevant checks itself (go vet/test, tsc,
  production build, affected browser journeys; the full browser suite for broad changes) and
  lists the actual results in the PR; the owner tests too. Container checks need the manual "Run
  workflow" full verification (Linux only; Windows CI removed until Arc runs on Windows), which
  release tags also require. The owner merges,
  or tells the agent to. Keep one
  branch per agent/task and never have two agents on the same branch. Preserve unrelated work,
  never force push, keep PRs focused, and keep GitHub handoffs and these notes current in the PR.
  Merging to main publishes the development `latest` image (docs/IMAGE-PUBLISHING.md). Start new
  work from the latest main, and after a PR merges restart the branch from main rather than
  stacking commits on merged history.
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
