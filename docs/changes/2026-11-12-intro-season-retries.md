# Intro detection: retry seasons that still have episodes without an intro

Since #369 an unreadable episode no longer fails its whole season, but a season was compared once
and then marked done until one of its files changed. Episodes that were skipped (unreadable audio)
or found no match stayed without an intro unless someone pressed Find Again.

Now, when a season comparison finishes with episodes still missing an intro, Arc compares that
season again a week later, then once more a month after that, and then stops. Only a changed
file (which also resets the retries) or Find Again brings it back after that. The owner asked
for retries but not for Arc to keep retrying the same files, hence the cap of two.

- `background_jobs` gains `retry_at` and `retries` (migration 0071). Changed files (a new
  revision) reset both. `background.Manager.RetryLater` schedules the next retry from a list of
  delays and refuses once they are used up; `Begin` admits a done job once its retry time comes.
- Markers counts the season's files without an intro after each comparison (whatever the source:
  chapters, online or detection) and asks for a retry when there are any.
- Online lookups were already re-checked every 30 days; this covers local detection.
- Retries still run only when maintenance allows "markers" work, like the first comparison.

Checks: go vet; `go test ./...` passes except the four conversion tests that need
jellyfin-ffmpeg's filters (FFmpeg 6.1 here; identical on main). New
TestRetryLaterIsBoundedAndResetByChangedFiles (due times, no early run, no repeat without asking,
the cap, and a changed file resetting retries). The two download migration tests that rebuild
an older schema now also drop the new columns.

Logging: each season left with episodes missing an intro logs one info line: the show, how many,
and the retry date, or that no automatic retries are left. No per-file logging.

Limits: still no list in the app of which episodes have no intro or why (it's in the log).
