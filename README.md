# Arc Media Server

A self-hosted media server for your movies, TV shows and home videos, in the spirit of Plex and
Jellyfin. Written in Go, with its own web app built in. Point it at your folders and it does the
rest: identifies everything, fetches artwork and details, and plays it in any browser.

> **Status: private alpha.** Arc runs day to day on a home server (Unraid, Docker), and work is now
> focused on stability and release readiness. Native TV and phone apps are planned. Expect rough
> edges, and keep backups of your Arc data folder (your media is never touched).

## Why Arc

- **Your files are never touched.** Arc only reads your media. It never renames, moves, tags or
  writes next to your files, so Sonarr, Radarr and anything else relying on your folders keep working.
- **Episode order is yours to choose.** Files stay numbered however they are on disk, while each
  show (and each profile) can be watched in any order TMDB or TheTVDB knows: aired, DVD, absolute,
  story order, anime cours, or one you make yourself.
- **Simple by default.** Good automatic choices, settings in the app instead of config files, and
  advanced options kept out of the way.
- **Nothing silently disappears.** Unidentified titles, unplaced episodes and missing files are
  shown and listed, and titles on an offline drive keep their watch history.
- **Its own apps and an open API.** The web app uses the same documented API anyone can build on.

## Features

**Libraries and metadata**
- Movie, TV and Other videos libraries, added in the app with a folder browser. Scanning runs in
  the background: titles show with posters within seconds, file details follow.
- Matching from TMDB and TheTVDB with Plex/Jellyfin-style naming, scene release names and
  `{tmdb-123}` / `{imdb-tt…}` tags. Fix Match shows exactly what Arc read and searched for.
- Cast and crew, studios, scores (IMDb, Rotten Tomatoes, Metacritic), age ratings for each
  library's region, and metadata in 35+ languages, set per library.
- Edit anything; edited fields are locked so refreshes never undo them. Choose or upload
  posters, backgrounds, logos and season posters. Local artwork and NFO IDs are respected.
- Versions and editions (4K and 1080p, Director's Cut), extras and trailers, theme music.
- Collections (automatic from TMDB, manual and smart), labels, merge and split titles or
  shows, and a folder view for home videos.
- Library Health: unidentified titles, unreadable or missing files, duplicates, missing
  episodes, skip-marker coverage and how each library plays in each browser.

**Playback**
- Direct play when the browser can, otherwise remuxing or conversion to HLS, with hardware
  encoding (NVIDIA, Intel Quick Sync, AMD, VA-API, VideoToolbox) and HDR tone mapping.
- Automatic quality that adapts to the connection, optimized versions for weak devices, and
  remote bandwidth limits.
- Subtitles in the file, beside it or from OpenSubtitles, with per-profile styling and timing.
- Skip Intro, Recap and Credits from chapters, IntroDB/TheIntroDB, or audio and picture
  detection overnight. Auto-Play, Up Next, chapters, Picture-in-Picture, a playback queue.
- Chromecast and AirPlay from the web player.
- Dolby Vision, HDR10+, Atmos and DTS:X labels from the source files.

**Profiles and home**
- Plex Home-style profiles with optional PINs, a "Who's watching?" screen, and sign-in without
  a password at home (including Tailscale/VPN networks).
- Friends with invite links, library access per profile and parental controls by age.
- Each profile arranges its own Home: Continue Watching, Up Next, custom and seasonal rows,
  collections and playlists. Watchlist, ratings, recommendations, New for You and Year in Review.
- Playlists and smart playlists, search with typo forgiveness, person pages.

**Running the server**
- Remote access with UPnP and HTTPS (your own certificate or Let's Encrypt) on one port, and
  trusted reverse proxies.
- Dashboard with now playing, resource graphs and the server log; Insights for watch history
  and storage; an Activity menu for background work.
- Scheduled maintenance in a quiet window: metadata refresh, previews, intro detection,
  database backups and optimization, cache clean-up and optional empty trash.
- Daily database backups with one-click restore, webhooks (Discord, Slack, JSON) and redacted
  diagnostic downloads.
- Moving in from Plex or Jellyfin: watch state, resume points, ratings, watchlists, playlists
  and collections.
- The web app installs to a phone's home screen, and TVs sign in with a short link code.

## Running with Docker

An image for amd64 and arm64, with FFmpeg included, is published to
`ghcr.io/jonzauk/arc-media-server`. `latest` follows `main` (the development channel); version
tags pass the full verification suite. See [image publishing](docs/IMAGE-PUBLISHING.md).

1. While the repository is private, sign in to the registry once: `docker login ghcr.io` (your
   GitHub name, and a personal access token with `read:packages` as the password).
2. Copy [`docker-compose.yml`](docker-compose.yml), set your media folders (mounted read-only)
   and run `docker compose up -d`.
3. Open `http://<that computer>:8099`. First-run setup creates your admin profile and takes
   your free [TMDB](https://www.themoviedb.org/settings/api) and
   [TheTVDB](https://thetvdb.com/api-information) API keys. Then add `/media/movies` and
   `/media/tv` as libraries.

Notes:
- Everything Arc keeps (database, artwork, uploads, backups, logs) lives in `/config`.
- Host networking is recommended on Linux so Arc sees devices' real addresses (for sign-in at
  home) and can open the router port. Docker Desktop on Windows or Mac uses `ports: ["8099:8099"]`.
- For hardware encoding pass `/dev/dri` (Intel/AMD) or use the NVIDIA Container Toolkit; both are
  in the example file.
- To run as a non-root user, set `user: "UID:GID"` and make `/config` writable by it. `PUID`/`PGID`
  aren't read.
- Keep `/config` on local storage; media can be on network shares. SQLite can't share a database
  over a network filesystem.
- Behind a reverse proxy (SWAG, Nginx Proxy Manager, Caddy…), have it set `X-Forwarded-For` and
  `X-Forwarded-Proto`, and add its address under Settings › Network › Trusted proxies so people at
  home still count as home.

### Backups and upgrades

Built-in backups cover the database. Before upgrading, stop Arc and copy the whole `/config`
folder somewhere safe; it also holds uploaded artwork and certificates. Arc backs up its database
automatically before any upgrade that changes it, and an older Arc refuses to open a newer
database. The [recovery guide](docs/RECOVERY.md) covers restoring and rolling back.

## Building from source

You need Go 1.27+, Node.js 22+ and FFmpeg (`ffprobe` and `ffmpeg`).

```sh
cd web && npm install && npm run build && cd ..   # the web app is embedded in the binary
go run ./cmd/ams                                  # serves http://127.0.0.1:8099
```

- `-data <folder>` (or `AMS_DATA`) sets where Arc keeps its data (default `./data`).
- `ams -reset-password` sets a new admin password if you've forgotten it.
- `AMS_TMDB_API_KEY`, `AMS_TVDB_API_KEY`, `AMS_FFPROBE` and `AMS_LISTEN` override the app's
  settings, for containers.
- Troubleshooting: Settings › Maintenance & diagnostics turns on detailed logging for 30 minutes
  and downloads redacted logs. `-debug` does the same at startup.

### Development

- `cd web && npm run dev` serves the web app with hot reload, forwarding API calls to a running
  server.
- `go vet ./... && go test ./...` for the server. `cd web && npm run test:e2e` runs the browser
  journeys (Playwright Chromium) against a real server with generated clips, local metadata
  fixtures and a fresh temporary database each test, never your own data.
- `go run ./tools/testmedia -out ../ams-test-media` makes a small test library, including shows
  with tricky episode orders.
- UI text goes through react-i18next; see [translation](docs/TRANSLATION.md).
- [ROADMAP.md](ROADMAP.md) has the plan and [docs/READINESS.md](docs/READINESS.md) the release
  checklist.

The server is Go with SQLite (pure Go, no CGO). The web app is React, TypeScript, Tailwind CSS,
Radix UI, Motion and TanStack Query.

## API

Everything the web app does goes through a documented REST API, with stable `/api/v1/...` paths,
Bearer tokens for apps and link codes for TVs. The full reference is [docs/API.md](docs/API.md),
generated from the code, and a running server describes itself at `/api/openapi.json`.

## Credits

Movie and TV information and images come from [TMDB](https://www.themoviedb.org) and
[TheTVDB](https://thetvdb.com), credited in the app wherever their information is shown.
This product uses the TMDB API but is not endorsed or certified by TMDB.
Skip markers can also come from [IntroDB](https://introdb.app) and
[TheIntroDB](https://theintrodb.org), subtitles from [OpenSubtitles.com](https://www.opensubtitles.com),
and critics' scores from [OMDb](https://www.omdbapi.com).

## Licence

Copyright © 2026 The Arc Media Server Authors.

Arc is free software: you can redistribute it and/or modify it under the terms of the
[GNU Affero General Public License, version 3](LICENSE). In short: you can use, change and share
Arc freely; if you distribute a modified version, or run one as a service for other people, you
must make your source code available under the same licence. Apps that only talk to Arc's API
are not affected and can use any licence.
