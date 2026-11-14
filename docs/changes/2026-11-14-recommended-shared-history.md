# Recommended reads the watch history once

Three Recommended rows start from what the profile watched: Because You Watched (the most
recently watched titles), More in a favourite genre and the featured person row. Each read the
profile's whole history in the library on its own; for featured people, working out when each
title was last watched also meant reading every episode of every show. On a 100,000-episode
test library those three took about 110, 90–140 and 160 ms of a Series page.

Now the page reads the history once (`store.LibraryWatchHistory`, one line per movie or show:
when it was last played, when its credits were last watched, how many of its items are
watched) and the three rows work from it:

- Because You Watched takes the newest titles from it.
- Favourite genres weighs each watched title's genres by its watched items, as before.
- Featured people counts each person's lead credits once over the library, then walks the
  watched titles newest first only until the first ten places are settled (stopping between,
  never inside, a group of titles watched at the same time), instead of dating every title.

The old store functions remain as wrappers, so other callers and tests are unchanged.

Decisions and limits:

- The page is the same, with one deliberate difference: Because You Watched rows are now
  always newest first, as the function always said. The old query left their order (and the
  pick among titles watched at the same moment) to the database.
- A show's own watch row (from an older import) still counts for Because You Watched and
  genres, and still doesn't date the show's cast, as before.
- The history is held for one request only; nothing is cached or stored, no migration.
- Stacked on #466–#469 (Codex's Recommended PRs); merge those first.

Validation: a new store test compares all three rows with the old queries on a library with
ties, part-watched and undated plays, missing files, an age limit, a second profile and walk
batches of 1, 3 and 50; deliberately breaking the stopping rule or the credit dating makes it
fail. Codex's genre-weight test and the existing Recommended tests pass. On Codex's
100,000-episode measurement (same machine, two rounds each), Series warm medians went from
1,092/1,093 to 875/905 ms (1,000 shows × 100) and 1,156/1,077 to 787/793 ms (200 × 500); the
three rows now take the history read (92–147 ms) plus about 12 ms. Films are unchanged
(about 700 ms). `go vet ./...`, `go test ./...` (all pass except the HDR tone-mapping tests,
which need FFmpeg's `tonemapx` filter this machine lacks), store and api race runs and
`git diff --check` passed. No owner-server measurement yet.

Logging: the Recommended debug timings gain a `watch_history` step, so the shared read shows
up on its own; errors return through the existing request handling. Nothing new at info level.
