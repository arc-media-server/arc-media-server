# Recommended: from seconds to about 150 ms on a large library

`GET /api/v1/libraries/{id}/recommended` took about 4.5 s on the owner's server (Unraid, 276
shows plus films), measured by the Kodi add-on. A synthetic library of the same size (276 shows
with 30 episodes each, or 2,000 films, with genres, cast and a watch history) reproduced it on a
fast machine: 950 ms for TV and 610 ms for films. Per-row timings showed four slow parts, all
caused by queries reading the profile's whole watch history once per title instead of looking
the title up:

- **"Has this show been watched?"** (`watchedSQL`: Start Watching, Rediscover, Recently Watched,
  and any listing filtered by watch state): SQLite chose to start from all of the profile's
  watch history and check each entry against the show, for every show. `CROSS JOIN` now makes it
  start from the show's own episodes (`items_parent`).
- **The "watched" sort** (Recently Watched): `id = X OR parent_id = X` can't use an index, so it
  read the history per title. It now looks up a title's own record by key, or a show's episodes.
- **Featured people** (Top by person): a summary of the history was joined to every credit and
  scanned in full for each one. Each title's last-watched time is now looked up once by key, and
  the titles are joined to their credits.

Results are unchanged: the same totals, the same people and the existing Recommended tests pass.

| Large library, whole call | Before | After |
|---|---|---|
| TV (276 shows) | ~950 ms | ~145 ms |
| Films (2,000) | ~610 ms | ~165 ms |

The slowest rows went from 255–439 ms to 10–30 ms. The same plans run in the library's listings
and filters wherever watch state is used, so those get faster too.

Diagnostics: with detailed logging on, Recommended now also logs one "recommended row timings"
line per full page (each row and lookup with its time, at debug, prepared only when debug is
enabled), next to the existing "recommended rows … took" line. That shows on a real server which
row is slow if this comes back.

Regression checks: `TestRecommendedQueriesLookUpWatchStateByTitle` (runs every time) fails if any
of these queries plans a read of the profile's whole history per title; it fails on the old
queries. `TestRecommendedTimingOnALargeLibrary` builds the large synthetic library and times the
page, with per-row timings, when run with `AMS_PERF=1` (it takes about 15 seconds).

Not done: caching between requests and building rows concurrently. Neither is needed at these
timings; the debug timings will show if they become worth it. Response shape unchanged.
