# Shared screenshot library for every Arc app

Work in progress, waiting on the owner's choice of titles.

The README screenshot library moves out of `web/tools/screenshots.mjs` into
`tools/screenshot-library.json`, so every Arc app photographs the same titles. It holds the films,
shows, collections, profiles, watching, watchlist and the featured film and show. The script also
gets three new options:

- `--serve` leaves the scratch Arc running on 127.0.0.1 for an app to photograph, printing its
  address, sign-in and ids;
- `--approve <code>` signs an app in with its link code;
- `--state <file>` writes the address, sign-in and ids as JSON.

Keys come only from `AMS_TMDB_API_KEY` / `AMS_TVDB_API_KEY`. `docs/SCREENSHOTS.md` explains how to
run it and how each app uses it. `docs/AGENT-RULES.md` and the README now mention the library and
credit TMDB.

Decisions: Continue Watching entries are written a second apart, oldest first, so the row comes
out in the same order every run. The Home hero still depends on the order in which the scan adds
titles.

Checks so far:
- README mode was run twice. The library and Continue Watching come out the same as before.
- `go vet ./...` and `cd web && npx tsc -b` pass.
- Not yet run: `--serve`, `--approve` and `go test ./...`.

Logging: none; this is a developer tool, and it never prints keys.
