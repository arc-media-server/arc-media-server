# #350: merging titles preserves the source's watch state and live queue references

A failing store regression merged a watched/watchlisted source into a target with an existing
rating row. The kept movie became unwatched with no watchlist mark, and the source queue entry
was deleted, losing the current entry. Use the existing mergeInto policy for movie versions too:
watched wins, play count/progress rules remain shared, and either watchlist mark is retained while
the target's nonzero rating keeps priority. Keep the existing movie-file override for future scans.

The shared helper now rebinds play-queue entries before source deletion. Entry ids, positions,
duplicates and current-entry identity remain stable; affected queue revisions advance so stale
pre-merge edits return a conflict. Series/episode merges using the helper gain the same protection.
Everything stays inside the existing transaction; no file/media writes or state-copy side jobs.

Tests first reproduced lost state/current queue identity and accepted stale edits, then verify
retained watched/watchlist/rating data, queue membership/current identity/revision conflict and the
existing merge/split/series cases. Temporary SQLite only. No API response fields or web changes.
Existing API transaction-failure/request diagnostics cover this scope; no new routine success,
per-item, URL/body/key/token or SQL-argument logs are needed.
