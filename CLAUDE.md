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
- If `ams.yaml` doesn't exist, copy `ams.example.yaml` to it and ask the owner for their TMDB and
  TVDB API keys (they have both; keys never go in git). If `ffprobe` isn't on PATH, put its full
  path in `ams.yaml`.
- Create the test library outside the repo with `go run ./tools/testmedia -out ../ams-test-media`
  and paste the printed `libraries:` block into `ams.yaml`, unless the owner gives real media paths.
- Build the UI (`cd web && npm install && npm run build`), then `go run ./cmd/ams`, and tell the
  owner to open http://127.0.0.1:8099.
- Check `ROADMAP.md` and the open GitHub milestones (`gh issue list -R JonzaUK/arc-media-server`) to
  see what's next, then confirm with the owner before starting.

## Where things stand

- Done: milestone 1 (movie core), milestone 2 (TV shows + display orderings), and the first version
  of the web app (pulled forward from milestone 5). The owner loves the design and the orderings feature.
- Next planned: milestone 3 (library health and maintenance), building its UI alongside.
- Keep this section current at the end of each working session.

## Principles (non-negotiable)

1. **Never modify media files.** AMS only reads: no renaming, moving, tagging or writing beside them.
2. **Ease over options.** Apple-level polish and simplicity; no setting without a reason.
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
cmd/ams/             entrypoint (flags: -config, -debug)
internal/config/     YAML config (ams.yaml; keys can also come from AMS_TMDB_API_KEY / AMS_TVDB_API_KEY)
internal/store/      SQLite (modernc, pure Go) + embedded migrations in store/migrations/*.sql
internal/probe/      ffprobe wrapper
internal/scanner/    folder walking, filename parsing (parse.go movies, parse_tv.go TV), shows.go TV sync
internal/metadata/   TMDB (movies + TV + episode groups) and TheTVDB v4 clients
internal/api/        REST API (net/http patterns); serves the web app for non-/api paths
internal/artwork/    local artwork cache (data/artwork, keyed by source URL hash; safe to delete)
internal/backup/     daily DB backups in data/backups (VACUUM INTO), retention, online restore
internal/watcher/    fsnotify on library folders -> scanner.Request after 10s quiet; hourly rescan
internal/webui/      embeds web/ build output (internal/webui/dist)
web/                 React 19 + TypeScript + Tailwind 4 + Radix UI + Motion + TanStack Query
tools/testmedia/     generates a fake test library
```

## Build, run, test

```sh
cd web && npm install && npm run build && cd ..   # MUST run before go build: the UI is embedded
go vet ./... && go test ./...
go run ./cmd/ams                                    # reads ams.yaml, serves http://127.0.0.1:8099
cd web && npm run dev                               # UI hot reload; proxies /api to :8099
cd web && npx tsc -b                                # type-check the UI
```

- `ams.yaml` is git-ignored (holds API keys). Copy `ams.example.yaml`. The owner has TMDB and TVDB keys.
- Test library: `go run ./tools/testmedia -out ../ams-test-media` (needs ffmpeg) prints the library
  config to paste into `ams.yaml`. It includes Firefly (TVDB 78874: aired vs DVD vs TMDB "Intended
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
- Items are hidden from listings while none of their files are present (`missing_since`), but never
  deleted, so history survives drives going offline.
- Match explanations: provider clients `record()` each search/lookup into a `metadata.Trace` carried
  in the context; `scanner.traced()` saves it to `items.match_info`. Nested calls share the outer trace.
- Provider data is written only through `store.UpdateMetadata`, which skips fields in the item's
  `locked_fields` (set by user edits). Don't write metadata columns any other way, or locks break.
- UI checks that need clicks: drive headless Edge over CDP (a small Node script; Node 24 has
  WebSocket built in). In Git Bash set `MSYS_NO_PATHCONV=1` or `/movie/...` args become Windows paths.
