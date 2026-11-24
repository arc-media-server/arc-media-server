# Continue Watching during a scan (#524)

Home reads the relevant series, episode orders and season artwork in batches. Continue Watching
shares the episode orders it used to pick Next Up with the code that displays the finished cards.
It keeps the same visibility checks, artwork rules and shared traversal for display orders and
inline specials. The API and write paths are unchanged.

On a disposable catalogue of 10,000 movies, 500 series, 20,000 episodes and 600 collections,
the median server response for Continue Watching with Next Up was:

| Database | Idle before → after | During a scan before → after |
| --- | ---: | ---: |
| SQLite | 37 → 13 ms | 155 → 57 ms |
| PostgreSQL 19 | 251 → 27 ms | 694 → 64 ms |

The scan used 180 copies of a generated clip. Both engines completed the scan, and concurrent
resume and watchlist writes succeeded and persisted. The idle measurements use five requests
per run. The active measurements sample while a scan runs, so the sample counts differ.
The results measure the server handler, excluding network and browser time.

The regression tests compare batched ordering with single-series traversal and check that aired
orders keep season artwork and custom orders omit it. Existing request logs cover timings and
errors. No additional logging is needed.
