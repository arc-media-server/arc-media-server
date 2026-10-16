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

- Movie library scanning (Plex/Jellyfin-style naming, scene release names, `{tmdb-123}` / `{imdb-tt…}` tags,
  multi-part `cd1`/`part2` files; extras and trailers are skipped)
- Media info via ffprobe (container, duration, codecs, resolution, audio and subtitle tracks)
- TMDB metadata matching
- Missing-file detection that keeps items and their history (e.g. when a drive is offline)
- Direct-play streaming with HTTP range requests
- SQLite database, with the schema already in place for TV episode orderings, users and watch state

## Getting started

Requirements: Go 1.24+ and FFmpeg (for `ffprobe`).

```sh
cp ams.example.yaml ams.yaml   # then edit library paths and add your TMDB key
go run ./cmd/ams
```

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
| GET | `/api/items/{id}/images/{primary\|backdrop}` | Artwork |

## Layout

```
cmd/ams/            entrypoint
internal/config/    YAML config
internal/store/     SQLite, migrations, queries
internal/probe/     ffprobe wrapper
internal/scanner/   folder walking and filename parsing
internal/metadata/  TMDB client
internal/api/       REST API
```
