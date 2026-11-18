# Arc Media Server (AMS): notes for AI agents

An open source media server (Plex/Jellyfin alternative) in Go with an embedded React web app.
@docs/AGENT-RULES.md

The shared rules above apply to every Arc repository, including loading references only when
relevant. This file adds the server's own. Read `docs/DEVELOPING.md` when needed for machine
setup, code layout, build/test details and TV ordering gotchas; `ROADMAP.md` for planning and
`README.md` for the project overview.

## Where things stand

- **Status:** private alpha. Milestones 1–20 are done; the focus is milestone 21, stability and
  release readiness (#130–#138), plus the owner's requests. Arc runs day to day on the owner's
  Unraid server in Docker. On a fresh clone, follow "Setting up a new machine" in
  `docs/DEVELOPING.md`.
- **SQLite only (owner, 11 November):** PostgreSQL is deferred under #308; keep PRs #337/#339
  unmerged as reference and don't maintain a parallel PostgreSQL path unless the owner asks.
- **Open owner items:** translations beyond English (#122); real-device playback and casting
  (#135, #192); the optimized copies storage budget (#193, deferred); decisions labelled
  `owner decision` (TheTVDB access is #478).
- **Ratings:** IMDb downloads and web scores/sort are paused; identifiers remain
  (`docs/RATING-SOURCES.md`).
- **Downloads (#62):** originals, offline bundles and resumable transfer are in; no converted
  downloads, offline progress sync or native acceptance yet (`docs/DOWNLOADS-OFFLINE-API.md`).
- **Apps:** arc-android, arc-apple, arc-kodi, arc-roku and arc-tv (LG/Samsung), each in its own
  repository, all on `/api/v1` only; API gaps they need are fixed here.
- **History:** `docs/changes/` (one note per change), `docs/HISTORY.md` (older notes); readiness
  evidence in `docs/READINESS.md`.

## Server rules

- **Checks before a PR:** `go vet ./...`, `go test ./...`, `cd web && npx tsc -b`, the production
  build, the affected browser journeys (the full suite for broad changes). Full container
  verification is the owner's manual "Run workflow", which release tags also require. Merging to
  main publishes the development `latest` image (`docs/IMAGE-PUBLISHING.md`).
- **API:** after API changes run `go run ./tools/apidoc` (rewrites `docs/API.md`,
  `docs/openapi.json` and adds to `docs/api-v1-contract.txt`). Delete a contract line only for a
  deliberate break.
- **Web text:** all new UI text goes through `t()` and the English catalogs in
  `web/src/locales/en`; `Trans` for rich text, whole phrases and plural keys. Never translate API
  enums, tokens, key values or stored identifiers (`docs/TRANSLATION.md`).
- **Metadata:** provider data is written only through `store.UpdateMetadata` (it honours
  `locked_fields`). Episode identity is separate from display order; watch history must survive
  re-keying (`docs/DEVELOPING.md`, TV orderings).
- **SQLite:** WAL with `_txlock=immediate`; keep write transactions short; INSERT/UPDATE RETURNING
  uses `WriteRowContext`. A new migration triggers a `before-upgrade` backup automatically.
- **Scratch servers only:** `AMS_LISTEN=127.0.0.1:8199 ams -data <scratch> -config none`, with the
  `tools/testmedia` library. Settings and keys live in the database, never in files.
- **Milestones:** work milestone by milestone (`ROADMAP.md`) and add web UI for new features as you
  build them. Close issues with the commit hash, tick the box in `ROADMAP.md`, close the milestone
  when it's finished. Provider attribution marks live in `web/src/assets/attribution/` (#64).
- **Build order:** `cd web && npm install && npm run build` before `go build` (the UI is embedded).

## Logging and PR review

Every PR considers whether failures and stalls in its scope can be diagnosed and says what it logs
(`.github/pull_request_template.md`; limits in `docs/LOGGING.md`).

- Log background-job starts/outcomes with duration and counts, and important decisions such as
  conversion reasons. Successful page loads and polls need no info-level record.
- Log actionable failures once at the responsible boundary, with safe operation/provider context.
  Expected cancellations, empty results and ordinary validation aren't server errors.
- Per-item detail at debug, behind temporary detailed logging; guard expensive debug arguments
  with `log.Enabled`. Never info in file/segment/progress loops; summarise and rate-limit repeats.
- Never log passwords, keys, tokens, raw URLs/query strings, request bodies or SQL arguments.
  Diagnostic stacks are admin-only and on demand.
- Test meaningful failure/stall and privacy boundaries with isolated fixtures.
