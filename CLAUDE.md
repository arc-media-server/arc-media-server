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
  Next: 17, then 18; the owner asked for 15-18 in one go.
- The owner has said to keep working through milestones without check-ins. For product decisions,
  pick a sensible default, build it to be easy to change, and log it as a comment on GitHub issue #80
  ("Decisions waiting for the owner"); they review those in a batch. Only stop for risky or
  irreversible things.
- Everything must be configurable in the web UI; no config files (issue #66).
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
```

## Build, run, test

```sh
cd web && npm install && npm run build && cd ..   # MUST run before go build: the UI is embedded
go vet ./... && go test ./...
go run ./cmd/ams                                    # data in ./data, serves http://127.0.0.1:8099
cd web && npm run dev                               # UI hot reload; proxies /api to :8099
cd web && npx tsc -b                                # type-check the UI
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
- SQLite runs with WAL and `_txlock=immediate`; keep write transactions short.
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
