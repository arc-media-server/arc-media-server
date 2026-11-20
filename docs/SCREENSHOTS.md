# Screenshots: the shared library

Every Arc repository's screenshots (READMEs, change notes, docs) show the same library:
well-known films and TV shows, matched on TMDB and TheTVDB, so the posters, backgrounds, logos,
descriptions and cast are real and recognisable. That keeps the apps looking alike side by side
and replaces hand-written fixture data (owner, 20 November). No films are included: the media is
copies of one ten-second generated clip named after each title.

- **What's in it:** `tools/screenshot-library.json`: 24 films, 5 shows with their first season,
  a collection (TMDB adds its own too, such as Dune's), three extra profiles (one a Kids profile), what's part-way through, finished and on
  the watchlist, and the featured film and show (Dune: Part Two, Breaking Bad) to photograph
  for a title page. Change the library there, not in an app.
- **Credit:** every README with these pictures says "Artwork and details from TMDB. Films and
  shows belong to their owners; Arc doesn't include any media." TMDB's terms ask for that credit.
- **Store listings** (Google Play, the App Store, LG and Samsung stores, Roku) are a separate job:
  they will need artwork whose licence has been checked, not these titles or TMDB's artwork.

## Running it

It runs from this repository (in another app's session, check it out beside the app as
`../arc-media-server`). It needs Go, FFmpeg, Node and a web build, and a TMDB API key in the
environment as `AMS_TMDB_API_KEY` (a TheTVDB key in `AMS_TVDB_API_KEY` too, for TheTVDB's episode
order; otherwise TMDB's is used). The Default cloud environment has both. The keys are passed
only to the scratch server; they are never written to a file, printed or committed. If there is
no key, ask the owner to add one as an environment secret; never ask for it in chat.

```sh
cd web && npm install && npm run build
npx playwright install chromium        # or add --chrome <path to Chrome/Chromium>
node tools/screenshots.mjs             # the web README pictures, into docs/screenshots
node tools/screenshots.mjs --serve     # leave Arc running for an app; Ctrl+C stops it
```

Both make the media (copies of one ten-second generated clip, named after the titles), build and
start a throwaway Arc on `127.0.0.1:18299` with its own data folder and `-config none`, set it up,
wait for TMDB matching to finish (a few minutes) and add the profiles and watching. Nothing of
yours is touched: never point it, or an app being photographed, at the owner's server, data or
media. Without `--serve` it photographs the web app and stops; with `--serve` it prints:

- the address, `http://127.0.0.1:18299` (`--port <n>` for another);
- the sign-in: admin profile **Jordan**, household password **screenshots-only** (a throwaway
  password for a throwaway server). The web app on the same machine is signed in without one;
- the library ids and the featured film and show ids, for opening those pages directly.

`--state <file>` also writes those as JSON (no keys) for a script to read. `--keep` leaves the
temporary folder afterwards; `--media <folder>` uses a library prepared beforehand.

### Signing an app in

- **Password:** the app's ordinary sign-in with the household password (`POST /api/v1/auth/login`),
  then pick Jordan (or Sam, Alex or Kids for the profile picker).
- **Link code:** let the app show its code, then, while `--serve` is running:
  `node tools/screenshots.mjs --approve <code>` (add `--port` if you changed it). The web app's
  Link a Device page (`/link`) works too.

## Each app

The server only listens on this machine, so photograph an emulator, simulator or desktop build
running here, not a real TV. Hardware screenshots are the owner's.

| App | How |
|---|---|
| Web (this repo) | `node tools/screenshots.mjs` writes `docs/screenshots/*.png` (1600×1000, dark). |
| arc-android | Emulator: the server is `http://10.0.2.2:18299`; capture with `adb exec-out screencap -p`. Roborazzi tests that need data use responses recorded from this server, not invented titles. |
| arc-apple | Simulator on a Mac: `http://127.0.0.1:18299`; `xcrun simctl io booted screenshot`. |
| arc-kodi | Kodi on the same machine: `http://127.0.0.1:18299`; Kodi's screenshot key. |
| arc-roku | brs-desktop on the same machine: `http://127.0.0.1:18299`. |
| arc-tv | The browser build (`npm run dev`) pointed at `http://127.0.0.1:18299`, at TV resolution. |

Photograph the same pages where the app has them: Home, the featured film and show, the Movies
library, and the profile picker. Change notes can show any page, from the same library.
