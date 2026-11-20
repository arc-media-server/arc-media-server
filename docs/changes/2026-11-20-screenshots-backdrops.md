# Screenshots: Continue Watching as Backdrops

What changed:
- **Home's look in the shared library (owner, 20 November):** `tools/screenshot-library.json`
  gains `home`, with Continue Watching as **Backdrops** (each title's backdrop with its clear
  logo; the web's "Backdrops" card style, `arc`). `web/tools/screenshots.mjs` saves it as the
  admin's default Home layout (`PUT /api/home/default`). Profiles that haven't arranged their own
  Home get it through their preferences (v1 `/me/prefs`), so every app photographed against
  `--serve` shows the same look.
- **`--only`:** `--only home.png,watch-together.png` saves just those web pictures, so a library
  change that affects one page doesn't rewrite the rest.
- `docs/screenshots/home.png` and `watch-together.png` retaken; the other five are unchanged.
- `docs/SCREENSHOTS.md` describes both.

Why: the owner prefers the Backdrops look for the screenshots; Thumbnails (Plex's default) stays
Arc's default for real profiles.

Checks:
- `node tools/screenshots.mjs --only home.png,watch-together.png` run against a scratch server;
  both pictures looked at (Severance, Breaking Bad, Dune: Part Two, Oppenheimer and Spider-Man
  with their logos).
- `cd web && npx tsc -b` passes. No Go or web app code changed.
- No owner server, data, media or keys used.

Limits: apps that don't support a row's card style yet show their own default there.

Logging: none; this is a developer tool.
