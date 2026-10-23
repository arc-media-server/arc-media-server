# Arc Media Server Roadmap

## Vision

Everything Jellyfin can do, with the ease and polish of Apple software. Simple by default,
powerful when you need it.

## Principles

1. **Never touch the user's files.** AMS reads media; it never renames, moves, tags or writes beside it.
2. **Ease over options.** Sensible defaults, no setting without a reason, nothing important buried.
3. **Built in, not bolted on.** No plugin system. Features live in AMS, are maintained with it and
   ship with it. Anyone can contribute a feature to the project instead.
4. **Storage order is not display order.** How files are named on disk never dictates how they're shown.
5. **Work offline.** Online services enrich AMS, but it must keep working when they're unavailable.

## Milestones

### ✅ 1. Core (movies)
- [x] SQLite database with migrations
- [x] Movie scanning with robust filename parsing; extras and trailers skipped
- [x] Media info via ffprobe
- [x] TMDB matching
- [x] Missing-file detection that keeps history
- [x] Direct-play streaming with range requests
- [x] Native REST API

### ✅ 2. TV shows and display orderings ⭐
- [x] Series/episode scanning (SxxEyy, multi-episode files, specials, absolute numbering)
- [x] TVDB and TMDB series metadata
- [x] Source ordering per library (how filenames are numbered)
- [x] Display orderings: TVDB aired/DVD/absolute, TMDB aired, TMDB episode groups, custom
- [x] Mapping between orderings: IDs first, then air date + title, then manual overrides
- [x] Display ordering chosen per series
- [x] Display ordering overridable per user
- [x] Display-order preview: compare orderings side by side before switching
- [x] Unmatched episodes collected in a review bucket, never hidden

### ✅ 3. Library health and maintenance
- [x] Library preview when adding a folder: what was found, what couldn't be identified and why, before anything is imported
- [x] Library health page: unmatched items, probe failures, missing files, duplicates, missing episodes
- [x] "Why didn't this match?": show what was parsed from the filename and what was searched for
- [x] "Fix match": search a provider and pick the right result
- [x] Lock metadata fields against refreshes
- [x] Folder watching: new files appear without a manual scan
- [x] Local artwork cache
- [x] Automatic database backups with retention, plus one-click restore

### ✅ 4. Users and watch state
- [x] Accounts, login, sessions/tokens; first-run setup creates the admin
- [x] Settings in the web app, no config file: API keys, ffprobe found automatically, port and network access
- [x] Per-user profiles with PINs (Plex-style managed users), optional sign-in bypass on the home network
- [x] Watched state, resume position, play counts
- [x] Mark a season or whole show as watched/unwatched
- [x] Continue Watching, Next Up (follows the display ordering), Recently Added
- [x] Richer metadata: genres, cast and crew, studio/network, content rating, tagline, ratings
- [x] Parental controls by content rating
- [x] Remove items whose files are gone ("Empty trash")

### ✅ 5. AMS web app (Apple-style)
_Started early (after milestone 2) so progress can be seen as it's built._
- [x] Foundation: React + TypeScript + Tailwind + Radix, embedded in the binary
- [x] Design system basics: typography, colour tokens, motion, light/dark
- [x] Design system: refine spacing, components and states as the app grows
- [x] Home, library browsing, item detail, search, player (direct play)
- [x] Admin area that's simple: setup wizard, libraries, users, health
- [x] Admin dashboard: now playing, stop a stream, server log
- [x] Browse filters (genre, decade, unwatched, resolution, rating) and more sort options
- [x] Use local artwork and NFO files (read-only)
- [x] Extras and trailers on movie and show pages
- [x] Choose artwork from TMDB/TVDB alternatives or upload your own
- [x] Customisable home: reorder and hide built-in rows
- [x] Custom rows from filters ("Unwatched 90s action", "Added this week", "Short films under 90m")
- [x] Seasonal rows (Christmas films in December, horror in October), built on the same filter engine
- [x] Spoiler protection: blur thumbnails and hide descriptions of unwatched episodes

### ✅ 7. Transcoding and subtitles
- [x] Direct play / direct stream / transcode decision per client
- [x] FFmpeg to HLS, with segment cache and cleanup
- [x] Hardware acceleration: NVENC, Quick Sync, VAAPI, VideoToolbox
- [x] HDR to SDR tone mapping
- [x] Multiple versions: group 4K/1080p copies of the same title, auto-pick the best one for the device and connection
- [x] Editions (Director's Cut, Extended…) as choices on one movie
- [x] Player basics: audio track choice, keyboard shortcuts, playback speed
- [x] Device compatibility report: which files will need transcoding on which devices, and why
- [x] Scrubbing thumbnails (trickplay) when seeking
- [x] Subtitles: embedded, external .srt/.ass/.vtt, burn-in when needed
- [x] Online subtitle download

### ✅ 9. Collections, playlists and discovery
- [x] Collections (TMDB collections automatically, plus manual)
- [x] Smart collections using the same filter engine as home rows
- [x] Playlists
- [x] Better search (people, genres, fuzzy matching)

### ✅ 10. Insights and stats
- [x] Watch time and history per user, per library, over time
- [x] Never-watched report: series and movies nobody has played
- [x] Storage report: what takes up the most space, and space used by never-watched titles
- [x] Most-watched titles, genres, and busiest days/times
- [x] Yearly "Wrapped"-style summary per user
- [x] Clear logos on movie and series pages (#81)
- [x] Cast higher up on series pages, not below every episode (#82)

### ✅ 12. Everyday playback
- [x] Auto-play the next episode, with a countdown (#83)
- [x] Preferred audio language per profile (#84)
- [x] Quality choice in the player (#85)
- [x] Remove from Continue Watching (#86)
- [x] Chapters in the player (#87)
- [x] Picture-in-Picture (#88)

### ✅ 13. Profiles and home
- [x] Choose which libraries each profile sees (#89)
- [x] Admin-set home layout for other profiles (#90)
- [x] Collections and playlists as home rows (#91)
- [x] Personal ratings and a watchlist (#79)
- [x] "More like this" on title pages and a Recommended view per library (#92)
- [x] Edit collections: artwork and details (#111)
- [x] Season posters on the show page (#112)

### ✅ 14. Library management
- [x] Merge and split titles by hand (#93)
- [x] Edit several titles at once (#94)
- [x] Labels, for filters, smart collections and profile restrictions (#95)
- [x] Metadata language per library (#96)
- [x] Scheduled maintenance window for heavy background jobs (#97)
- [x] TMDB numbering by default, per-show episode numbering, editable genres (#103)
- [x] "Other videos" (home videos) library type (#77)
- [x] Show status and next air date (#107)
- [x] Original language of movies and shows (#108)
- [x] Search by alternative titles and keywords (#110)
- [x] External ratings: IMDb, Rotten Tomatoes, Metacritic (#113)
- [x] Activity: see what the server is doing, like a library scan's progress (#114)

### ✅ 15. Moving in and extras
- [x] Import from Plex and Jellyfin: users, watch history, resume points (#22)
- [x] Notifications and webhooks (#78)
- [x] Theme music on show pages (#98)
- [x] Pre-roll videos before movies (#99)

### ✅ 16. Intro and credits skipping
- [x] Crowdsourced timestamp source (IntroDB or similar) as the first choice (#43)
- [x] Check timestamps against each file's runtime before trusting them (#44)
- [x] Local detection fallback (audio fingerprinting across episodes), run in the background at low priority (#45)
- [x] Skip Intro / Skip Credits in the player (#46)
- [x] Manual editing of markers (#47)

### 17. Remote access and devices ✅
- [x] HTTPS, secure remote access setup (#57)
- [x] Sharing with friends, library access per user (#58)
- [x] Bandwidth limits for remote streams (#59)
- [x] Casting: Chromecast and AirPlay (#100)
- [x] Pre-converted versions for difficult devices (#101)

### 18. Arc apps and client API
_Arc has its own clients; the API is open for anyone to build more._
- [x] Client API for apps: documentation, app sign-in (device tokens, TV sign-in codes) and versioning (#102)
- [ ] Native apps (tvOS, iOS, Android TV) (#63)

### 19. Quick wins and server resources ✅
_From the Oct 2026 Plex review: small gaps Plex users expect, plus resource graphs._
- [x] Home networks setting (#115)
- [x] Transcoder: temporary folder and conversions at once (#116)
- [x] Shuffle play (#117)
- [x] Subtitle styling and subtitle mode (#118)
- [x] Collection display mode (#119)
- [x] Smart playlists (#120)
- [x] Dashboard: server resource graphs (#121)

### 20. Faster scanning ✅
_First scans of big libraries (about 1,800 films on Unraid) are much slower than Plex's._
- [x] Identify titles from names first, read files in the background (#124)
- [x] Look up several titles at once (#125)
- [x] Read files in parallel, kindly to spinning disks (#126)
- [x] Clearer scan progress (#127)
- [x] Measure scan speed on a large, slow library (#128)
- [x] Reuse the preview's matches when the library is added (#129)

### 21. Stability and release readiness
_Feature breadth is in place. Prove the existing features work together before an alpha release._
_Acceptance criteria and verification records: [docs/READINESS.md](docs/READINESS.md)._
- [x] Harden forwarded sign-in, HTTPS session cookies and cast-session revocation (#130)
- [x] Enforce conversion limits under simultaneous playback (#131)
- [x] Gate Docker publication on Linux/Windows checks and Linux race/media tests (#132)
- [x] Automate core browser journeys (#133)
- [x] Save the final resume point when the player closes (#140)
- [x] Keep late subtitle responses from replacing the latest selection (#141)
- [x] Filter Continue Watching before limiting its results (#142)
- [x] Label removal safety backups correctly (#143)
- [x] Review everyday UI flows, log candidates, reproduce and fix confirmed defects (#144)
- [x] Reduce repeated scanning/matching work and time until new media is usable (#145; isolated correctness and timing evidence in READINESS; live-library validation remains #134)
- [x] Add libraries directly with background matching and optional Advanced preview (#146)
- [x] Add global scanning controls; separate recurring discovery from maintenance refresh (#147; contract in [SCANNING-CONTROLS](docs/SCANNING-CONTROLS.md), isolated evidence in READINESS)
- [x] Assess broader everyday UI/feature gaps across the eight requested areas (#148; [UI-REVIEW](docs/UI-REVIEW.md); source assessment and focused reproductions, not exhaustive acceptance)
- [x] Fix reproduced Home/load/save feedback, mobile Profiles and keyboard Home reordering gaps (#149–#152)
- [x] Complete approved UI follow-up: shared filters/full results, display-order inheritance, grouped Settings, mobile controls, audio preference and editable playback queue (#148; isolated evidence in UI-REVIEW and READINESS)
- [x] Serialize differently named folders for the same series during progressive scanning (#153; CI-discovered failure and repeated regression in READINESS)
- [x] Queue SQLite writes independently of reads to prevent in-process SQLITE_BUSY under contention (#154; deployment confirmation remains separate)
- [x] Add temporary detailed logging, retained history and redacted diagnostic downloads (#155)
- [x] Remove redundant poster-only save work and show save-stage progress (#156; remaining real-server latency needs timed evidence)
- [x] Clarify scan Activity and keep browsing independent of session bookkeeping; index cached-episode cleanup (#158; isolated evidence in READINESS, deployment confirmation pending)
- [x] Balance movie/series headers, enlarge desktop posters and expand long synopses (#160; isolated viewport and keyboard evidence in READINESS)
- [x] Refine Settings grouping, persistent navigation, local save feedback and accessibility; lower title headers and tighten the Cast gap (#161; isolated evidence in READINESS)
- [x] Show evidence-based source HDR and codec/profile audio labels with bounded one-time enrichment (#163; source metadata only, device acceptance remains #135)
- [x] Unify background analysis and maintenance scheduling, processing modes, resumable work and task controls (#164; isolated evidence in READINESS)
- [x] Avoid the slow Azure HTTP package mirror in CI media prerequisite installation (#165; hosted verification remains separate)
- [x] Consolidate management pages under Settings and link Dashboard from Activity (#166; desktop/phone navigation and profile-boundary checks in READINESS)
- [x] Explain active scan/matching/inspection work by title, preserving concurrent operations and truthful reused/error summaries (#167; isolated backend/browser evidence in READINESS)
- [x] Add per-library and all-library scans in Settings library management, with running guards and partial-failure retry (#168; isolated desktop/phone evidence in READINESS)
- [ ] Validate real-library scanning and metadata/watch-history preservation (#134)
- [ ] Verify playback, hardware conversion and casting on real devices (#135)
- [ ] Prove failure recovery, upgrades and backup completeness (#136)
- [ ] Complete a 7–14 day real-server trial with resource measurements (#137)
- [ ] Verify clean installations and alpha release packaging (#138)

### Before public release
_Doesn't block development; must be done before AMS is shared or published._
- [ ] TheTVDB and TMDB attribution (required by their API terms)
- [ ] Check TheTVDB licence terms for open-source distribution (shipped key vs each user's own key)
- [ ] Docker image (multi-arch, FFmpeg included, published to GitHub Container Registry) (#123)

## Later

- Music (deliberately deferred)
- Audiobooks, photos, live TV/DVR
- Offline downloads/sync
- Translate the web app into other languages (react-i18next + Weblate) (#122)
- Proper `/docs` API documentation: OpenAPI contract, searchable reference, guides and examples (#139)
- Fill blank episode details from the other provider (#104)
- Keep every country's age rating (#105)
- Per-episode cast and crew, and actor episode counts (#106)
- Place specials between seasons (TheTVDB "airs before") (#109)

## Explicitly not planned

- Jellyfin-compatible API (was milestone 6): decided in Oct 2026 that Arc is its own thing, with its
  own apps and an open API for anyone to build clients on (milestone 18)
- Plugin system or third-party extension API
