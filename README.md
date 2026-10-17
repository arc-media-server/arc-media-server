# Arc Media Server (AMS)

An open source media server written in Go, built from the ground up.

**Principles**

- **Your files are never touched.** AMS only reads your media. It never renames, moves or writes
  next to your files, so Sonarr, Radarr and anything else relying on your folder layout keep working.
- **Display order is separate from storage order.** A series can be stored in TVDB aired order and
  shown in a TMDB episode group order (per series, and eventually per user) without changing anything on disk.
- **Its own apps, and an open API.** Arc has its own web app (native TV and phone apps are planned), and its API is
  documented so anyone can build their own client.

## Status

Early development. Working today:

- TV show scanning: `S01E02`, multi-episode files, `1x02`, date-based shows, specials, anime absolute numbering
- **Display orderings**: every TVDB order (aired, DVD, absolute) and TMDB order (aired plus all episode
  groups) per series, mapped onto your files whichever way they're named. Pick one per series, make
  custom orders, and manually place episodes. Episodes an order doesn't place are listed, never hidden.
  "Compare Orders" shows where each of your episodes lands in another order before you switch
- Movie library scanning (Plex/Jellyfin-style naming, scene release names, `{tmdb-123}` / `{imdb-tt…}` tags,
  multi-part `cd1`/`part2` files; samples are skipped)
- Extras: trailers, featurettes, behind the scenes, deleted scenes, interviews and shorts (in
  `Extras/`, `Trailers/`… folders or named `<movie>-trailer.mkv`) shown with their movie or show
- Media info via ffprobe (container, duration, codecs, resolution, audio and subtitle tracks)
- TMDB and TheTVDB metadata matching, with genres, cast and crew (with photos), studio or network,
  tagline, score and age ratings for your region
- "Fix Match": see exactly what Arc read from a file name and searched for, then search TMDB or
  TheTVDB and pick the right movie or show
- Edit any title, date, description or genres; edited fields (and artwork you choose to keep) are
  locked so refreshes never undo them
- Episode numbering: TV libraries read file names in TMDB's numbering by default (as Plex does), or
  TheTVDB's aired, DVD or absolute order; any show can override its library in Edit Details.
  Switching keeps episodes and their watch history
- Missing-file detection that keeps items and their history (e.g. when a drive is offline)
- Folder watching: new, changed and removed files are picked up within seconds, plus an hourly
  rescan for folders (like network shares) that don't report changes
- Playback that adapts to the browser: files it can play are sent as they are (direct play, with
  range requests for seeking); otherwise FFmpeg repackages them (video untouched, e.g. MKV or AC-3
  audio) or converts the video to H.264 (e.g. HEVC, 10-bit H.264), as HLS. Seeking anywhere works:
  within what's converted it's instant, beyond it the conversion restarts there. The player's Info
  menu says which method is in use and why
- Playback compatibility (Library Health): for this browser, Chrome, Edge, Firefox, Safari or
  iPhone/iPad, how many titles in each library play directly, are repackaged or are converted, why,
  and which ones need converting
- Converting uses the graphics card when there is one (NVIDIA NVENC, Intel Quick Sync, AMD AMF, Apple
  VideoToolbox or VA-API, found by a quick test at start-up), falling back to the processor, and maps
  HDR to SDR so converted HDR doesn't look washed out. Both are on by default, as in Plex (Settings >
  Converting)
- Collections: films TMDB groups together (a trilogy) become a collection by themselves once you have
  enough of them (2 by default, set per library); make your own by picking titles (in any order) or
  as smart collections that hold everything matching a filter. Each library has a Collections view,
  and a title's page shows the rest of its collection
- Playlists (each profile's own): movies, episodes, or a whole show (its episodes in your episode
  order, specials last). Play or shuffle, drag to arrange; the player carries on to the next one
  after a 10-second Up Next countdown, with Next and Previous buttons (Shift+N / Shift+P)
- Search everything at once: movies, shows, episodes, people, collections and genres. Accents and
  case don't matter and small typos are forgiven ("matirx" finds The Matrix). Cast and crew open a
  page with everything in your libraries they're in
- Versions and editions: several files of one film (4K and 1080p, or `{edition-Director's Cut}`,
  "Extended", "Theatrical" in the name) are one title. Pick the edition on the film's page; Arc plays
  the best version for the screen (one that needs no conversion, then the sharpest that fits), and
  the player's Version menu switches where you are. Library Health only reports real duplicates
- Scrubbing previews (per library, off by default as in Plex): a picture every 10 seconds shows while
  dragging along the timeline. They're made in the background from keyframes only (about 5 MB per film),
  a title that's being watched goes first, and turning the setting off deletes them
- Subtitles: inside the file or beside it (`Movie.en.srt`, `Movie.eng.forced.srt`, `.ass`, `.vtt`;
  old Windows-1252 files are read correctly). Text subtitles are drawn by the player in three sizes;
  picture subtitles (Blu-ray, DVD) are burned into the video. Each profile's last choice of
  language turns subtitles on by themselves next time, and forced subtitles show for foreign dialogue
- Find subtitles online from the player (Subtitles > Find Online…) on OpenSubtitles.com, matched by
  TMDB/IMDb id and the file's hash (exact matches first). Needs a free OpenSubtitles API key in
  Settings; an account allows more downloads a day. Downloads are kept in Arc's data folder
- A player with its own controls: audio track choice, playback speed, volume, full screen, and
  keyboard shortcuts (Space, ←/→, J/L, ↑/↓, M, C, F, < and >, 0–9)
- Artwork cached locally after each scan, so it loads fast and works offline
- Local media assets (read only): poster.jpg / folder.jpg / fanart.jpg / clearlogo.png and `<file>-poster.jpg`,
  `<file>-fanart.jpg`, `<file>-clearlogo.png`, `<episode>-thumb.jpg` are shown instead of online artwork; ids in Kodi,
  Radarr and Sonarr `.nfo` files (movie.nfo, tvshow.nfo, `<file>.nfo`) make identification exact
- Clear logos: movie and series pages show the title's logo artwork (from TMDB, or a local
  clearlogo.png / logo.png) instead of plain text, and the cast sits right under the summary
- Artwork, in the Edit dialog (Details, Poster, Background and Logo tabs, as in Plex): pick any poster, background or logo TMDB has for a title, or upload your own (kept in
  the data folder; media files are never touched). "Automatic" goes back to local or online artwork
- Automatic daily database backups (plus one before every upgrade), with one-click restore that
  saves the current state first
- Profiles like Plex Home: a "Who's watching?" screen, optional PINs, an admin profile that manages
  everything, and optional sign-in without the password on your home network
- Dashboard (admin): who is watching what, on which device, how far in and how it is being
  played; stop a stream with an optional message for the viewer; the recent server log, filtered
  by level and easy to copy
- Insights (admin): watch time over time, the most watched movies, shows and genres, the busiest
  days and hours, and watching by profile, library, device and how it played; full watch history
  per profile and library; and storage: the space each library takes, the biggest titles, and the
  space taken by titles nobody has watched. A viewing counts once it has played for a minute, and
  history keeps what was watched even if it later leaves the library
- Your Year in Review (every profile, from the profile menu): hours watched, top movies and shows,
  genres, favourite actors, busiest month, day and hour, longest streak and biggest binge
- Parental controls: limit a profile to titles rated for ages up to 7, 12 or 15 (age ratings for
  your region); anything else is hidden from it everywhere
- Watch state per profile: resume where you left off, watched ticks and unwatched counts, play
  counts, mark a movie, episode, season or show watched; Continue Watching and Up Next on Home
  (Up Next follows the episode order you chose)
- A home page each profile arranges for itself ("Edit Home", top right of Home or in the profile
  menu): drag rows into order, hide rows, choose whether Up Next is part of Continue Watching (as
  in Plex, the default) or its own row, and
  add your own rows from filters (library, genre, decade, quality, unwatched, added this week/month/
  year, length, rating, any sort including a daily shuffle) with a live preview
- Each home row can use posters or landscape thumbnails (Continue Watching uses thumbnails by
  default); rows with episodes or seasons can show the show's poster or the season's. A TV library's Recently Added groups new episodes as Plex does: a new show is one card,
  several new episodes of a season show as that season with its poster, a single new episode as
  itself. Season posters come from TMDB or a `season01-poster.jpg` in the show's folder
- Seasonal rows: romance before Valentine's Day, horror through October, Christmas films in December
- Hide Spoilers (per profile, in the profile menu): stills are blurred and descriptions hidden for
  episodes you haven't started
- Appearance per profile (profile menu): Automatic (follows the device), Dark or Light
- Set up and configured entirely in the web app: first-run setup, Settings (API keys, ffprobe found
  automatically, network access and port)
- Libraries managed in the app, with a folder browser and a preview of what Arc finds (and what it
  can't identify, and why) before anything is added
- **Web app**: home with featured title and recently added rows, library grids with sorting and
  scanning, movie and series pages with file details, an episode-order picker, in-browser playback,
  search, and a Library Health page (unidentified titles, unreadable, unrecognised and missing
  files, duplicates, missing episodes). Light and dark mode follow your system
- SQLite database, with the schema already in place for users and per-user watch state

## Getting started

Requirements: Go 1.27+, Node.js 22+ (to build the web app) and FFmpeg (for `ffprobe`).

```sh
cd web && npm install && npm run build && cd ..   # builds the web app into the binary
go run ./cmd/ams
```

Then open http://127.0.0.1:8099. Arc walks you through the rest: your admin profile and password,
your free TMDB and TheTVDB API keys, and your first library (choose its folders, see a preview of
what Arc finds, then add it). There are no config files; everything is set in the app.

- `-data <folder>` (or `AMS_DATA`) chooses where Arc keeps its database, artwork cache, uploaded artwork and backups
  (default: `data` in the current folder).
- Forgot the password? Run `ams -reset-password` on the server for a new one.
- `AMS_TMDB_API_KEY`, `AMS_TVDB_API_KEY`, `AMS_FFPROBE` and `AMS_LISTEN` override the app's
  settings, for containers.

### Working on the web app

Run AMS, then in another terminal `cd web && npm run dev` and open the address Vite prints.
Changes appear instantly; API calls are forwarded to the running server.

The web app is React + TypeScript, styled with Tailwind CSS, using Radix UI primitives,
Motion for animation and TanStack Query for data loading.

## API

| Method | Path | |
|---|---|---|
| GET | `/api/health` | Server name and version |
| GET | `/api/auth/status` | Setup needed? Signed in? Current profile and the profiles to pick from |
| POST | `/api/auth/setup` | First run: create the admin profile (`name`, `password`, `local_bypass`, API keys) |
| POST | `/api/auth/login` | Sign in with the password |
| POST / DELETE | `/api/auth/profile` | Switch to a profile (`id`, `pin` or `password`) / back to the picker |
| POST | `/api/auth/logout` | Sign out |
| POST | `/api/auth/password` | Change the password (admin) |
| GET / POST | `/api/profiles` | Profiles (admin) / add one: `{"name", "color", "pin"}` |
| PATCH / DELETE | `/api/profiles/{id}` | Edit (a `pin` of `""` removes it) / remove a profile |
| GET / PATCH | `/api/settings` | Server settings: API keys, ffprobe, sign-in at home, port, network (admin) |

Everything except `/api/health` and `/api/auth/*` needs a signed-in profile (a session cookie);
managing libraries, settings, profiles, backups and metadata needs the admin profile.

| Method | Path | |
|---|---|---|
| GET | `/api/libraries` | Libraries with their size and scan status |
| POST | `/api/libraries` | Add a library: `{"name", "kind": "movies"\|"shows", "paths": [...], "source_ordering", "trickplay"}` |
| PATCH | `/api/libraries/{id}` | Rename it, change its folders or episode numbering |
| DELETE | `/api/libraries/{id}` | Remove a library from Arc (backed up first; files untouched) |
| POST | `/api/library-previews` | Preview what a library with these folders would contain (same body as adding) |
| GET | `/api/library-previews/{id}` | The preview's progress and results |
| GET | `/api/folders[?path=]` | Folders inside a folder (drives when no path), for choosing library folders |
| POST | `/api/libraries/{id}/scan` | Start a scan |
| GET | `/api/libraries/{id}/scan` | Scan progress |
| GET | `/api/items?library_id=&kind=&q=&sort=&genre=&decade=&resolution=&unwatched=1&added_within=&min_runtime=&max_runtime=&min_rating=&about=&limit=&offset=` | Browse, filter and search. Sorts: title, added, year, release, rating, runtime, watched, shuffle (a new order each day). Resolutions: 4k, 1080, 720, sd. `added_within` is days, runtimes are minutes, `about` is comma-separated words to find in the title, tagline or description |
| GET | `/api/libraries/{id}/facets` | Genres, decades and qualities to filter a library by, with counts |
| GET | `/api/items/{id}` | Item details with files and streams |
| PATCH | `/api/items/{id}` | Edit metadata (incl. `genres`); edited fields are locked. `locked_fields` replaces the locked set. For a show, `source_ordering` sets its episode numbering (`""` for its library's) |
| POST | `/api/items/{id}/refresh` | Re-fetch metadata now (locked fields are kept) |
| GET | `/api/items/{id}/match` | How the item was identified: what was read from its names and every search made |
| GET | `/api/items/{id}/match/search?provider=tmdb\|tvdb&q=&year=` | Search a provider for the right match |
| PUT | `/api/items/{id}/match` | Use a match: `{"provider": "tmdb", "id": 603}` |
| GET | `/api/items/{id}/stream[?file=]` | Direct-play the original file |
| POST | `/api/items/{id}/playback` | Decide how to play: `{"caps": {"containers", "video", "audio", "max_height"}, "file_id" or "edition", "audio_stream", "start_ms", "burn_subtitle"}` → `{"method": "direct|remux|transcode", "reasons", "url", "hls", "offset_ms", "audio_tracks", "subtitle_tracks", "versions", ...}`; starting again ends the player's previous conversion |
| GET | `/api/items/{id}/trickplay?file=` | A file's scrubbing previews `{"info": {"interval_ms", "width", "height", "columns", "rows", "count"}, "sheets": [urls]}`; 404 until made |
| GET | `/api/items/{id}/subtitles?file=` | A file's subtitle tracks (in it, beside it, downloaded) |
| GET | `/api/items/{id}/subtitles/search?file=&languages=eng` | Find subtitles on OpenSubtitles |
| POST | `/api/items/{id}/subtitles/download` | Download one: `{"file", "result", "language", "forced", "hearing_impaired"}` |
| GET | `/api/items/{id}/subtitles/{track}?file=` | A text subtitle track's cues `{"start_ms", "end_ms", "text"}` (tracks are listed in the playback plan; picture ones are burned in with `"burn_subtitle"`) |
| GET | `/api/transcode/{session}/index.m3u8` | A conversion's HLS playlist (and its `init.mp4` and `sNNNNN.m4s` segments) |
| POST | `/api/transcode/{session}/ping` | Keep a paused player's conversion alive (they end after 2 minutes unused) |
| DELETE | `/api/transcode/{session}` | End a conversion |
| PUT | `/api/items/{id}/progress` | Player progress `{"position_ms", "duration_ms", "state": "playing\|paused\|stopped"}`; past 90% counts as watched. 409 means the admin stopped playback |
| PUT | `/api/items/{id}/played` | Mark watched/unwatched `{"played": true}`; for a show, add `season_number` for one season |
| GET / POST | `/api/playlists` | The profile's playlists; make one `{"title", "item_ids"}` |
| GET / PATCH / DELETE | `/api/playlists/{id}` | A playlist and its entries; rename `{"title"}`; delete |
| POST | `/api/playlists/{id}/items` | Add `{"item_ids"}` (movies, episodes, or shows for all their episodes) |
| DELETE / PUT | `/api/playlists/{id}/entries/{entry}`, `/order` | Remove an entry; order them `{"entry_ids"}` |
| GET | `/api/home/continue?with_up_next=1` | What the profile is part-way through; with `with_up_next=1`, also the next episode of each show, most recent first (one per show) |
| GET | `/api/libraries/{id}/recent-shows` | A TV library's Recently Added, grouped: `[{"type": "series"\|"season"\|"episode", "item", "season_number", "episodes", "season_poster"}]` |
| GET | `/api/home/next-up` | The next episode of each show the profile is watching, in their episode order |
| GET | `/api/me/prefs` | The profile's own preferences: `{"hide_spoilers", "theme": ""\|"dark"\|"light", "split_up_next", "home": [rows]}` (an empty `home` is the default layout) |
| GET | `/api/me/year?year=` | The profile's year in review (this year by default), and the years it has history for |
| PATCH | `/api/me/prefs` | Change them; a home row is `{"id": "continue"\|"next-up"\|"seasonal"\|"recent:<library>"\|"custom:<any>", "hidden", "title", "filter"}` |
| GET | `/api/items/{id}/images/{primary\|backdrop}` | Artwork, served from the local cache |
| GET | `/api/items/{id}/artwork` | Posters and backgrounds to choose from, plus what's in use, chosen or local |
| PUT | `/api/items/{id}/artwork` | Choose one: `{"type": "poster\|backdrop", "path": "/abc.jpg"}`; `"path": null` goes back to automatic |
| POST | `/api/items/{id}/artwork/{poster\|backdrop}` | Upload your own (JPEG, PNG or WebP body, up to 20 MB) |
| GET | `/api/search?q=` | Search everything: `{"movies", "shows", "episodes", "people", "collections", "genres"}`, best matches first; ignores accents and case, and forgives small typos |
| GET | `/api/people/{id}` | Someone in the cast or crew and the titles in the libraries they're in: `{"person", "credits": [{"item", "roles", "character"}]}` |
| GET | `/api/people/{id}/image` | A cast or crew member's photo |
| GET | `/api/extras/{id}/stream` | Play a trailer or other extra |
| GET | `/api/libraries/{id}/unmatched` | Items that need attention |
| GET | `/api/libraries/{id}/health` | Library health: unidentified items, unreadable/unrecognised/missing files, duplicates, missing episodes |
| POST | `/api/libraries/{id}/compatibility` | How a library plays on a device: `{"caps": {...}}` → `{"total", "counts": {"direct", "remux", "transcode"}, "reasons", "converted": [{"item", "reasons"}]}` |
| GET | `/api/libraries/{id}/collections` | A library's collections |
| GET | `/api/collections/{id}` | A collection and its titles |
| POST | `/api/collections` | Make one: `{"library_id", "kind": "manual"|"smart", "title", "overview", "filter", "item_ids"}` |
| PATCH / DELETE | `/api/collections/{id}` | Change or delete one (automatic ones are hidden) |
| POST / DELETE / PUT | `/api/collections/{id}/items[/{item}]`, `/order` | Add, remove and order a manual collection's titles |
| GET | `/api/health/libraries` | Count of problems needing the user, per library |
| POST | `/api/libraries/{id}/empty-trash` | Forget titles whose files are all gone (backed up first) |
| DELETE | `/api/items/{id}` | Forget one title whose files are all gone (backed up first) |
| GET | `/api/series/{id}/seasons/{n}/image` | A season's poster |
| GET | `/api/series/{id}/seasons[?ordering=&missing=1]` | Episodes arranged by an ordering (display ordering by default) |
| GET | `/api/series/{id}/orderings` | Available orderings and the one in use |
| PUT | `/api/series/{id}/display-ordering` | `{"ordering_id": 5}` or `null` to reset |
| POST | `/api/series/{id}/orderings` | Custom ordering: `{"name": "...", "copy_from": 5}` |
| DELETE | `/api/series/{id}/orderings/{oid}` | Delete a custom ordering |
| PUT | `/api/series/{id}/orderings/{oid}/entries/{episode}` | Place an episode: `{"season_number": 1, "episode_number": 3}` |
| DELETE | `/api/series/{id}/orderings/{oid}/entries/{episode}` | Remove a manual placement |
| POST | `/api/series/{id}/refresh[?deep=1]` | Re-fetch metadata and orderings |
| GET | `/api/playing` | What is being watched right now: profile, title, device, position, state |
| POST | `/api/playing/{id}/stop` | Stop a playback, with an optional `{"message": "..."}` for the viewer |
| GET | `/api/logs?level=info\|warn\|error\|debug&after=<seq>` | Recent server log lines, oldest first |
| GET | `/api/stats?days=30&profile=&library=` | Watch activity (`days=0` for all time): totals, timeline, top movies, shows and genres, by profile, library, device and method, and watch time by weekday and hour |
| GET | `/api/stats/history?profile=&library=&before=&limit=50` | Watch history, newest first; pass the last id as `before` for more |
| GET | `/api/stats/storage?library=` | Space per library, the biggest titles, and titles nobody has watched |
| GET | `/api/backups` | Database backups, newest first |
| POST | `/api/backups` | Back up now |
| POST | `/api/backups/{name}/restore` | Restore a backup (the current state is backed up first) |
| DELETE | `/api/backups/{name}` | Delete a backup |

## Layout

```
cmd/ams/            entrypoint
internal/config/    YAML config
internal/store/     SQLite, migrations, queries
internal/probe/     ffprobe wrapper
internal/scanner/   folder walking and filename parsing
internal/metadata/  TMDB and TheTVDB clients
internal/api/       REST API
internal/webui/     serves the built web app (embedded)
web/                web app source (React + TypeScript)
```
