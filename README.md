# Arc Media Server (AMS)

An open source media server written in Go, built from the ground up.

**Principles**

- **Your files are never touched.** AMS only reads your media. It never renames, moves or writes
  next to your files, so Sonarr, Radarr and anything else relying on your folder layout keep working.
- **Display order is separate from storage order.** A series can be stored in TVDB aired order and
  shown in a TMDB episode group order (per series, and eventually per user) without changing anything on disk.
- **Clients come for free.** A Jellyfin-compatible API layer is planned so existing Jellyfin apps can connect.

## Status

Early development. Working today:

- TV show scanning: `S01E02`, multi-episode files, `1x02`, date-based shows, specials, anime absolute numbering
- **Display orderings**: every TVDB order (aired, DVD, absolute) and TMDB order (aired plus all episode
  groups) per series, mapped onto your files whichever way they're named. Pick one per series, make
  custom orders, and manually place episodes. Episodes an order doesn't place are listed, never hidden
- Movie library scanning (Plex/Jellyfin-style naming, scene release names, `{tmdb-123}` / `{imdb-tt…}` tags,
  multi-part `cd1`/`part2` files; extras and trailers are skipped)
- Media info via ffprobe (container, duration, codecs, resolution, audio and subtitle tracks)
- TMDB and TheTVDB metadata matching
- Missing-file detection that keeps items and their history (e.g. when a drive is offline)
- Direct-play streaming with HTTP range requests
- Artwork cached locally after each scan, so it loads fast and works offline
- **Web app**: home with featured title and recently added rows, library grids with sorting and
  scanning, movie and series pages with file details, an episode-order picker, in-browser playback,
  search, and a Needs Attention list. Light and dark mode follow your system
- SQLite database, with the schema already in place for users and per-user watch state

## Getting started

Requirements: Go 1.24+, Node.js 22+ (to build the web app) and FFmpeg (for `ffprobe`).

```sh
cp ams.example.yaml ams.yaml   # then edit library paths and add your TMDB/TVDB keys
cd web && npm install && npm run build && cd ..   # builds the web app into the binary
go run ./cmd/ams
```

Then open http://127.0.0.1:8099.

### Working on the web app

Run AMS, then in another terminal `cd web && npm run dev` and open the address Vite prints.
Changes appear instantly; API calls are forwarded to the running server.

The web app is React + TypeScript, styled with Tailwind CSS, using Radix UI primitives,
Motion for animation and TanStack Query for data loading.

## API

| Method | Path | |
|---|---|---|
| GET | `/api/health` | Server name and version |
| GET | `/api/libraries` | Libraries with scan status |
| POST | `/api/libraries/{id}/scan` | Start a scan |
| GET | `/api/libraries/{id}/scan` | Scan progress |
| GET | `/api/items?library_id=&kind=&q=&sort=title\|added\|year&limit=&offset=` | Browse and search |
| GET | `/api/items/{id}` | Item details with files and streams |
| GET | `/api/items/{id}/stream[?file=]` | Direct-play the original file |
| GET | `/api/items/{id}/images/{primary\|backdrop}` | Artwork, served from the local cache |
| GET | `/api/libraries/{id}/unmatched` | Items that need attention |
| GET | `/api/series/{id}/seasons[?ordering=&missing=1]` | Episodes arranged by an ordering (display ordering by default) |
| GET | `/api/series/{id}/orderings` | Available orderings and the one in use |
| PUT | `/api/series/{id}/display-ordering` | `{"ordering_id": 5}` or `null` to reset |
| POST | `/api/series/{id}/orderings` | Custom ordering: `{"name": "...", "copy_from": 5}` |
| DELETE | `/api/series/{id}/orderings/{oid}` | Delete a custom ordering |
| PUT | `/api/series/{id}/orderings/{oid}/entries/{episode}` | Place an episode: `{"season_number": 1, "episode_number": 3}` |
| DELETE | `/api/series/{id}/orderings/{oid}/entries/{episode}` | Remove a manual placement |
| POST | `/api/series/{id}/refresh[?deep=1]` | Re-fetch metadata and orderings |

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
