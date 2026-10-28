# README rewritten as the GitHub landing page (28 October)

The README had grown into a 540-line running changelog plus a hand-kept API table that duplicated
docs/API.md. It is now a short landing page: what Arc is, alpha status, principles, features
grouped by area, Docker install, backups/upgrades, building from source, development, a pointer to
the generated API reference, and provider credits (including TMDB's required notice).

Corrected on the way: per-show "Details From" (superseded on 26 October) is gone; reverse-proxy
guidance now points at Settings › Network › Trusted proxies (#252) instead of saying no such
setting exists; the stale Layout block is removed (CLAUDE.md keeps the current one).
Detailed feature notes stay in docs/ and docs/changes/.

## Licence (owner decision, 28 October)

Arc is licensed AGPL-3.0-only, copyright "The Arc Media Server Authors". `LICENSE` holds the
unmodified licence text (GitHub detects it); the README has a short Licence section;
`web/package.json` and the Docker image (`org.opencontainers.image.licenses`) declare it. All
direct server and web dependencies are permissive (BSD, MIT, Apache-2.0, ISC, OFL), and FFmpeg
runs as a separate program, so nothing conflicts. A modified Arc run for others must offer its
source (AGPL section 13); a visible "Source" link in the app would make that easy for forks.
