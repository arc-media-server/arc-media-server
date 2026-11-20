# Shared screenshot library for every Arc app

Every Arc repository's screenshots now show the same library: well-known films and shows,
matched on TMDB/TheTVDB, so the artwork, descriptions and cast are real and recognisable. This
replaces each app's own fixture data (owner, 20 November).

- The library is defined in `tools/screenshot-library.json`:
  - 24 films and 5 shows with their first season;
  - a Christopher Nolan collection;
  - three extra profiles (one Kids);
  - Continue Watching, Up Next and watchlist entries;
  - the featured film and show (Dune: Part Two, Breaking Bad).
- `web/tools/screenshots.mjs` reads that file and gains three options:
  - `--serve` keeps the scratch Arc running on 127.0.0.1 for an app to photograph, prints its
    address, sign-in and ids, and stops on Ctrl+C;
  - `--approve <code>` approves an app's link code;
  - `--state <file>` writes the address, sign-in and ids as JSON.
- Keys come only from `AMS_TMDB_API_KEY` / `AMS_TVDB_API_KEY` in the environment. The
  `--tmdb`/`--tvdb` flags are gone, so keys don't end up in shell history.
- `docs/SCREENSHOTS.md` (new) explains how to run it and how each app uses it.
- `docs/AGENT-RULES.md` gains the rule.
- The README credits TMDB under the pictures.
- The README pictures were retaken from the new library.

Decisions:
- Well-known titles for repositories, with licence-checked artwork kept for store listings
  (owner). TMDB's artwork belongs to the studios, so the README credits TMDB and says the titles
  belong to their owners. Plex's own marketing uses licensed titles; Jellyfin's uses open films.
- Continue Watching is written oldest first, one second apart, so its order is the same every
  run.
- The Home hero is whichever show the scan added last, so it can differ between runs.

Checks:
- README mode was run, and all 29 titles matched with artwork.
- `--serve` was run on another port:
  - a v1 link code approved with `--approve` returned the token on its poll;
  - password login returned 200;
  - Ctrl+C stopped the server and removed the temporary folder.
- `go vet ./...` and `cd web && npx tsc -b` pass. `go test ./...`: 36 packages pass; `internal/transcode` fails the same four FFmpeg tests as unchanged main here (this FFmpeg lacks the Jellyfin filters). No Go code changed.
- No owner server, data, media or keys were used; the keys came from the session's environment.

Logging: none added. It's a developer tool, and it never prints keys. It prints the throwaway
password for the throwaway server, which is the point of `--serve`.
