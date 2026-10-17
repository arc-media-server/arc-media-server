# Arc Media Server Roadmap

## Vision

Everything Jellyfin can do, with the ease and polish of Apple software. Simple by default,
powerful when you need it.

## Principles

1. **Never touch the user's files.** AMS reads media; it never renames, moves, tags or writes beside it.
2. **Ease over options.** Sensible defaults, no setting without a reason, nothing important buried.
3. **Built in, not bolted on.** No plugin system. Features live in AMS, are maintained with it and
   ship with it. Anyone can contribute a feature to the project instead.
4. **Storage order is not display order.** How files are named on disk never dictates how they're shown.
5. **Work offline.** Online services enrich AMS, but it must keep working when they're unavailable.

## Milestones

### ✅ 1. Core (movies)
- [x] SQLite database with migrations
- [x] Movie scanning with robust filename parsing; extras and trailers skipped
- [x] Media info via ffprobe
- [x] TMDB matching
- [x] Missing-file detection that keeps history
- [x] Direct-play streaming with range requests
- [x] Native REST API

### ✅ 2. TV shows and display orderings ⭐
- [x] Series/episode scanning (SxxEyy, multi-episode files, specials, absolute numbering)
- [x] TVDB and TMDB series metadata
- [x] Source ordering per library (how filenames are numbered)
- [x] Display orderings: TVDB aired/DVD/absolute, TMDB aired, TMDB episode groups, custom
- [x] Mapping between orderings: IDs first, then air date + title, then manual overrides
- [x] Display ordering chosen per series
- [x] Display ordering overridable per user
- [x] Display-order preview: compare orderings side by side before switching
- [x] Unmatched episodes collected in a review bucket, never hidden

### ✅ 3. Library health and maintenance
- [x] Library preview when adding a folder: what was found, what couldn't be identified and why, before anything is imported
- [x] Library health page: unmatched items, probe failures, missing files, duplicates, missing episodes
- [x] "Why didn't this match?": show what was parsed from the filename and what was searched for
- [x] "Fix match": search a provider and pick the right result
- [x] Lock metadata fields against refreshes
- [x] Folder watching: new files appear without a manual scan
- [x] Local artwork cache
- [x] Automatic database backups with retention, plus one-click restore

### ✅ 4. Users and watch state
- [x] Accounts, login, sessions/tokens; first-run setup creates the admin
- [x] Settings in the web app, no config file: API keys, ffprobe found automatically, port and network access
- [x] Per-user profiles with PINs (Plex-style managed users), optional sign-in bypass on the home network
- [x] Watched state, resume position, play counts
- [x] Mark a season or whole show as watched/unwatched
- [x] Continue Watching, Next Up (follows the display ordering), Recently Added
- [x] Richer metadata: genres, cast and crew, studio/network, content rating, tagline, ratings
- [x] Parental controls by content rating
- [x] Remove items whose files are gone ("Empty trash")

### ✅ 5. AMS web app (Apple-style)
_Started early (after milestone 2) so progress can be seen as it's built._
- [x] Foundation: React + TypeScript + Tailwind + Radix, embedded in the binary
- [x] Design system basics: typography, colour tokens, motion, light/dark
- [x] Design system: refine spacing, components and states as the app grows
- [x] Home, library browsing, item detail, search, player (direct play)
- [x] Admin area that's simple: setup wizard, libraries, users, health
- [x] Admin dashboard: now playing, stop a stream, server log
- [x] Browse filters (genre, decade, unwatched, resolution, rating) and more sort options
- [x] Use local artwork and NFO files (read-only)
- [x] Extras and trailers on movie and show pages
- [x] Choose artwork from TMDB/TVDB alternatives or upload your own
- [x] Customisable home: reorder and hide built-in rows
- [x] Custom rows from filters ("Unwatched 90s action", "Added this week", "Short films under 90m")
- [x] Seasonal rows (Christmas films in December, horror in October), built on the same filter engine
- [x] Spoiler protection: blur thumbnails and hide descriptions of unwatched episodes

### ✅ 7. Transcoding and subtitles
- [x] Direct play / direct stream / transcode decision per client
- [x] FFmpeg to HLS, with segment cache and cleanup
- [x] Hardware acceleration: NVENC, Quick Sync, VAAPI, VideoToolbox
- [x] HDR to SDR tone mapping
- [x] Multiple versions: group 4K/1080p copies of the same title, auto-pick the best one for the device and connection
- [x] Editions (Director's Cut, Extended…) as choices on one movie
- [x] Player basics: audio track choice, keyboard shortcuts, playback speed
- [x] Device compatibility report: which files will need transcoding on which devices, and why
- [x] Scrubbing thumbnails (trickplay) when seeking
- [x] Subtitles: embedded, external .srt/.ass/.vtt, burn-in when needed
- [x] Online subtitle download

### 8. Intro and credits skipping
- [ ] Crowdsourced timestamp source (IntroDB or similar) as the first choice
- [ ] Check timestamps against each file's runtime before trusting them
- [ ] Local detection fallback (audio fingerprinting across episodes), run in the background at low priority
- [ ] Skip Intro / Skip Credits / auto-play next episode in the player and Jellyfin layer
- [ ] Manual editing of markers

### ✅ 9. Collections, playlists and discovery
- [x] Collections (TMDB collections automatically, plus manual)
- [x] Smart collections using the same filter engine as home rows
- [x] Playlists
- [x] Better search (people, genres, fuzzy matching)

### ✅ 10. Insights and stats
- [x] Watch time and history per user, per library, over time
- [x] Never-watched report: series and movies nobody has played
- [x] Storage report: what takes up the most space, and space used by never-watched titles
- [x] Most-watched titles, genres, and busiest days/times
- [x] Yearly "Wrapped"-style summary per user
- [x] Clear logos on movie and series pages (#81)
- [x] Cast higher up on series pages, not below every episode (#82)

### 11. Remote access
- [ ] HTTPS, secure remote access setup
- [ ] Sharing with friends, library access per user
- [ ] Bandwidth limits for remote streams

### Before public release
_Doesn't block development; must be done before AMS is shared or published._
- [ ] TheTVDB and TMDB attribution (required by their API terms)
- [ ] Check TheTVDB licence terms for open-source distribution (shipped key vs each user's own key)

## Later

- Music (deliberately deferred)
- Jellyfin-compatible API (was milestone 6; deferred in Oct 2026 so Arc's own features come first):
  discovery, auth, browsing, images; PlaybackInfo, streaming, progress reporting; display orderings
  and Next Up exposed to Jellyfin clients; tested against Jellyfin Web, Android/Android TV, Swiftfin,
  Findroid, Streamyfin, Infuse
- Audiobooks, photos, live TV/DVR
- Offline downloads/sync
- Import from Plex and Jellyfin: users, watch history, resume points
- "Other videos" (home videos) library type
- Notifications and webhooks
- Personal ratings and a watchlist
- Native apps (tvOS, iOS, Android TV) if the Jellyfin clients aren't polished enough

## Explicitly not planned

- Plugin system or third-party extension API
